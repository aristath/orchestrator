# Local LLM Orchestrator

An MCP server that turns local LLMs into sub-agent tools and provides orchestration prompts for iterative code improvement.

## What it does

This project provides two things:

1. **Sub-agent tools** — MCP tools that call a local LLM with specialized system prompts (coder, reviewer, improver, simplifier, planner). Any MCP-compatible agent can use these to get a second opinion, review code, or generate implementations.

2. **Orchestration prompts** — MCP prompts (`/brainstorm`, `/plan`, `/orchestrate`) that guide a session through problem exploration, planning, and autonomous execution with built-in review loops.

## How it works

```
You ←→ Agent (Crush/OpenCode/Claude Code)
              ↓ MCP tools
        Local LLM (llama.cpp / vLLM / Ollama)
              ↓ specialized roles
        coder / reviewer / improver / simplifier
```

The agent you're chatting with uses its own tools for file I/O, shell, and git. When it needs code generated, reviewed, or improved, it calls the MCP tools which send the request to a local LLM with the appropriate system prompt. The result comes back as text, and the agent applies it.

## Session flow

```
/brainstorm → explore the problem, challenge assumptions, decide on approach
/plan       → produce plan.md with concrete, ordered steps
/orchestrate → execute plan.md step by step, calling sub-agents for each step
```

Each phase is a prompt injected into the conversation. `plan.md` acts as persistent state — the orchestrator reads it, executes pending steps, and marks them done. This survives context compactions.

## Requirements

- Node.js 18+
- A local LLM server with an OpenAI-compatible API (llama.cpp, vLLM, Ollama, LM Studio, etc.)

## Installation

```bash
cd ~/orchestrator
npm install
```

## Configuration

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LLAMA_BASE_URL` | `http://localhost:8080` | Base URL of your OpenAI-compatible LLM server |

### Claude Code

Add to `~/.mcp.json`:

```json
{
  "mcpServers": {
    "local-llm": {
      "command": "node",
      "args": ["/path/to/orchestrator/server.js"],
      "env": {
        "LLAMA_BASE_URL": "http://localhost:8080"
      }
    }
  }
}
```

### Crush

Add to `~/.config/crush/crush.json`:

```json
{
  "mcp": {
    "local-llm": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/orchestrator/server.js"],
      "env": {
        "LLAMA_BASE_URL": "http://localhost:8080"
      }
    }
  }
}
```

### OpenCode

Add to `~/.config/opencode/opencode.json`:

```json
{
  "mcp": {
    "local-llm": {
      "type": "local",
      "command": ["node", "/path/to/orchestrator/server.js"],
      "environment": {
        "LLAMA_BASE_URL": "http://localhost:8080"
      }
    }
  }
}
```

## Available tools

These are registered as MCP tools. The agent calls them like any other tool.

| Tool | Description |
|------|-------------|
| `generate_code` | Send a task description (with file context), get code back |
| `review_code` | Send code, get a list of issues with severity levels |
| `improve_code` | Send code + review feedback, get improved code back |
| `simplify_code` | Send code, get a simplified version back |
| `plan_task` | Send a complex task, get it broken into implementation steps |

Each tool calls the local LLM as a **stateless, single-turn request** with a specialized system prompt. The tools don't read or write files — the calling agent handles that.

## Available prompts

These are registered as MCP prompts. In clients that support them (like Crush), they appear as slash commands.

| Prompt | Description |
|--------|-------------|
| `/brainstorm` | Collaborative problem exploration. No code, just thinking. |
| `/plan` | Turn the discussion into a structured `plan.md` with steps. |
| `/orchestrate` | Execute `plan.md` using sub-agent tools with review loops. |

## Project structure

```
orchestrator/
├── server.js              # MCP server — registers tools + prompts
├── roles/                 # Sub-agent system prompts → MCP tools
│   ├── coder.md
│   ├── reviewer.md
│   ├── improver.md
│   ├── simplifier.md
│   └── planner.md
└── prompts/               # Session prompts → MCP prompts (slash commands)
    ├── brainstorm.md
    ├── plan.md
    └── orchestrate.md
```

## Adding new roles or prompts

**New sub-agent tool:** Create a `.md` file in `roles/` with this format:

```markdown
---
name: tool_name
description: What the tool does (shown to the calling agent)
temperature: 0.4
---

Your system prompt here. This is what the local LLM sees.
```

**New prompt/command:** Create a `.md` file in `prompts/` with this format:

```markdown
---
name: command_name
description: What the command does
---

Your prompt content here. This is injected into the conversation.
```

Restart the MCP server after adding files.

## How the orchestration loop works

When `/orchestrate` is active, the agent follows this cycle for each step in `plan.md`:

```
generate_code → write to files → lint/test
     ↓
review_code → issues found?
     ↓ yes              ↓ no
improve_code         simplify_code
     ↓                   ↓
  (loop max 3x)    final review_code
                         ↓
                    git commit
                         ↓
                  update plan.md status
                         ↓
                    next step
```

The orchestrator uses its judgment — trivial steps skip the review cycle, clearly broken code gets regenerated instead of improved, and ambiguous steps prompt the user for clarification.

## License

MIT
