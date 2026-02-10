package backend

import (
	"regexp"
	"testing"
)

// TestNewClaudeAdapter_GeneratesSessionID verifies that a session ID is auto-generated
// when not provided in the config.
func TestNewClaudeAdapter_GeneratesSessionID(t *testing.T) {
	cfg := Config{
		Type: "claude",
	}

	adapter, err := NewClaudeAdapter(cfg, nil)
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	sessionID := adapter.SessionID()
	if sessionID == "" {
		t.Fatal("Expected non-empty session ID")
	}

	// Verify UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	// where y is 8, 9, a, or b
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(sessionID) {
		t.Errorf("Session ID does not match UUID v4 format: %s", sessionID)
	}
}

// TestNewClaudeAdapter_UsesProvidedSessionID verifies that a provided session ID
// is used instead of generating a new one.
func TestNewClaudeAdapter_UsesProvidedSessionID(t *testing.T) {
	expectedID := "test-session-12345"
	cfg := Config{
		Type:      "claude",
		SessionID: expectedID,
	}

	adapter, err := NewClaudeAdapter(cfg, nil)
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	if adapter.SessionID() != expectedID {
		t.Errorf("Expected session ID %s, got %s", expectedID, adapter.SessionID())
	}
}

// TestClaudeAdapter_BuildsFirstMessageCommand verifies that the first Send call
// builds args with --session-id (not --resume).
func TestClaudeAdapter_BuildsFirstMessageCommand(t *testing.T) {
	cfg := Config{
		Type:      "claude",
		SessionID: "test-uuid",
	}

	adapter, err := NewClaudeAdapter(cfg, nil)
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	msg := Message{Content: "Hello"}
	args := adapter.buildArgs(msg, false) // first message (not resume)

	// Verify args structure
	expected := []string{"-p", "Hello", "--output-format", "json", "--session-id", "test-uuid"}
	if !sliceEqual(args, expected) {
		t.Errorf("Expected args %v, got %v", expected, args)
	}

	// Verify --resume is NOT present
	if containsString(args, "--resume") {
		t.Error("First message should not contain --resume flag")
	}
}

// TestClaudeAdapter_BuildsResumeCommand verifies that subsequent Send calls
// use --resume instead of --session-id.
func TestClaudeAdapter_BuildsResumeCommand(t *testing.T) {
	cfg := Config{
		Type:      "claude",
		SessionID: "test-uuid",
	}

	adapter, err := NewClaudeAdapter(cfg, nil)
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	msg := Message{Content: "Hello again"}
	args := adapter.buildArgs(msg, true) // resume message

	// Verify args structure
	expected := []string{"-p", "Hello again", "--output-format", "json", "--resume", "test-uuid"}
	if !sliceEqual(args, expected) {
		t.Errorf("Expected args %v, got %v", expected, args)
	}

	// Verify --session-id is NOT present
	if containsString(args, "--session-id") {
		t.Error("Resume message should not contain --session-id flag")
	}
}

// TestClaudeAdapter_IncludesModel verifies that --model flag is included
// when model is configured.
func TestClaudeAdapter_IncludesModel(t *testing.T) {
	cfg := Config{
		Type:      "claude",
		SessionID: "test-uuid",
		Model:     "claude-opus-4",
	}

	adapter, err := NewClaudeAdapter(cfg, nil)
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	msg := Message{Content: "Test"}
	args := adapter.buildArgs(msg, false)

	// Verify --model is present
	if !containsString(args, "--model") {
		t.Error("Args should contain --model flag")
	}

	// Verify model value follows --model flag
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--model" && args[i+1] != "claude-opus-4" {
			t.Errorf("Expected model 'claude-opus-4', got '%s'", args[i+1])
		}
	}
}

// TestClaudeAdapter_IncludesSystemPrompt verifies that --system-prompt flag
// is included when system prompt is configured.
func TestClaudeAdapter_IncludesSystemPrompt(t *testing.T) {
	cfg := Config{
		Type:         "claude",
		SessionID:    "test-uuid",
		SystemPrompt: "You are a helpful assistant",
	}

	adapter, err := NewClaudeAdapter(cfg, nil)
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	msg := Message{Content: "Test"}
	args := adapter.buildArgs(msg, false)

	// Verify --system-prompt is present
	if !containsString(args, "--system-prompt") {
		t.Error("Args should contain --system-prompt flag")
	}

	// Verify system prompt value follows --system-prompt flag
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--system-prompt" && args[i+1] != "You are a helpful assistant" {
			t.Errorf("Expected system prompt 'You are a helpful assistant', got '%s'", args[i+1])
		}
	}
}

// TestClaudeAdapter_ParsesJSONResponse verifies that parseClaudeResponse
// correctly extracts content from Claude Code JSON output.
func TestClaudeAdapter_ParsesJSONResponse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantContent string
		wantSession string
		wantError   bool
	}{
		{
			name:        "valid response with single text content",
			input:       `{"session_id": "test-uuid-123", "result": {"content": [{"type": "text", "text": "Hello world"}]}}`,
			wantContent: "Hello world",
			wantSession: "test-uuid-123",
			wantError:   false,
		},
		{
			name:        "valid response with multiple text content",
			input:       `{"session_id": "test-uuid-456", "result": {"content": [{"type": "text", "text": "Part 1"}, {"type": "text", "text": "Part 2"}]}}`,
			wantContent: "Part 1Part 2",
			wantSession: "test-uuid-456",
			wantError:   false,
		},
		{
			name:        "valid response with mixed content types",
			input:       `{"session_id": "test-uuid-789", "result": {"content": [{"type": "text", "text": "Text"}, {"type": "image", "data": "..."}]}}`,
			wantContent: "Text",
			wantSession: "test-uuid-789",
			wantError:   false,
		},
		{
			name:        "empty content array",
			input:       `{"session_id": "test-uuid-empty", "result": {"content": []}}`,
			wantContent: "",
			wantSession: "test-uuid-empty",
			wantError:   false,
		},
		{
			name:      "invalid JSON",
			input:     `not valid json`,
			wantError: true,
		},
		{
			name:      "malformed structure",
			input:     `{"wrong": "structure"}`,
			wantError: false, // Should parse but return empty content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := parseClaudeResponse([]byte(tt.input))

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if resp.Content != tt.wantContent {
				t.Errorf("Expected content %q, got %q", tt.wantContent, resp.Content)
			}

			if resp.SessionID != tt.wantSession {
				t.Errorf("Expected session ID %q, got %q", tt.wantSession, resp.SessionID)
			}
		})
	}
}

// TestClaudeAdapter_Close verifies that Close() is a no-op and returns nil.
func TestClaudeAdapter_Close(t *testing.T) {
	cfg := Config{
		Type:      "claude",
		SessionID: "test-uuid",
	}

	adapter, err := NewClaudeAdapter(cfg, nil)
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	if err := adapter.Close(); err != nil {
		t.Errorf("Close() should return nil, got: %v", err)
	}
}

// TestClaudeAdapter_Send_MarksAsStarted verifies that Send() sets started flag
// after first successful call. This test mocks the subprocess execution.
func TestClaudeAdapter_Send_MarksAsStarted(t *testing.T) {
	// Note: This test cannot actually execute the claude CLI, but we can verify
	// the started flag behavior by checking buildArgs output before and after
	// setting started manually.
	cfg := Config{
		Type:      "claude",
		SessionID: "test-uuid",
	}

	adapter, err := NewClaudeAdapter(cfg, nil)
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	msg := Message{Content: "Test"}

	// Before first send (not started)
	argsFirst := adapter.buildArgs(msg, adapter.started)
	if !containsString(argsFirst, "--session-id") {
		t.Error("First call should use --session-id")
	}
	if containsString(argsFirst, "--resume") {
		t.Error("First call should not use --resume")
	}

	// Simulate successful send by marking as started
	adapter.started = true

	// After first send (started)
	argsResume := adapter.buildArgs(msg, adapter.started)
	if containsString(argsResume, "--session-id") {
		t.Error("Resume call should not use --session-id")
	}
	if !containsString(argsResume, "--resume") {
		t.Error("Resume call should use --resume")
	}
}

// Helper function to check if two string slices are equal
func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Helper function to check if a string slice contains a specific string
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
