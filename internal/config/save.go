package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Save persists the configuration to a JSON file.
// Creates parent directories if they don't exist.
func Save(cfg *OrchestratorConfig, path string) error {
	// Marshal config to JSON with indentation
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config to %s: %w", path, err)
	}

	return nil
}
