# Local LLM Orchestrator

An MCP server that exposes local LLMs as tools for Claude Code (or any MCP-compatible agent). The idea: use Claude as the senior engineer who thinks, decides, and verifies — and delegate the mechanical work (code generation, review, improvement) to local models running on your own hardware.

## Architecture

```
You ↔ Claude Code
          ↓ MCP tools
    local-llm server (this project)
          ↓ HTTP (OpenAI-compatible API)
    llama-router (llama.cpp, port 8080)
          ↓
    ┌─────────────────┬──────────────────────────┐
    │   Vulkan1+3     │         Vulkan2           │
    │  qwen3-coder-   │  devstral-small-2-24b-   │
    │  next-q40 (80B) │  2512-q40 (24B)           │
    │  code generation│  review + planning        │
    └─────────────────┴──────────────────────────┘
```

The router keeps both models loaded simultaneously. Claude never writes code itself when it can delegate — see `CLAUDE.md` for the delegation guidelines.

## How to use it

### The delegation workflow

For any non-trivial coding task, Claude follows this pipeline:

```
generate_code → review_code → (issues found?) → improve_code → Claude verifies
```

Claude also uses `plan_task` to break down complex tasks before starting, and applies its own judgment throughout — local LLMs handle drafts, Claude handles decisions.

### The session workflow (for complex tasks)

Three MCP prompts guide you through a full implementation session:

```
/brainstorm → explore the problem, challenge assumptions, no code yet
/plan       → produce plan.md with ordered, concrete steps
/orchestrate → execute plan.md step by step using sub-agent tools
```

`plan.md` is the persistent state for the session — the orchestrator reads it, executes pending steps, marks them done. It survives context compactions.

## MCP tools

These are the sub-agent tools Claude calls to delegate work to local LLMs. Each tool is a stateless, single-turn request to the local model — it does not read or write files.

| Tool | Model | Description |
|------|-------|-------------|
| `generate_code` | qwen3-coder-next-q40 | First-draft code from a task description. Include relevant file contents as context. |
| `improve_code` | qwen3-coder-next-q40 | Apply review feedback to code. Send both the code and the review. |
| `simplify_code` | qwen3-coder-next-q40 | Remove unnecessary complexity without changing behavior. |
| `review_code` | devstral-small-2-24b-2512-q40 | Review code for bugs, security issues, and improvements. Returns numbered findings with severity levels. |
| `plan_task` | devstral-small-2-24b-2512-q40 | Break a complex task into concrete implementation steps. |

## MCP prompts

These inject a specialized system prompt into the conversation. In Claude Code they appear as slash commands.

| Prompt | Phase | Description |
|--------|-------|-------------|
| `/brainstorm` | 1 | Collaborative problem exploration. No code. Surfaces edge cases and alternatives. |
| `/plan` | 2 | Produces a structured `plan.md` with ordered steps and acceptance criteria. |
| `/orchestrate` | 3 | Executes `plan.md` step by step using the sub-agent tools, with review loops and git commits per step. |

## Setup

### Requirements

- Node.js 18+
- A local LLM server with an OpenAI-compatible API at `http://localhost:8080` (or configure `LLAMA_BASE_URL`)

### Install

```bash
cd ~/orchestrator
npm install
```

### Register with Claude Code

Add to `~/.mcp.json`:

```json
{
  "mcpServers": {
    "local-llm": {
      "command": "node",
      "args": ["/home/aristath/orchestrator/server.js"]
    }
  }
}
```

The server reads `LLAMA_BASE_URL` from the environment (default: `http://localhost:8080`).

## Project structure

```
orchestrator/
├── server.js          # MCP server — registers tools and prompts from .md files
├── CLAUDE.md          # Delegation guidelines for Claude Code
├── roles/             # Sub-agent system prompts → registered as MCP tools
│   ├── coder.md       → generate_code
│   ├── reviewer.md    → review_code
│   ├── improver.md    → improve_code
│   ├── simplifier.md  → simplify_code
│   └── planner.md     → plan_task
└── prompts/           # Session prompts → registered as MCP prompts
    ├── brainstorm.md  → /brainstorm
    ├── plan.md        → /plan
    └── orchestrate.md → /orchestrate
```

The server reloads role and prompt files on every request — edit them without restarting.

## Adding roles and prompts

**New tool** — create a file in `roles/`:

```markdown
---
name: tool_name
description: What this tool does (shown to the calling agent)
model: model-id-from-llama-router
temperature: 0.4
---

System prompt for the local LLM goes here.
```

**New prompt/command** — create a file in `prompts/`:

```markdown
---
name: command_name
description: What this command does
---

Prompt content injected into the conversation.
```

The `model` field in role files must match a model ID registered in the llama-router. If omitted, the request is sent without a model name and the router will reject it.
