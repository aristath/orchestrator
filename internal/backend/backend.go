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
func New(cfg Config) (Backend, error) {
	switch cfg.Type {
	case "claude":
		// TODO: Implement Claude Code adapter in plan 01-02
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Type)
	case "codex":
		// TODO: Implement Codex adapter in plan 01-03
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Type)
	case "goose":
		// TODO: Implement Goose adapter in plan 01-04
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Type)
	default:
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Type)
	}
}
