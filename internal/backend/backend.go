package backend

import (
	"context"
	"fmt"
)

// Backend defines the interface that all backend adapters must implement.
type Backend interface {
	// Send sends a message to the backend and returns the response.
	Send(ctx context.Context, msg Message) (Response, error)

	// Close terminates the backend subprocess gracefully.
	Close() error

	// SessionID returns the current session identifier.
	SessionID() string
}

// New creates a new backend based on the provided configuration.
// This factory function switches on cfg.Type and returns the appropriate adapter.
func New(cfg Config, pm *ProcessManager) (Backend, error) {
	switch cfg.Type {
	case "claude":
		return NewClaudeAdapter(cfg, pm)
	case "codex":
		return NewCodexAdapter(cfg, pm)
	case "goose":
		return NewGooseAdapter(cfg, pm)
	default:
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Type)
	}
}
