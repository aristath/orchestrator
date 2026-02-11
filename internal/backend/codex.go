package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CodexAdapter is the Codex CLI backend adapter.
// It uses the `codex` CLI tool to interact with OpenAI's GPT models.
type CodexAdapter struct {
	threadID string          // Thread ID for resuming conversations
	workDir  string          // Working directory for the CLI
	model    string          // Model override (optional)
	started  bool            // Tracks whether first message has been sent
	procMgr  *ProcessManager // Reference to shared process manager
}

// codexEvent is the base event type for all Codex events.
type codexEvent struct {
	Type string `json:"type"`
}

// codexThreadStarted represents the ThreadStarted event.
type codexThreadStarted struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
}

// codexTurnCompleted represents the TurnCompleted event.
type codexTurnCompleted struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// NewCodexAdapter creates a new Codex backend adapter.
// If cfg.SessionID is provided, it will be used as the initial thread ID for resuming sessions.
func NewCodexAdapter(cfg Config, procMgr *ProcessManager) (*CodexAdapter, error) {
	adapter := &CodexAdapter{
		threadID: cfg.SessionID, // May be empty for new threads
		workDir:  cfg.WorkDir,
		model:    cfg.Model,
		started:  cfg.SessionID != "", // If we have a session ID, we're resuming
		procMgr:  procMgr,
	}

	return adapter, nil
}

// Send sends a message to the Codex CLI and returns the response.
func (c *CodexAdapter) Send(ctx context.Context, msg Message) (Response, error) {
	// Build command arguments based on whether this is first message or resume
	args := c.buildArgs(msg)

	// Create command with process group isolation
	cmd := newCommand(ctx, "codex", args...)
	cmd.Dir = c.workDir

	// Execute command and capture output
	stdout, _, err := executeCommand(ctx, cmd, c.procMgr)
	if err != nil {
		return Response{
			Error: fmt.Sprintf("codex command failed: %v", err),
		}, err
	}

	// Parse the newline-delimited JSON event stream
	threadID, content, parseErr := parseCodexEvents(stdout)
	if parseErr != nil {
		return Response{
			Error: fmt.Sprintf("failed to parse codex events: %v", parseErr),
		}, parseErr
	}

	// Store thread ID if we got one from ThreadStarted event
	if threadID != "" {
		c.threadID = threadID
	}

	// Mark as started for future calls
	c.started = true

	return Response{
		Content:   content,
		SessionID: c.threadID,
	}, nil
}

// buildArgs constructs the command arguments for codex CLI.
// First message: ["exec", prompt, "--json"]
// Resume: ["resume", threadID, prompt, "--json"]
func (c *CodexAdapter) buildArgs(msg Message) []string {
	var args []string

	// Determine if this is first exec or resume
	if !c.started && c.threadID == "" {
		// First message: use exec
		args = []string{"exec", msg.Content, "--json"}
	} else {
		// Resume existing thread and send the new user message
		args = []string{"resume", c.threadID, msg.Content, "--json"}
	}

	// Add model override if configured
	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	return args
}

// parseCodexEvents parses newline-delimited JSON events from Codex CLI output.
// It extracts the thread_id from ThreadStarted events and content from TurnCompleted events.
func parseCodexEvents(data []byte) (threadID string, content string, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// First parse to get event type
		var evt codexEvent
		if parseErr := json.Unmarshal([]byte(line), &evt); parseErr != nil {
			return "", "", fmt.Errorf("failed to parse event type: %w", parseErr)
		}

		// Handle different event types
		switch evt.Type {
		case "ThreadStarted":
			var started codexThreadStarted
			if parseErr := json.Unmarshal([]byte(line), &started); parseErr != nil {
				return "", "", fmt.Errorf("failed to parse ThreadStarted event: %w", parseErr)
			}
			threadID = started.ThreadID

		case "TurnCompleted":
			var completed codexTurnCompleted
			if parseErr := json.Unmarshal([]byte(line), &completed); parseErr != nil {
				return "", "", fmt.Errorf("failed to parse TurnCompleted event: %w", parseErr)
			}
			content = completed.Content
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("error reading events: %w", err)
	}

	return threadID, content, nil
}

// Close terminates the Codex backend gracefully.
// Since Codex is invoked per-message (not a persistent subprocess), this is a no-op.
func (c *CodexAdapter) Close() error {
	return nil
}

// SessionID returns the current thread ID.
func (c *CodexAdapter) SessionID() string {
	return c.threadID
}
