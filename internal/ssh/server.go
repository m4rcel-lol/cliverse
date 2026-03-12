package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	glssh "github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/auth"
	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
	"github.com/m4rcel-lol/cliverse/internal/models"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
)

// contextKey is an unexported type for SSH session context values.
type contextKey string

const ctxKeyUser contextKey = "user"

// Server is the CLIverse SSH server.
type Server struct {
	config      *config.Config
	db          *db.DB
	shell       *Shell
	logger      *zap.Logger
	server      *glssh.Server
	rateLimiter *auth.RateLimiter
}

// New creates a new SSH Server.
func New(cfg *config.Config, database *db.DB, shell *Shell, logger *zap.Logger, rateLimiter *auth.RateLimiter) *Server {
	s := &Server{
		config:      cfg,
		db:          database,
		shell:       shell,
		logger:      logger,
		rateLimiter: rateLimiter,
	}
	s.server = s.buildServer()
	return s
}

// Start begins listening for SSH connections.
func (s *Server) Start() error {
	hostKey, err := s.loadOrGenerateHostKey()
	if err != nil {
		return fmt.Errorf("host key: %w", err)
	}
	s.server.AddHostKey(hostKey)

	addr := fmt.Sprintf(":%d", s.config.SSHPort)
	s.logger.Info("SSH server listening", zap.String("addr", addr))
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the SSH server.
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) buildServer() *glssh.Server {
	idleTimeout := s.config.SSHIdleTimeout
	if idleTimeout == 0 {
		idleTimeout = 30 * time.Minute
	}

	return &glssh.Server{
		Addr:        fmt.Sprintf(":%d", s.config.SSHPort),
		IdleTimeout: idleTimeout,
		Handler:     s.handleSession,
		PublicKeyHandler: func(ctx glssh.Context, key glssh.PublicKey) bool {
			return s.handlePublicKeyAuth(ctx, key)
		},
		PasswordHandler: func(ctx glssh.Context, password string) bool {
			return s.handlePasswordAuth(ctx, password)
		},
	}
}

// handlePublicKeyAuth authenticates a connection via SSH public key.
func (s *Server) handlePublicKeyAuth(ctx glssh.Context, key glssh.PublicKey) bool {
	remoteAddr := ctx.RemoteAddr().String()
	ip := extractIP(remoteAddr)
	h := gossh.FingerprintSHA256(key)

	// Check rate limit by IP for public key auth as well.
	if s.rateLimiter != nil {
		allowed, err := s.rateLimiter.CheckLoginAttempts(context.Background(), ip)
		if err != nil {
			s.logger.Warn("rate limiter check failed", zap.Error(err))
		} else if !allowed {
			s.logAttempt(nil, remoteAddr, "pubkey_auth", false, "rate limited")
			s.logger.Warn("SSH pubkey auth rate limited", zap.String("ip", ip))
			return false
		}
	}

	sshKey, err := s.db.GetSSHKeyByFingerprint(context.Background(), h)
	if err != nil {
		s.logger.Warn("public key lookup error", zap.String("fingerprint", h), zap.Error(err))
		s.logAttempt(nil, remoteAddr, "pubkey_auth", false, "db error")
		return false
	}
	if sshKey == nil {
		s.logAttempt(nil, remoteAddr, "pubkey_auth", false, "key not found")
		if s.rateLimiter != nil {
			_ = s.rateLimiter.RecordLoginAttempt(context.Background(), ip)
		}
		return false
	}

	user, err := s.db.GetUserByID(context.Background(), sshKey.UserID)
	if err != nil || user == nil {
		s.logAttempt(nil, remoteAddr, "pubkey_auth", false, "user not found")
		return false
	}

	if user.IsLocked {
		s.logAttempt(&user.ID, remoteAddr, "pubkey_auth", false, "account suspended")
		return false
	}

	ctx.SetValue(ctxKeyUser, user)
	s.logAttempt(&user.ID, remoteAddr, "pubkey_auth", true, "")
	s.logger.Info("SSH public key auth success",
		zap.String("username", user.Username),
		zap.String("remote", remoteAddr))
	return true
}

// handlePasswordAuth authenticates a connection via password.
func (s *Server) handlePasswordAuth(ctx glssh.Context, password string) bool {
	username := ctx.User()
	remoteAddr := ctx.RemoteAddr().String()
	ip := extractIP(remoteAddr)

	// Check rate limit by IP to slow brute-force attacks.
	if s.rateLimiter != nil {
		allowed, err := s.rateLimiter.CheckLoginAttempts(context.Background(), ip)
		if err != nil {
			s.logger.Warn("rate limiter check failed", zap.Error(err))
		} else if !allowed {
			s.logAttempt(nil, remoteAddr, "password_auth", false, "rate limited")
			s.logger.Warn("SSH login rate limited", zap.String("ip", ip))
			return false
		}
	}

	user, err := s.db.GetUserByUsername(context.Background(), username)
	if err != nil {
		s.logger.Warn("password auth lookup error", zap.String("username", username), zap.Error(err))
		s.logAttempt(nil, remoteAddr, "password_auth", false, "db error")
		if s.rateLimiter != nil {
			_ = s.rateLimiter.RecordLoginAttempt(context.Background(), ip)
		}
		return false
	}
	if user == nil {
		s.logAttempt(nil, remoteAddr, "password_auth", false, "user not found")
		if s.rateLimiter != nil {
			_ = s.rateLimiter.RecordLoginAttempt(context.Background(), ip)
		}
		return false
	}

	if user.IsLocked {
		s.logAttempt(&user.ID, remoteAddr, "password_auth", false, "account suspended")
		return false
	}

	ok, err := auth.VerifyPassword(password, user.PasswordHash)
	if err != nil || !ok {
		s.logAttempt(&user.ID, remoteAddr, "password_auth", false, "wrong password")
		s.logger.Info("SSH password auth failed", zap.String("username", username))
		if s.rateLimiter != nil {
			_ = s.rateLimiter.RecordLoginAttempt(context.Background(), ip)
		}
		return false
	}

	ctx.SetValue(ctxKeyUser, user)
	s.logAttempt(&user.ID, remoteAddr, "password_auth", true, "")
	s.logger.Info("SSH password auth success",
		zap.String("username", username),
		zap.String("remote", remoteAddr))
	return true
}

// handleSession runs the user's shell after successful authentication.
func (s *Server) handleSession(sess glssh.Session) {
	userVal := sess.Context().Value(ctxKeyUser)
	if userVal == nil {
		fmt.Fprintln(sess, "Authentication error: no user in context")
		sess.Exit(1)
		return
	}
	user, ok := userVal.(*models.User)
	if !ok || user == nil {
		fmt.Fprintln(sess, "Authentication error: invalid user context")
		sess.Exit(1)
		return
	}

	if user.IsLocked {
		fmt.Fprintf(sess, "\r\n\033[31mYour account has been suspended. Contact the administrator.\033[0m\r\n")
		sess.Exit(1)
		return
	}

	// Reject raw command execution for security — only our shell runs.
	if cmd := sess.RawCommand(); cmd != "" {
		fmt.Fprintf(sess, "\r\n\033[31mRemote command execution is not allowed.\033[0m\r\n")
		sess.Exit(1)
		return
	}

	remoteAddr := sess.RemoteAddr().String()
	if remoteAddr == "" {
		remoteAddr = "unknown"
	}

	sessionID := uuid.New()
	now := time.Now()
	dbSession := &models.Session{
		ID:         sessionID,
		UserID:     user.ID,
		RemoteAddr: remoteAddr,
		CreatedAt:  now,
		LastSeenAt: now,
		Ended:      false,
	}

	if err := s.db.CreateSession(context.Background(), dbSession); err != nil {
		s.logger.Warn("create session record failed", zap.Error(err))
	}

	s.logger.Info("session started",
		zap.String("username", user.Username),
		zap.String("remote", remoteAddr),
		zap.String("session_id", sessionID.String()))

	s.shell.Handle(sess, user, sessionID)

	if err := s.db.EndSession(context.Background(), sessionID); err != nil {
		s.logger.Warn("end session failed", zap.Error(err))
	}

	s.logger.Info("session ended",
		zap.String("username", user.Username),
		zap.String("session_id", sessionID.String()))
}

// logAttempt writes a login attempt to the audit log.
func (s *Server) logAttempt(userID *uuid.UUID, remoteAddr, action string, success bool, reason string) {
	details := fmt.Sprintf(`{"success":%v,"reason":%q}`, success, reason)
	_ = s.db.CreateAuditLog(context.Background(), &models.AuditLog{
		ID:        uuid.New(),
		ActorID:   userID,
		Action:    action,
		Target:    remoteAddr,
		Details:   details,
		IPAddr:    extractIP(remoteAddr),
		CreatedAt: time.Now(),
	})
}

// extractIP strips the port from an address like "1.2.3.4:5678".
func extractIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// loadOrGenerateHostKey loads the server's RSA host key from disk, generating
// a new 4096-bit key if none exists.
func (s *Server) loadOrGenerateHostKey() (gossh.Signer, error) {
	keyPath := filepath.Join("data", "host_key")

	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	data, err := os.ReadFile(keyPath)
	if err == nil {
		signer, err := gossh.ParsePrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("parse host key: %w", err)
		}
		return signer, nil
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("generate host key: %w", err)
	}

	signer, err := gossh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("create signer: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	if err := os.WriteFile(keyPath, privPEM, 0o600); err != nil {
		s.logger.Warn("could not persist host key", zap.Error(err))
	} else {
		s.logger.Info("generated new SSH host key", zap.String("path", keyPath))
	}

	return signer, nil
}
