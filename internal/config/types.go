package config

// ProviderConfig defines a transport layer (CLI command, args, base settings).
// Providers are separate from agents -- multiple agents can share one provider.
type ProviderConfig struct {
	Command string   `json:"command"`          // CLI binary name (e.g., "claude", "codex", "goose")
	Args    []string `json:"args,omitempty"`   // Default args appended to every invocation
	Type    string   `json:"type"`             // Backend type matching backend.Config.Type: "claude", "codex", "goose"
}

// AgentConfig defines a role that uses a specific provider and model.
type AgentConfig struct {
	Provider     string   `json:"provider"`               // Key into Providers map
	Model        string   `json:"model,omitempty"`        // Model override (e.g., "opus-4", "gpt-4.1")
	SystemPrompt string   `json:"system_prompt,omitempty"` // Role-specific system prompt
	Tools        []string `json:"tools,omitempty"`         // Allowed tools for this role
}

// WorkflowStepConfig defines one step in a workflow pipeline.
type WorkflowStepConfig struct {
	Agent string `json:"agent"` // Key into Agents map
}

// WorkflowConfig defines a pipeline of agent steps (e.g., code -> review -> test).
type WorkflowConfig struct {
	Steps []WorkflowStepConfig `json:"steps"`
}

// OrchestratorConfig is the top-level configuration.
type OrchestratorConfig struct {
	Providers map[string]ProviderConfig `json:"providers"`
	Agents    map[string]AgentConfig    `json:"agents"`
	Workflows map[string]WorkflowConfig `json:"workflows"`
}
