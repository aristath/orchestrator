package config

// DefaultConfig returns the default configuration with built-in providers, agents, and workflows.
func DefaultConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		Providers: map[string]ProviderConfig{
			"claude": {
				Command: "claude",
				Type:    "claude",
			},
			"codex": {
				Command: "codex",
				Type:    "codex",
			},
			"goose": {
				Command: "goose",
				Type:    "goose",
			},
		},
		Agents: map[string]AgentConfig{
			"orchestrator": {
				Provider:     "claude",
				SystemPrompt: "You coordinate task planning and agent workflows.",
			},
			"coder": {
				Provider:     "claude",
				SystemPrompt: "You implement features and write production code.",
			},
			"reviewer": {
				Provider:     "claude",
				SystemPrompt: "You review code for correctness, style, and best practices.",
			},
			"tester": {
				Provider:     "claude",
				SystemPrompt: "You write comprehensive tests and validate functionality.",
			},
		},
		Workflows: map[string]WorkflowConfig{
			"standard": {
				Steps: []WorkflowStepConfig{
					{Agent: "coder"},
					{Agent: "reviewer"},
					{Agent: "tester"},
				},
			},
		},
	}
}
