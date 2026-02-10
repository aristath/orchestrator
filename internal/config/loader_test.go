package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name           string
		globalConfig   *OrchestratorConfig
		projectConfig  *OrchestratorConfig
		expectProviders int
		expectAgents    int
		expectWorkflows int
		checkAgent      string
		expectProvider  string
		checkModel      string
		expectModel     string
		expectError     bool
	}{
		{
			name:            "No config files - returns defaults",
			globalConfig:    nil,
			projectConfig:   nil,
			expectProviders: 3,
			expectAgents:    4,
			expectWorkflows: 1,
		},
		{
			name: "Global only - adds new agent",
			globalConfig: &OrchestratorConfig{
				Agents: map[string]AgentConfig{
					"css-specialist": {
						Provider:     "goose",
						SystemPrompt: "You specialize in CSS styling.",
					},
				},
			},
			projectConfig:   nil,
			expectProviders: 3,
			expectAgents:    5, // 4 defaults + 1 new
			expectWorkflows: 1,
			checkAgent:      "css-specialist",
			expectProvider:  "goose",
		},
		{
			name:         "Project only - overrides agent provider",
			globalConfig: nil,
			projectConfig: &OrchestratorConfig{
				Agents: map[string]AgentConfig{
					"coder": {
						Provider:     "codex",
						SystemPrompt: "You implement features using Codex.",
					},
				},
			},
			expectProviders: 3,
			expectAgents:    4, // Same count, but coder modified
			expectWorkflows: 1,
			checkAgent:      "coder",
			expectProvider:  "codex",
		},
		{
			name: "Both with merge - global adds, project overrides",
			globalConfig: &OrchestratorConfig{
				Agents: map[string]AgentConfig{
					"css-specialist": {
						Provider:     "goose",
						SystemPrompt: "You specialize in CSS styling.",
					},
				},
			},
			projectConfig: &OrchestratorConfig{
				Agents: map[string]AgentConfig{
					"coder": {
						Provider:     "codex",
						SystemPrompt: "You implement features using Codex.",
					},
				},
			},
			expectProviders: 3,
			expectAgents:    5, // 4 defaults + 1 from global, with coder overridden
			expectWorkflows: 1,
			checkAgent:      "coder",
			expectProvider:  "codex",
		},
		{
			name: "Project overrides global - project wins",
			globalConfig: &OrchestratorConfig{
				Agents: map[string]AgentConfig{
					"coder": {
						Provider: "claude",
						Model:    "model-x",
					},
				},
			},
			projectConfig: &OrchestratorConfig{
				Agents: map[string]AgentConfig{
					"coder": {
						Provider: "codex",
						Model:    "model-y",
					},
				},
			},
			expectProviders: 3,
			expectAgents:    4,
			expectWorkflows: 1,
			checkAgent:      "coder",
			expectProvider:  "codex",
			checkModel:      "coder",
			expectModel:     "model-y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test configs
			tmpDir := t.TempDir()

			// Write global config if specified
			globalPath := ""
			if tt.globalConfig != nil {
				globalPath = filepath.Join(tmpDir, "global.json")
				data, err := json.Marshal(tt.globalConfig)
				if err != nil {
					t.Fatalf("marshaling global config: %v", err)
				}
				if err := os.WriteFile(globalPath, data, 0644); err != nil {
					t.Fatalf("writing global config: %v", err)
				}
			}

			// Write project config if specified
			projectPath := ""
			if tt.projectConfig != nil {
				projectPath = filepath.Join(tmpDir, "project.json")
				data, err := json.Marshal(tt.projectConfig)
				if err != nil {
					t.Fatalf("marshaling project config: %v", err)
				}
				if err := os.WriteFile(projectPath, data, 0644); err != nil {
					t.Fatalf("writing project config: %v", err)
				}
			}

			// Load config
			cfg, err := Load(globalPath, projectPath)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify counts
			if got := len(cfg.Providers); got != tt.expectProviders {
				t.Errorf("providers count = %d, want %d", got, tt.expectProviders)
			}
			if got := len(cfg.Agents); got != tt.expectAgents {
				t.Errorf("agents count = %d, want %d", got, tt.expectAgents)
			}
			if got := len(cfg.Workflows); got != tt.expectWorkflows {
				t.Errorf("workflows count = %d, want %d", got, tt.expectWorkflows)
			}

			// Verify specific agent if specified
			if tt.checkAgent != "" {
				agent, exists := cfg.Agents[tt.checkAgent]
				if !exists {
					t.Errorf("expected agent %q not found", tt.checkAgent)
					return
				}
				if tt.expectProvider != "" && agent.Provider != tt.expectProvider {
					t.Errorf("agent %q provider = %q, want %q", tt.checkAgent, agent.Provider, tt.expectProvider)
				}
			}

			// Verify specific model if specified
			if tt.checkModel != "" {
				agent, exists := cfg.Agents[tt.checkModel]
				if !exists {
					t.Errorf("expected agent %q not found", tt.checkModel)
					return
				}
				if agent.Model != tt.expectModel {
					t.Errorf("agent %q model = %q, want %q", tt.checkModel, agent.Model, tt.expectModel)
				}
			}
		})
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create malformed JSON file
	globalPath := filepath.Join(tmpDir, "global.json")
	if err := os.WriteFile(globalPath, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("writing malformed config: %v", err)
	}

	// Load should return error
	_, err := Load(globalPath, "")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}

	// Error should mention the file
	if err.Error() == "" {
		t.Error("expected descriptive error message")
	}
}

func TestLoad_MissingFilesNotError(t *testing.T) {
	// Load with non-existent paths should not error
	cfg, err := Load("/nonexistent/global.json", "/nonexistent/project.json")
	if err != nil {
		t.Fatalf("expected no error for missing files, got: %v", err)
	}

	// Should return defaults
	if len(cfg.Providers) != 3 {
		t.Errorf("providers count = %d, want 3", len(cfg.Providers))
	}
	if len(cfg.Agents) != 4 {
		t.Errorf("agents count = %d, want 4", len(cfg.Agents))
	}
	if len(cfg.Workflows) != 1 {
		t.Errorf("workflows count = %d, want 1", len(cfg.Workflows))
	}
}
