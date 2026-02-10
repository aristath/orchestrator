package backend

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
)

// ClaudeAdapter implements the Backend interface for Claude Code CLI.
type ClaudeAdapter struct {
	sessionID    string
	workDir      string
	model        string
	systemPrompt string
	started      bool
	procMgr      *ProcessManager
}

// claudeResponse represents the JSON structure returned by Claude Code CLI.
// Example: {"session_id": "uuid", "result": {"content": [{"type": "text", "text": "response"}]}}
type claudeResponse struct {
	SessionID string `json:"session_id"`
	Result    struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
}

// NewClaudeAdapter creates a new Claude Code backend adapter.
// If cfg.SessionID is empty, a new UUID will be generated.
// The ProcessManager is optional - if nil, subprocesses won't be tracked.
func NewClaudeAdapter(cfg Config, procMgr *ProcessManager) (*ClaudeAdapter, error) {
	sessionID := cfg.SessionID
	if sessionID == "" {
		var err error
		sessionID, err = generateUUID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate session ID: %w", err)
		}
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	return &ClaudeAdapter{
		sessionID:    sessionID,
		workDir:      workDir,
		model:        cfg.Model,
		systemPrompt: cfg.SystemPrompt,
		started:      false,
		procMgr:      procMgr,
	}, nil
}

// Send sends a message to Claude Code CLI and returns the response.
// The first call uses --session-id, subsequent calls use --resume.
func (a *ClaudeAdapter) Send(ctx context.Context, msg Message) (Response, error) {
	// Build command arguments
	args := a.buildArgs(msg, a.started)

	// Create the command
	cmd := newCommand(ctx, "claude", args...)
	cmd.Dir = a.workDir

	// Execute the command (with optional ProcessManager tracking)
	stdout, stderr, err := executeCommand(ctx, cmd, a.procMgr)
	if err != nil {
		return Response{
			Error: fmt.Sprintf("claude command failed: %v", err),
		}, err
	}

	// Parse the JSON response
	resp, err := parseClaudeResponse(stdout)
	if err != nil {
		return Response{
			Error: fmt.Sprintf("failed to parse claude response: %v (stderr: %s)", err, string(stderr)),
		}, err
	}

	// Mark as started after first successful call
	a.started = true

	return resp, nil
}

// Close is a no-op for Claude Code (subprocess-per-invocation model).
func (a *ClaudeAdapter) Close() error {
	return nil
}

// SessionID returns the current session identifier.
func (a *ClaudeAdapter) SessionID() string {
	return a.sessionID
}

// buildArgs constructs the command-line arguments for the claude CLI.
// isResume determines whether to use --session-id (false) or --resume (true).
func (a *ClaudeAdapter) buildArgs(msg Message, isResume bool) []string {
	args := []string{"-p", msg.Content, "--output-format", "json"}

	// Session management: first call uses --session-id, subsequent use --resume
	if isResume {
		args = append(args, "--resume", a.sessionID)
	} else {
		args = append(args, "--session-id", a.sessionID)
	}

	// Add optional model override
	if a.model != "" {
		args = append(args, "--model", a.model)
	}

	// Add optional system prompt
	if a.systemPrompt != "" {
		args = append(args, "--system-prompt", a.systemPrompt)
	}

	return args
}

// parseClaudeResponse parses the JSON output from Claude Code CLI.
// Returns a Response struct with extracted content.
func parseClaudeResponse(data []byte) (Response, error) {
	var cr claudeResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return Response{}, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Extract text content from the content array
	var content string
	for _, item := range cr.Result.Content {
		if item.Type == "text" {
			content += item.Text
		}
	}

	return Response{
		Content:   content,
		SessionID: cr.SessionID,
	}, nil
}

// generateUUID generates a version 4 UUID (random).
// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx where y is 8, 9, a, or b.
func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Set version (4) and variant (RFC 4122)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant is 10

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
