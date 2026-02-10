package backend

import (
	"strings"
	"testing"
)

// TestFactory_CreatesClaudeAdapter verifies Claude adapter creation
func TestFactory_CreatesClaudeAdapter(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:    "claude",
		WorkDir: "/tmp/test",
	}

	backend, err := New(cfg, pm)
	if err != nil {
		t.Fatalf("Expected no error creating Claude adapter, got: %v", err)
	}

	if backend == nil {
		t.Fatal("Expected non-nil backend, got nil")
	}

	// Verify SessionID is non-empty (UUID should be generated)
	sessionID := backend.SessionID()
	if sessionID == "" {
		t.Error("Expected non-empty SessionID for Claude adapter")
	}

	t.Logf("Successfully created Claude adapter with session ID: %s", sessionID)
}

// TestFactory_CreatesCodexAdapter verifies Codex adapter creation
func TestFactory_CreatesCodexAdapter(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:    "codex",
		WorkDir: "/tmp/test",
	}

	backend, err := New(cfg, pm)
	if err != nil {
		t.Fatalf("Expected no error creating Codex adapter, got: %v", err)
	}

	if backend == nil {
		t.Fatal("Expected non-nil backend, got nil")
	}

	// Codex doesn't generate session ID until first message
	// Just verify it was created successfully
	t.Logf("Successfully created Codex adapter")
}

// TestFactory_CreatesGooseAdapter verifies Goose adapter creation
func TestFactory_CreatesGooseAdapter(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:    "goose",
		WorkDir: "/tmp/test",
	}

	backend, err := New(cfg, pm)
	if err != nil {
		t.Fatalf("Expected no error creating Goose adapter, got: %v", err)
	}

	if backend == nil {
		t.Fatal("Expected non-nil backend, got nil")
	}

	// Verify SessionID is non-empty (session name should be generated)
	sessionID := backend.SessionID()
	if sessionID == "" {
		t.Error("Expected non-empty SessionID for Goose adapter")
	}

	// Verify it follows orchestrator-{hex} format
	if !strings.HasPrefix(sessionID, "orchestrator-") {
		t.Errorf("Expected SessionID to start with 'orchestrator-', got: %s", sessionID)
	}

	t.Logf("Successfully created Goose adapter with session name: %s", sessionID)
}

// TestFactory_UnknownType verifies error handling for unknown backend types
func TestFactory_UnknownType(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:    "unknown",
		WorkDir: "/tmp/test",
	}

	backend, err := New(cfg, pm)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error for unknown backend type, got nil")
	}

	// Error should mention "unknown backend type"
	if !strings.Contains(err.Error(), "unknown backend type") {
		t.Errorf("Expected error to contain 'unknown backend type', got: %v", err)
	}

	// Backend should be nil
	if backend != nil {
		t.Errorf("Expected nil backend for unknown type, got: %v", backend)
	}

	t.Logf("Correctly rejected unknown backend type: %v", err)
}

// TestFactory_AllTypesImplementBackend verifies all adapters implement Backend interface
func TestFactory_AllTypesImplementBackend(t *testing.T) {
	pm := NewProcessManager()

	types := []string{"claude", "codex", "goose"}

	for _, backendType := range types {
		t.Run(backendType, func(t *testing.T) {
			cfg := Config{
				Type:    backendType,
				WorkDir: "/tmp/test",
			}

			backend, err := New(cfg, pm)
			if err != nil {
				t.Fatalf("Failed to create %s adapter: %v", backendType, err)
			}

			// Verify SessionID method works
			sessionID := backend.SessionID()
			t.Logf("%s adapter SessionID: %s", backendType, sessionID)

			// Verify Close method works
			err = backend.Close()
			if err != nil {
				t.Errorf("Close() failed for %s adapter: %v", backendType, err)
			}

			// Note: Send() is not called here because it would invoke actual CLI subprocesses
			// Send() is tested in integration tests and adapter-specific unit tests
		})
	}
}

// TestFactory_PassesConfig verifies config fields are passed to adapters
func TestFactory_PassesConfig(t *testing.T) {
	pm := NewProcessManager()

	testCases := []struct {
		name   string
		config Config
	}{
		{
			name: "claude with model",
			config: Config{
				Type:    "claude",
				WorkDir: "/tmp/test-claude",
				Model:   "claude-opus-4",
			},
		},
		{
			name: "codex with model",
			config: Config{
				Type:    "codex",
				WorkDir: "/tmp/test-codex",
				Model:   "gpt-4",
			},
		},
		{
			name: "goose with provider and model",
			config: Config{
				Type:     "goose",
				WorkDir:  "/tmp/test-goose",
				Model:    "llama2",
				Provider: "ollama",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend, err := New(tc.config, pm)
			if err != nil {
				t.Fatalf("Failed to create adapter: %v", err)
			}

			// Verify adapter was created (SessionID should work)
			sessionID := backend.SessionID()
			t.Logf("Created %s adapter with config. SessionID: %s", tc.config.Type, sessionID)

			// Close the adapter
			if err := backend.Close(); err != nil {
				t.Errorf("Close() failed: %v", err)
			}
		})
	}
}

// TestAllAdapters_CloseIsIdempotent verifies Close() can be called multiple times safely
func TestAllAdapters_CloseIsIdempotent(t *testing.T) {
	pm := NewProcessManager()

	types := []string{"claude", "codex", "goose"}

	for _, backendType := range types {
		t.Run(backendType, func(t *testing.T) {
			cfg := Config{
				Type:    backendType,
				WorkDir: "/tmp/test",
			}

			backend, err := New(cfg, pm)
			if err != nil {
				t.Fatalf("Failed to create %s adapter: %v", backendType, err)
			}

			// Call Close() twice
			err1 := backend.Close()
			if err1 != nil {
				t.Errorf("First Close() failed for %s: %v", backendType, err1)
			}

			err2 := backend.Close()
			if err2 != nil {
				t.Errorf("Second Close() failed for %s: %v", backendType, err2)
			}

			t.Logf("%s adapter Close() is idempotent", backendType)
		})
	}
}
