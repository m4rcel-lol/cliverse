package commands

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
	"github.com/m4rcel-lol/cliverse/internal/models"
	"go.uber.org/zap"
)

// Context holds all info for a command execution.
type Context struct {
	Ctx       context.Context
	User      *models.User
	Args      []string
	W         io.Writer
	DB        *db.DB
	Config    *config.Config
	Logger    *zap.Logger
	SessionID string
}

// HandlerFunc is the signature for command handlers.
type HandlerFunc func(ctx *Context) error

// Dispatcher routes commands to handlers.
type Dispatcher struct {
	handlers map[string]HandlerFunc
	cfg      *config.Config
	db       *db.DB
	logger   *zap.Logger
}

// NewDispatcher creates a Dispatcher and registers all built-in command handlers.
func NewDispatcher(cfg *config.Config, database *db.DB, logger *zap.Logger) *Dispatcher {
	d := &Dispatcher{
		handlers: make(map[string]HandlerFunc),
		cfg:      cfg,
		db:       database,
		logger:   logger,
	}
	registerAll(d)
	return d
}

// Register adds a named handler to the dispatcher.
func (d *Dispatcher) Register(name string, handler HandlerFunc) {
	d.handlers[name] = handler
}

// Dispatch looks up and calls the handler for the given command.
func (d *Dispatcher) Dispatch(ctx *Context, command string, args []string) error {
	handler, ok := d.handlers[command]
	if !ok {
		return fmt.Errorf("unknown command: %s. Type 'help' for available commands.", command)
	}
	ctx.Args = args
	return handler(ctx)
}

// Commands returns a sorted list of all registered command names.
func (d *Dispatcher) Commands() []string {
	names := make([]string, 0, len(d.handlers))
	for name := range d.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// sshKeyName generates a short display name for an SSH key from its raw string
// and fingerprint. Format: "key-type last-8-chars-of-fingerprint".
func sshKeyName(keyStr, fingerprint string) string {
	parts := strings.Fields(keyStr)
	name := fingerprint
	if len(parts) > 0 {
		name = parts[0]
		if len(fingerprint) > 8 {
			name += " " + fingerprint[len(fingerprint)-8:]
		}
	}
	return name
}
