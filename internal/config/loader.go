package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Load reads and merges configuration from global and project paths.
// Order of precedence (highest to lowest): project config, global config, defaults.
// Missing files are not errors; malformed JSON returns an error.
func Load(globalPath, projectPath string) (*OrchestratorConfig, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Merge global config if exists
	if globalPath != "" {
		if err := mergeConfigFile(cfg, globalPath); err != nil {
			return nil, fmt.Errorf("loading global config: %w", err)
		}
	}

	// Merge project config if exists (highest precedence)
	if projectPath != "" {
		if err := mergeConfigFile(cfg, projectPath); err != nil {
			return nil, fmt.Errorf("loading project config: %w", err)
		}
	}

	return cfg, nil
}

// LoadDefault loads configuration from conventional paths.
// Global: ~/.orchestrator/config.json
// Project: .orchestrator/config.json (relative to cwd)
func LoadDefault() (*OrchestratorConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	globalPath := filepath.Join(homeDir, ".orchestrator", "config.json")
	projectPath := filepath.Join(".orchestrator", "config.json")

	return Load(globalPath, projectPath)
}

// mergeConfigFile reads a JSON config file and merges it into the base config.
// Missing files are silently skipped. Malformed JSON returns an error.
func mergeConfigFile(base *OrchestratorConfig, path string) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Missing file is not an error
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	// Parse JSON
	var loaded OrchestratorConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	// Merge providers
	for key, provider := range loaded.Providers {
		base.Providers[key] = provider
	}

	// Merge agents
	for key, agent := range loaded.Agents {
		base.Agents[key] = agent
	}

	// Merge workflows
	for key, workflow := range loaded.Workflows {
		base.Workflows[key] = workflow
	}

	return nil
}
