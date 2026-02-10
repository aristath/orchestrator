package backend

// Message represents a message sent to the backend.
type Message struct {
	Content string
	Role    string // "user" or "system"
}

// Response represents a response from the backend.
type Response struct {
	Content   string
	SessionID string
	Error     string
}

// Config defines the configuration for a backend.
type Config struct {
	Type         string // "claude", "codex", or "goose"
	WorkDir      string
	SessionID    string
	Model        string
	Provider     string // For Goose local LLMs (e.g., "ollama", "lmstudio", "llama.cpp")
	SystemPrompt string
}
