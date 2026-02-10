package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// GooseAdapter is a Backend implementation for the Goose CLI.
// Goose supports local LLM providers (Ollama, LM Studio, llama.cpp) via --provider and --model flags.
type GooseAdapter struct {
	sessionName  string
	workDir      string
	model        string
	provider     string
	systemPrompt string
	started      bool
	procMgr      *ProcessManager
}

// gooseResponse represents the JSON response structure from Goose CLI.
// Goose's JSON output format is less documented, so this struct is flexible.
type gooseResponse struct {
	Content string `json:"content"`
	// Additional fields can be added as discovered during integration testing
}

// NewGooseAdapter creates a new Goose adapter.
// If cfg.SessionID is empty, a session name is generated with format "orchestrator-{random-hex}".
func NewGooseAdapter(cfg Config, procMgr *ProcessManager) (*GooseAdapter, error) {
	sessionName := cfg.SessionID
	if sessionName == "" {
		// Generate session name: "orchestrator-{random-hex}"
		randomBytes := make([]byte, 4)
		if _, err := rand.Read(randomBytes); err != nil {
			return nil, fmt.Errorf("failed to generate session name: %w", err)
		}
		sessionName = "orchestrator-" + hex.EncodeToString(randomBytes)
	}

	return &GooseAdapter{
		sessionName:  sessionName,
		workDir:      cfg.WorkDir,
		model:        cfg.Model,
		provider:     cfg.Provider,
		systemPrompt: cfg.SystemPrompt,
		started:      false,
		procMgr:      procMgr,
	}, nil
}

// Send sends a message to Goose and returns the response.
// First call uses --name to start a new session.
// Subsequent calls use --resume to continue the session.
func (g *GooseAdapter) Send(ctx context.Context, msg Message) (Response, error) {
	// Build command arguments
	args := g.buildArgs(msg)

	// Create and execute command
	cmd := newCommand(ctx, "goose", args...)
	cmd.Dir = g.workDir

	stdout, stderr, err := executeCommand(ctx, cmd, g.procMgr)
	if err != nil {
		return Response{
			Error:     fmt.Sprintf("goose command failed: %v", err),
			SessionID: g.sessionName,
		}, err
	}

	// Parse response
	resp, parseErr := parseGooseResponse(stdout)
	if parseErr != nil {
		// If JSON parsing fails, fall back to treating stdout as plain text
		// This handles the case where --output-format json is not supported
		resp = Response{
			Content:   string(stdout),
			SessionID: g.sessionName,
		}
		// Include stderr if available
		if len(stderr) > 0 {
			resp.Content = string(stdout) + "\n[stderr]: " + string(stderr)
		}
	} else {
		resp.SessionID = g.sessionName
	}

	// Mark as started for future resume operations
	g.started = true

	return resp, nil
}

// buildArgs constructs the command-line arguments for the Goose CLI.
// Extracted into a separate method to make it testable.
func (g *GooseAdapter) buildArgs(msg Message) []string {
	args := []string{"run", "--text", msg.Content, "--output-format", "json"}

	// Session management: --name for first message, --resume for subsequent
	if !g.started {
		args = append(args, "--name", g.sessionName)
	} else {
		args = append(args, "--resume")
	}

	// Local LLM support: --provider and --model flags
	if g.provider != "" {
		args = append(args, "--provider", g.provider)
	}
	if g.model != "" {
		args = append(args, "--model", g.model)
	}

	// System prompt
	if g.systemPrompt != "" {
		args = append(args, "--system", g.systemPrompt)
	}

	return args
}

// parseGooseResponse parses the JSON response from Goose CLI.
// Tries parsing as a single JSON object first.
// If that fails, tries newline-delimited JSON (stream-json format).
func parseGooseResponse(data []byte) (Response, error) {
	// Try parsing as single JSON object
	var gooseResp gooseResponse
	if err := json.Unmarshal(data, &gooseResp); err == nil {
		return Response{
			Content: gooseResp.Content,
		}, nil
	}

	// Try newline-delimited JSON (same approach as Codex)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var contents []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var lineResp gooseResponse
		if err := json.Unmarshal([]byte(line), &lineResp); err == nil {
			if lineResp.Content != "" {
				contents = append(contents, lineResp.Content)
			}
		}
	}

	if len(contents) > 0 {
		return Response{
			Content: strings.Join(contents, "\n"),
		}, nil
	}

	return Response{}, fmt.Errorf("failed to parse Goose JSON response")
}

// Close terminates the Goose subprocess gracefully.
// For Goose, each invocation is a separate subprocess (no persistent connection),
// so this is a no-op.
func (g *GooseAdapter) Close() error {
	return nil
}

// SessionID returns the current session name.
func (g *GooseAdapter) SessionID() string {
	return g.sessionName
}
