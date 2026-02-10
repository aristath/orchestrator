package backend

import (
	"context"
	"strings"
	"testing"
)

func TestNewGooseAdapter_GeneratesSessionName(t *testing.T) {
	cfg := Config{
		Type:      "goose",
		WorkDir:   "/tmp",
		SessionID: "", // Empty - should generate
	}
	pm := NewProcessManager()

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	sessionID := adapter.SessionID()
	if !strings.HasPrefix(sessionID, "orchestrator-") {
		t.Errorf("Expected session ID to start with 'orchestrator-', got: %s", sessionID)
	}

	// Check that it has a hex suffix
	parts := strings.Split(sessionID, "-")
	if len(parts) != 2 {
		t.Errorf("Expected session ID format 'orchestrator-{hex}', got: %s", sessionID)
	}
}

func TestNewGooseAdapter_UsesProvidedSessionName(t *testing.T) {
	expectedSession := "my-custom-session"
	cfg := Config{
		Type:      "goose",
		WorkDir:   "/tmp",
		SessionID: expectedSession,
	}
	pm := NewProcessManager()

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	if adapter.SessionID() != expectedSession {
		t.Errorf("Expected session ID %s, got: %s", expectedSession, adapter.SessionID())
	}
}

func TestGooseAdapter_BuildsFirstRunCommand(t *testing.T) {
	cfg := Config{
		Type:      "goose",
		WorkDir:   "/tmp",
		SessionID: "test-session",
	}
	pm := NewProcessManager()

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	msg := Message{Content: "Hello Goose"}
	args := adapter.buildArgs(msg)

	// Verify required args for first run
	expectedArgs := []string{"run", "--text", "Hello Goose", "--output-format", "json", "--name", "test-session"}
	if !sliceContainsSequence(args, expectedArgs) {
		t.Errorf("Expected args to contain %v, got: %v", expectedArgs, args)
	}

	// Verify --resume is NOT present
	if sliceContains(args, "--resume") {
		t.Errorf("First run should not contain --resume, got: %v", args)
	}
}

func TestGooseAdapter_BuildsResumeCommand(t *testing.T) {
	cfg := Config{
		Type:      "goose",
		WorkDir:   "/tmp",
		SessionID: "test-session",
	}
	pm := NewProcessManager()

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	// Mark as started (simulate first call completed)
	adapter.started = true

	msg := Message{Content: "Continue conversation"}
	args := adapter.buildArgs(msg)

	// Verify --resume is present
	if !sliceContains(args, "--resume") {
		t.Errorf("Resume command should contain --resume, got: %v", args)
	}

	// Verify --name is NOT present
	if sliceContains(args, "--name") {
		t.Errorf("Resume command should not contain --name, got: %v", args)
	}
}

func TestGooseAdapter_IncludesProvider(t *testing.T) {
	cfg := Config{
		Type:      "goose",
		WorkDir:   "/tmp",
		SessionID: "test-session",
		Provider:  "ollama",
	}
	pm := NewProcessManager()

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	msg := Message{Content: "Test"}
	args := adapter.buildArgs(msg)

	// Verify --provider ollama is present
	if !sliceContainsSequence(args, []string{"--provider", "ollama"}) {
		t.Errorf("Expected args to contain --provider ollama, got: %v", args)
	}
}

func TestGooseAdapter_IncludesModel(t *testing.T) {
	cfg := Config{
		Type:      "goose",
		WorkDir:   "/tmp",
		SessionID: "test-session",
		Model:     "llama2",
	}
	pm := NewProcessManager()

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	msg := Message{Content: "Test"}
	args := adapter.buildArgs(msg)

	// Verify --model llama2 is present
	if !sliceContainsSequence(args, []string{"--model", "llama2"}) {
		t.Errorf("Expected args to contain --model llama2, got: %v", args)
	}
}

func TestGooseAdapter_IncludesSystemPrompt(t *testing.T) {
	cfg := Config{
		Type:         "goose",
		WorkDir:      "/tmp",
		SessionID:    "test-session",
		SystemPrompt: "You are a helpful assistant",
	}
	pm := NewProcessManager()

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	msg := Message{Content: "Test"}
	args := adapter.buildArgs(msg)

	// Verify --system is present
	if !sliceContainsSequence(args, []string{"--system", "You are a helpful assistant"}) {
		t.Errorf("Expected args to contain --system flag, got: %v", args)
	}
}

func TestGooseAdapter_LocalLLMConfig(t *testing.T) {
	cfg := Config{
		Type:      "goose",
		WorkDir:   "/tmp",
		SessionID: "test-session",
		Provider:  "ollama",
		Model:     "codellama",
	}
	pm := NewProcessManager()

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	msg := Message{Content: "Test"}
	args := adapter.buildArgs(msg)

	// Verify both --provider and --model are present
	if !sliceContainsSequence(args, []string{"--provider", "ollama"}) {
		t.Errorf("Expected args to contain --provider ollama, got: %v", args)
	}
	if !sliceContainsSequence(args, []string{"--model", "codellama"}) {
		t.Errorf("Expected args to contain --model codellama, got: %v", args)
	}
}

func TestGooseAdapter_ParsesJSONResponse(t *testing.T) {
	jsonData := []byte(`{"content": "Hello from Goose"}`)

	resp, err := parseGooseResponse(jsonData)
	if err != nil {
		t.Fatalf("parseGooseResponse failed: %v", err)
	}

	if resp.Content != "Hello from Goose" {
		t.Errorf("Expected content 'Hello from Goose', got: %s", resp.Content)
	}
}

func TestGooseAdapter_ParsesPlainTextFallback(t *testing.T) {
	// Test that invalid JSON falls through to error
	plainText := []byte("This is plain text, not JSON")

	_, err := parseGooseResponse(plainText)
	if err == nil {
		t.Error("Expected error for plain text input")
	}

	// The fallback to plain text happens in Send(), not in parseGooseResponse()
	// This test verifies that parseGooseResponse correctly returns an error
	// when JSON parsing fails, triggering the fallback logic in Send()
}

func TestGooseAdapter_ParsesNewlineDelimitedJSON(t *testing.T) {
	// Test newline-delimited JSON (stream format)
	ndjsonData := []byte(`{"content": "First line"}
{"content": "Second line"}
{"content": "Third line"}`)

	resp, err := parseGooseResponse(ndjsonData)
	if err != nil {
		t.Fatalf("parseGooseResponse failed for ndjson: %v", err)
	}

	expected := "First line\nSecond line\nThird line"
	if resp.Content != expected {
		t.Errorf("Expected content '%s', got: %s", expected, resp.Content)
	}
}

func TestGooseAdapter_Close(t *testing.T) {
	cfg := Config{
		Type:      "goose",
		WorkDir:   "/tmp",
		SessionID: "test-session",
	}
	pm := NewProcessManager()

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	// Close should return nil (no-op for per-invocation subprocess)
	if err := adapter.Close(); err != nil {
		t.Errorf("Close() should return nil, got: %v", err)
	}
}

// Helper function to check if a slice contains a value
func sliceContains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// Helper function to check if a slice contains a sequence of values in order
func sliceContainsSequence(slice []string, sequence []string) bool {
	if len(sequence) == 0 {
		return true
	}
	if len(slice) < len(sequence) {
		return false
	}

	for i := 0; i <= len(slice)-len(sequence); i++ {
		match := true
		for j := 0; j < len(sequence); j++ {
			if slice[i+j] != sequence[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestGooseAdapter_ImplementsBackendInterface verifies that GooseAdapter implements Backend
func TestGooseAdapter_ImplementsBackendInterface(t *testing.T) {
	cfg := Config{
		Type:      "goose",
		WorkDir:   "/tmp",
		SessionID: "test-session",
	}
	pm := NewProcessManager()

	var _ Backend = &GooseAdapter{} // Compile-time check

	adapter, err := NewGooseAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewGooseAdapter failed: %v", err)
	}

	// Verify all Backend interface methods exist
	ctx := context.Background()
	msg := Message{Content: "test"}

	// We can't actually execute Send without the goose CLI, but we can verify the signature
	_, _ = adapter.Send(ctx, msg)
	_ = adapter.Close()
	_ = adapter.SessionID()
}
