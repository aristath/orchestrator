package backend

import (
	"strings"
	"testing"
)

// TestNewCodexAdapter_NoInitialThreadID verifies that a new adapter starts without a thread ID.
func TestNewCodexAdapter_NoInitialThreadID(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:      "codex",
		WorkDir:   "/tmp",
		SessionID: "", // No initial thread ID
	}

	adapter, err := NewCodexAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewCodexAdapter failed: %v", err)
	}

	if adapter.SessionID() != "" {
		t.Errorf("Expected empty SessionID, got: %s", adapter.SessionID())
	}

	if adapter.started {
		t.Error("Expected started to be false for new adapter")
	}
}

// TestNewCodexAdapter_UsesProvidedThreadID verifies that an adapter with a SessionID is marked as started.
func TestNewCodexAdapter_UsesProvidedThreadID(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:      "codex",
		WorkDir:   "/tmp",
		SessionID: "thread_abc123",
	}

	adapter, err := NewCodexAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewCodexAdapter failed: %v", err)
	}

	if adapter.SessionID() != "thread_abc123" {
		t.Errorf("Expected SessionID 'thread_abc123', got: %s", adapter.SessionID())
	}

	if !adapter.started {
		t.Error("Expected started to be true when SessionID is provided")
	}
}

// TestCodexAdapter_BuildsFirstExecCommand verifies that the first Send builds an exec command.
func TestCodexAdapter_BuildsFirstExecCommand(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:      "codex",
		WorkDir:   "/tmp",
		SessionID: "",
	}

	adapter, err := NewCodexAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewCodexAdapter failed: %v", err)
	}

	msg := Message{Content: "Hello, world!", Role: "user"}
	args := adapter.buildArgs(msg)

	// Verify args contain exec, prompt, and --json
	if len(args) < 3 {
		t.Fatalf("Expected at least 3 args, got %d: %v", len(args), args)
	}

	if args[0] != "exec" {
		t.Errorf("Expected first arg 'exec', got: %s", args[0])
	}

	if args[1] != "Hello, world!" {
		t.Errorf("Expected second arg to be prompt, got: %s", args[1])
	}

	if args[2] != "--json" {
		t.Errorf("Expected third arg '--json', got: %s", args[2])
	}

	// Verify args do NOT contain resume
	for _, arg := range args {
		if arg == "resume" {
			t.Error("Expected args to NOT contain 'resume'")
		}
	}
}

// TestCodexAdapter_BuildsResumeCommand verifies that after setting threadID, args use resume.
func TestCodexAdapter_BuildsResumeCommand(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:      "codex",
		WorkDir:   "/tmp",
		SessionID: "thread_xyz789",
	}

	adapter, err := NewCodexAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewCodexAdapter failed: %v", err)
	}

	msg := Message{Content: "Follow-up question", Role: "user"}
	args := adapter.buildArgs(msg)

	// Verify args contain resume, thread ID, prompt, and --json
	if len(args) < 4 {
		t.Fatalf("Expected at least 4 args, got %d: %v", len(args), args)
	}

	if args[0] != "resume" {
		t.Errorf("Expected first arg 'resume', got: %s", args[0])
	}

	if args[1] != "thread_xyz789" {
		t.Errorf("Expected second arg to be thread ID, got: %s", args[1])
	}

	if args[2] != "Follow-up question" {
		t.Errorf("Expected third arg to be prompt, got: %s", args[2])
	}

	if args[3] != "--json" {
		t.Errorf("Expected fourth arg '--json', got: %s", args[3])
	}
}

// TestCodexAdapter_IncludesModel verifies that --model appears in args when configured.
func TestCodexAdapter_IncludesModel(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:      "codex",
		WorkDir:   "/tmp",
		Model:     "gpt-4",
		SessionID: "",
	}

	adapter, err := NewCodexAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewCodexAdapter failed: %v", err)
	}

	msg := Message{Content: "Test with model", Role: "user"}
	args := adapter.buildArgs(msg)

	// Find --model in args
	foundModel := false
	for i, arg := range args {
		if arg == "--model" && i+1 < len(args) && args[i+1] == "gpt-4" {
			foundModel = true
			break
		}
	}

	if !foundModel {
		t.Errorf("Expected args to contain '--model gpt-4', got: %v", args)
	}
}

// TestCodexAdapter_ParsesEventStream verifies event stream parsing logic.
func TestCodexAdapter_ParsesEventStream(t *testing.T) {
	data := `{"type":"ThreadStarted","thread_id":"thread_abc123"}
{"type":"TurnStarted"}
{"type":"TurnCompleted","content":"The answer is 42"}`

	threadID, content, err := parseCodexEvents([]byte(data))
	if err != nil {
		t.Fatalf("parseCodexEvents failed: %v", err)
	}

	if threadID != "thread_abc123" {
		t.Errorf("Expected threadID 'thread_abc123', got: %s", threadID)
	}

	if content != "The answer is 42" {
		t.Errorf("Expected content 'The answer is 42', got: %s", content)
	}
}

// TestCodexAdapter_ParsesEmptyEvents verifies parsing with empty input.
func TestCodexAdapter_ParsesEmptyEvents(t *testing.T) {
	data := ""

	threadID, content, err := parseCodexEvents([]byte(data))
	if err != nil {
		t.Fatalf("parseCodexEvents failed: %v", err)
	}

	if threadID != "" {
		t.Errorf("Expected empty threadID, got: %s", threadID)
	}

	if content != "" {
		t.Errorf("Expected empty content, got: %s", content)
	}
}

// TestCodexAdapter_ParsesMalformedJSON verifies error handling for invalid JSON.
func TestCodexAdapter_ParsesMalformedJSON(t *testing.T) {
	data := `{"type":"ThreadStarted","thread_id":"thread_abc123"}
{invalid json line}
{"type":"TurnCompleted","content":"Answer"}`

	_, _, err := parseCodexEvents([]byte(data))
	if err == nil {
		t.Error("Expected parseCodexEvents to return error for malformed JSON")
	}

	if !strings.Contains(err.Error(), "failed to parse event type") {
		t.Errorf("Expected error about parsing event type, got: %v", err)
	}
}

// TestCodexAdapter_ExtractsThreadIDFromResponse verifies thread ID is stored after parsing.
func TestCodexAdapter_ExtractsThreadIDFromResponse(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:      "codex",
		WorkDir:   "/tmp",
		SessionID: "",
	}

	adapter, err := NewCodexAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewCodexAdapter failed: %v", err)
	}

	// Simulate parsing a response with ThreadStarted
	data := `{"type":"ThreadStarted","thread_id":"thread_new123"}
{"type":"TurnCompleted","content":"Response text"}`

	threadID, content, err := parseCodexEvents([]byte(data))
	if err != nil {
		t.Fatalf("parseCodexEvents failed: %v", err)
	}

	// Manually store the threadID (this is what Send() does)
	adapter.threadID = threadID
	adapter.started = true

	// Verify the adapter now has the thread ID
	if adapter.SessionID() != "thread_new123" {
		t.Errorf("Expected SessionID 'thread_new123', got: %s", adapter.SessionID())
	}

	if content != "Response text" {
		t.Errorf("Expected content 'Response text', got: %s", content)
	}

	// Verify subsequent buildArgs uses resume and includes prompt
	msg := Message{Content: "Next message", Role: "user"}
	args := adapter.buildArgs(msg)

	if args[0] != "resume" {
		t.Errorf("Expected first arg 'resume' after thread ID is set, got: %s", args[0])
	}

	if args[1] != "thread_new123" {
		t.Errorf("Expected second arg to be thread ID, got: %s", args[1])
	}

	if args[2] != "Next message" {
		t.Errorf("Expected third arg to be prompt, got: %s", args[2])
	}
}

// TestCodexAdapter_Close verifies Close returns nil.
func TestCodexAdapter_Close(t *testing.T) {
	pm := NewProcessManager()
	cfg := Config{
		Type:      "codex",
		WorkDir:   "/tmp",
		SessionID: "",
	}

	adapter, err := NewCodexAdapter(cfg, pm)
	if err != nil {
		t.Fatalf("NewCodexAdapter failed: %v", err)
	}

	if err := adapter.Close(); err != nil {
		t.Errorf("Expected Close to return nil, got: %v", err)
	}
}
