package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveCreatesFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	// Create test config
	cfg := &OrchestratorConfig{
		Providers: map[string]ProviderConfig{
			"test": {Command: "test-cmd", Type: "test"},
		},
		Agents: map[string]AgentConfig{
			"test-agent": {Provider: "test", Model: "test-model"},
		},
		Workflows: map[string]WorkflowConfig{},
	}

	// Save config
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Config file was not created: %s", path)
	}

	// Verify file contains valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var loaded OrchestratorConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Config file contains invalid JSON: %v", err)
	}

	// Verify provider was saved
	if loaded.Providers["test"].Command != "test-cmd" {
		t.Errorf("Expected provider command 'test-cmd', got '%s'", loaded.Providers["test"].Command)
	}
}

func TestSaveCreatesParentDir(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	// Nested path that doesn't exist yet
	path := filepath.Join(tmpDir, "nested", "deep", "config.json")

	// Create minimal config
	cfg := &OrchestratorConfig{
		Providers: map[string]ProviderConfig{},
		Agents:    map[string]AgentConfig{},
		Workflows: map[string]WorkflowConfig{},
	}

	// Save should create all parent directories
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Config file was not created: %s", path)
	}

	// Verify parent directories exist
	parentDir := filepath.Dir(path)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Fatalf("Parent directory was not created: %s", parentDir)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	// Create config with diverse fields
	cfg := &OrchestratorConfig{
		Providers: map[string]ProviderConfig{
			"claude": {Command: "claude", Type: "claude"},
			"goose":  {Command: "goose", Type: "goose", Args: []string{"--verbose"}},
		},
		Agents: map[string]AgentConfig{
			"coder": {
				Provider:     "claude",
				Model:        "opus-4",
				SystemPrompt: "You write code.",
				Tools:        []string{"read", "write"},
			},
			"reviewer": {
				Provider:     "claude",
				Model:        "sonnet-4",
				SystemPrompt: "You review code.",
			},
		},
		Workflows: map[string]WorkflowConfig{
			"standard": {
				Steps: []WorkflowStepConfig{
					{Agent: "coder"},
					{Agent: "reviewer"},
				},
			},
		},
	}

	// Save config
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load it back
	loaded, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify providers
	if loaded.Providers["claude"].Command != "claude" {
		t.Errorf("Claude provider command mismatch: got '%s'", loaded.Providers["claude"].Command)
	}
	if len(loaded.Providers["goose"].Args) != 1 || loaded.Providers["goose"].Args[0] != "--verbose" {
		t.Errorf("Goose provider args mismatch: got %v", loaded.Providers["goose"].Args)
	}

	// Verify agents
	if loaded.Agents["coder"].Model != "opus-4" {
		t.Errorf("Coder model mismatch: got '%s'", loaded.Agents["coder"].Model)
	}
	if len(loaded.Agents["coder"].Tools) != 2 {
		t.Errorf("Coder tools count mismatch: got %d", len(loaded.Agents["coder"].Tools))
	}

	// Verify workflows
	if len(loaded.Workflows["standard"].Steps) != 2 {
		t.Errorf("Workflow steps count mismatch: got %d", len(loaded.Workflows["standard"].Steps))
	}
	if loaded.Workflows["standard"].Steps[0].Agent != "coder" {
		t.Errorf("Workflow first step mismatch: got '%s'", loaded.Workflows["standard"].Steps[0].Agent)
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	// Save first config
	cfg1 := &OrchestratorConfig{
		Providers: map[string]ProviderConfig{
			"test": {Command: "first-value", Type: "test"},
		},
		Agents:    map[string]AgentConfig{},
		Workflows: map[string]WorkflowConfig{},
	}
	if err := Save(cfg1, path); err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	// Save second config with different values
	cfg2 := &OrchestratorConfig{
		Providers: map[string]ProviderConfig{
			"test": {Command: "second-value", Type: "test"},
		},
		Agents:    map[string]AgentConfig{},
		Workflows: map[string]WorkflowConfig{},
	}
	if err := Save(cfg2, path); err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	// Load and verify second value wins
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var loaded OrchestratorConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if loaded.Providers["test"].Command != "second-value" {
		t.Errorf("Expected 'second-value', got '%s'", loaded.Providers["test"].Command)
	}
}
