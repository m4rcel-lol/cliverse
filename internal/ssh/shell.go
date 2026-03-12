package ssh

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	gossh "github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/commands"
	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
	"github.com/m4rcel-lol/cliverse/internal/models"
	"go.uber.org/zap"
)

// ANSI color constants used throughout the shell output.
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorCyan    = "\033[36m"
	ColorBold    = "\033[1m"
	ColorMagenta = "\033[35m"
	ColorBlue    = "\033[34m"
)

// Shell is the interactive CLI shell presented to authenticated users.
type Shell struct {
	config   *config.Config
	db       *db.DB
	logger   *zap.Logger
	dispatch *commands.Dispatcher
}

// NewShell creates a new Shell.
func NewShell(cfg *config.Config, database *db.DB, logger *zap.Logger, dispatch *commands.Dispatcher) *Shell {
	return &Shell{
		config:   cfg,
		db:       database,
		logger:   logger,
		dispatch: dispatch,
	}
}

// Handle runs the interactive shell loop for the given SSH session.
func (s *Shell) Handle(sess gossh.Session, user *models.User, sessionID uuid.UUID) {
	ctx := context.Background()

	// Write the welcome banner
	s.writeBanner(sess, user)

	// Show dashboard summary on login
	s.writeDashboard(sess, ctx, user, time.Now())

	history := make([]string, 0, 50)
	historyIdx := -1

	prompt := fmt.Sprintf("%sCLIverse%s | %s@%s@%s%s > ",
		ColorCyan+ColorBold, ColorReset,
		ColorCyan, user.Username, s.config.Domain, ColorReset)

	var line []byte

	for {
		fmt.Fprint(sess, prompt)
		line = line[:0]

		// Read input character by character
		buf := make([]byte, 1)
		var escaped bool
		var escBuf []byte

	inputLoop:
		for {
			n, err := sess.Read(buf)
			if n == 0 || err != nil {
				if err == io.EOF {
					fmt.Fprintf(sess, "\r\n%sGoodbye!%s\r\n", ColorCyan, ColorReset)
					return
				}
				return
			}

			b := buf[0]

			if escaped {
				escBuf = append(escBuf, b)
				if len(escBuf) == 2 && escBuf[0] == '[' {
					switch escBuf[1] {
					case 'A': // Arrow up
						if len(history) > 0 {
							if historyIdx < len(history)-1 {
								historyIdx++
							}
							// Clear current line
							clearLine(sess, line)
							line = []byte(history[len(history)-1-historyIdx])
							fmt.Fprint(sess, string(line))
						}
					case 'B': // Arrow down
						if historyIdx > 0 {
							historyIdx--
							clearLine(sess, line)
							line = []byte(history[len(history)-1-historyIdx])
							fmt.Fprint(sess, string(line))
						} else if historyIdx == 0 {
							historyIdx = -1
							clearLine(sess, line)
							line = line[:0]
						}
					}
					escaped = false
					escBuf = nil
					continue
				}
				if len(escBuf) >= 3 {
					escaped = false
					escBuf = nil
				}
				continue
			}

			switch b {
			case 0x1b: // ESC — start of escape sequence
				escaped = true
				escBuf = []byte{}
				continue

			case '\r', '\n': // Enter
				fmt.Fprintf(sess, "\r\n")
				break inputLoop

			case 0x7f, 0x08: // Backspace / DEL
				if len(line) > 0 {
					// Remove last UTF-8 rune
					_, size := utf8.DecodeLastRune(line)
					line = line[:len(line)-size]
					fmt.Fprintf(sess, "\b \b")
				}

			case 0x03: // Ctrl-C — cancel current line
				fmt.Fprintf(sess, "^C\r\n")
				line = line[:0]
				historyIdx = -1
				break inputLoop

			case 0x04: // Ctrl-D — exit
				if len(line) == 0 {
					fmt.Fprintf(sess, "\r\n%sGoodbye!%s\r\n", ColorCyan, ColorReset)
					return
				}

			case 0x15: // Ctrl-U — clear line
				clearLine(sess, line)
				line = line[:0]

			default:
				if b >= 0x20 { // Printable character
					line = append(line, b)
					fmt.Fprintf(sess, "%s", string([]byte{b}))
				}
			}
		}

		input := strings.TrimSpace(string(line))
		if input == "" {
			historyIdx = -1
			continue
		}

		// Add to history (avoid duplicates at the end)
		if len(history) == 0 || history[len(history)-1] != input {
			history = append(history, input)
			if len(history) > 100 {
				history = history[1:]
			}
		}
		historyIdx = -1

		// Built-in commands
		if input == "exit" || input == "quit" {
			fmt.Fprintf(sess, "%sGoodbye!%s\r\n", ColorCyan, ColorReset)
			return
		}

		// Parse and dispatch command
		parts := parseCommand(input)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		args := parts[1:]

		cmdCtx := &commands.Context{
			Ctx:        ctx,
			User:       user,
			Args:       args,
			W:          newSessionWriter(sess),
			DB:         s.db,
			Config:     s.config,
			Logger:     s.logger,
			SessionID:  sessionID.String(),
			Dispatcher: s.dispatch,
		}

		if err := s.dispatch.Dispatch(cmdCtx, cmd, args); err != nil {
			fmt.Fprintf(sess, "%sError: %s%s\r\n", ColorRed, err.Error(), ColorReset)
		}

		// Update session last-seen periodically
		_ = s.db.UpdateSessionLastSeen(ctx, sessionID)
	}
}

// writeBanner prints the CLIverse welcome banner with instance name.
func (s *Shell) writeBanner(w io.Writer, user *models.User) {
	fmt.Fprintf(w, "\r\n")
	fmt.Fprintf(w, "%s%s╔══════════════════════════════════════╗%s\r\n", ColorCyan, ColorBold, ColorReset)
	fmt.Fprintf(w, "%s%s║           Welcome to CLIverse         ║%s\r\n", ColorCyan, ColorBold, ColorReset)
	fmt.Fprintf(w, "%s%s║    The Fediverse — right in your SSH  ║%s\r\n", ColorCyan, ColorBold, ColorReset)
	fmt.Fprintf(w, "%s%s╚══════════════════════════════════════╝%s\r\n", ColorCyan, ColorBold, ColorReset)
	fmt.Fprintf(w, "\r\n")
	fmt.Fprintf(w, "  Instance: %s%s%s (%s)\r\n", ColorBold, s.config.InstanceName, ColorReset, s.config.Domain)
	fmt.Fprintf(w, "  Logged in as: %s@%s@%s%s\r\n", ColorGreen, user.Username, s.config.Domain, ColorReset)
	if user.IsAdmin {
		fmt.Fprintf(w, "  Role: %s[admin]%s\r\n", ColorYellow, ColorReset)
	}
	fmt.Fprintf(w, "\r\n  Type %shelp%s for available commands, %sexit%s to disconnect.\r\n\r\n",
		ColorCyan, ColorReset, ColorCyan, ColorReset)
}

// writeDashboard prints a quick summary of the user's account state.
// connectedAt should be captured when the session begins.
func (s *Shell) writeDashboard(w io.Writer, ctx context.Context, user *models.User, connectedAt time.Time) {
	unread, _ := s.db.CountUnreadNotifications(ctx, user.ID)
	posts, _ := s.db.ListPostsByUser(ctx, user.ID, 5, 0)

	fmt.Fprintf(w, "%s── Dashboard ──%s\r\n", ColorBold, ColorReset)
	fmt.Fprintf(w, "  Handle:        %s@%s@%s%s\r\n", ColorCyan, user.Username, s.config.Domain, ColorReset)

	if unread > 0 {
		fmt.Fprintf(w, "  Notifications: %s%d unread%s\r\n", ColorYellow, unread, ColorReset)
	} else {
		fmt.Fprintf(w, "  Notifications: none\r\n")
	}

	fmt.Fprintf(w, "  Recent posts:  %d\r\n", len(posts))

	if user.ForcePassChange {
		fmt.Fprintf(w, "\r\n  %s⚠ Please change your password: settings update_password%s\r\n", ColorYellow, ColorReset)
	}
	if user.IsSilenced {
		fmt.Fprintf(w, "  %s⚠ Your account is silenced. Posts are visible locally only.%s\r\n", ColorYellow, ColorReset)
	}

	// Show message of the day if configured.
	if motd, err := s.db.GetSystemConfig(ctx, "motd"); err == nil && motd != "" {
		fmt.Fprintf(w, "\r\n  %s📢 %s%s\r\n", ColorMagenta, motd, ColorReset)
	}

	fmt.Fprintf(w, "\r\n")

	fmt.Fprintf(w, "  Connected at:  %s\r\n\r\n", connectedAt.UTC().Format("2006-01-02 15:04 UTC"))
}

// clearLine erases the current line content from the terminal.
func clearLine(w io.Writer, line []byte) {
	for range line {
		fmt.Fprint(w, "\b \b")
	}
}

// parseCommand splits an input string by spaces, respecting double-quoted strings.
// Example: `post global "hello world"` → ["post", "global", "hello world"]
func parseCommand(input string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(input); i++ {
		c := input[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
		case c == ' ' && !inQuotes:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// sessionWriter wraps an SSH session so output uses \r\n line endings,
// required for raw SSH terminal mode.
type sessionWriter struct {
	sess gossh.Session
}

func newSessionWriter(sess gossh.Session) io.Writer {
	return &sessionWriter{sess: sess}
}

func (sw *sessionWriter) Write(p []byte) (int, error) {
	// Replace bare \n with \r\n for proper terminal display
	converted := strings.ReplaceAll(string(p), "\n", "\r\n")
	// Avoid double \r\r\n
	converted = strings.ReplaceAll(converted, "\r\r\n", "\r\n")
	n, err := sw.sess.Write([]byte(converted))
	if err != nil {
		return 0, err
	}
	// Return original length so callers aren't confused by the expansion
	if n > len(p) {
		return len(p), nil
	}
	return n, nil
}
