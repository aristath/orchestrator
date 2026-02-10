# Orchestrator

## What This Is

A multi-agent orchestrator for agentic coding that coordinates specialized AI agents across multiple backends — Claude Code, OpenAI Codex, and local LLMs via Ollama/LM Studio/llama.cpp. Built as a fork of Crush (Charmbracelet's terminal coding agent), it adds hub-and-spoke multi-agent orchestration: one orchestrator agent holds the full plan context and spawns satellite agents for sub-tasks, routing each task to the best backend and model for the job.

## Core Value

The orchestrator enables a single developer to plan a task with an AI, then have that AI autonomously decompose and execute the plan across multiple specialized agents running in parallel — coordinating them, answering their questions, and ensuring quality through review and testing agents — all from a terminal TUI.

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

(None yet — ship to validate)

### Active

<!-- Current scope. Building toward these. -->

- [ ] User can define custom agent roles via config (system prompt + model + backend + tool set per role)
- [ ] Orchestrator agent holds full plan context and manages task execution
- [ ] Orchestrator decomposes a plan into a DAG of sub-tasks with dependencies
- [ ] Orchestrator spawns satellite agents for each sub-task
- [ ] Orchestrator routes tasks to the appropriate backend (Claude Code, Codex, or local LLM)
- [ ] Orchestrator selects the best model for each task based on agent role config
- [ ] Satellite agents using Claude Code communicate via CLI (`claude -p` with `--session-id`/`--resume` for multi-turn)
- [ ] Satellite agents using Codex communicate via CLI (`codex exec` with `resume <THREAD_ID>` for multi-turn)
- [ ] Satellite agents using local LLMs communicate via OpenAI-compatible API with built-in agentic tool loop
- [ ] Local LLM agents have full tool access (file read/write, shell, grep, glob)
- [ ] Parallel execution of independent tasks (task A depends on B & C; B depends on D; so D & C run simultaneously)
- [ ] Orchestrator answers satellite agent questions/clarifications using its full plan context
- [ ] After each task completes, orchestrator can spawn follow-up agents (reviewer, tester) per workflow config
- [ ] TUI displays split panes showing parallel agent activity simultaneously
- [ ] Predefined workflows configurable (e.g., code -> review -> test pipeline)
- [ ] Orchestrator itself can run on any backend (Claude Code, Codex, or local LLM — configurable)
- [ ] All multi-turn conversations maintain full context across turns

### Out of Scope

- Mobile or web UI — terminal TUI only
- Building a new LLM provider abstraction — reuse Crush's `charm.land/fantasy` for local LLMs
- Cloud hosting or SaaS deployment — this is a local developer tool
- Real-time collaboration between multiple human users
- Custom model training or fine-tuning integration

## Context

**Starting point:** Fork of Crush (Charmbracelet), a Go-based terminal coding agent built on Bubble Tea. Crush already provides:
- Full Bubble Tea TUI
- Complete tool system (file edit, grep, glob, bash, LSP, MCP)
- LLM abstraction via `charm.land/fantasy` (OpenAI, Anthropic, OpenAI-compat for local models, Google, Bedrock, etc.)
- Session/message persistence (SQLite)
- Config system (providers, models, permissions)
- `SessionAgent` interface with `SetSystemPrompt()`, `SetModels()`, `SetTools()`
- Coordinator with `agents map[string]SessionAgent` (multi-agent ready but unused)
- Non-interactive mode (`crush run`)
- Permission system (tool approval / yolo mode)

**Key Crush code pointers for multi-agent extension:**
- `internal/config/config.go:67-68` — `AgentCoder` and `AgentTask` constants (expand to user-defined roles)
- `internal/config/config.go:744-768` — `SetupAgents()` creates agent definitions with per-agent tool lists
- `internal/agent/coordinator.go:47` — commented-out `SetMainAgent` method
- `internal/agent/coordinator.go:103` — `// TODO: make this dynamic when we support multiple agents`
- `internal/agent/agent.go` — `SessionAgent` interface already supports `SetSystemPrompt()`, `SetModels()`, `SetTools()`

**External backend multi-turn capabilities:**
- Claude Code: `--session-id <uuid>` creates session, `--resume <session-id>` continues it, `--output-format json` for structured output, `--system-prompt` for role customization
- Codex CLI: `codex exec --json` returns `thread_id`, `codex exec resume <THREAD_ID>` continues conversation
- Codex TypeScript SDK: `startThread()` / `thread.run()` for programmatic multi-turn
- Local LLMs: orchestrator manages conversation history directly via OpenAI-compatible API

**Inspiration and prior art:**
- Crush (Charmbracelet) — single-agent TUI with excellent infrastructure
- Claude Code — multi-turn non-interactive mode with session management
- OpenAI Codex — thread-based conversations with TypeScript SDK
- The GSD workflow system (used in this project's initialization) — demonstrates effective orchestrator/satellite agent patterns

## Constraints

- **Tech stack**: Go (fork of Crush) — leverages existing Bubble Tea TUI, `charm.land/fantasy` LLM abstraction, and tool system
- **License**: Crush is FSL-1.1-MIT — "Competing Use" restricted for 2 years, personal/internal use explicitly permitted
- **External dependencies**: Claude Code CLI and Codex CLI must be installed separately for those backends; local LLM servers (Ollama, LM Studio, llama.cpp) must be running for local backend
- **Architecture**: Hub-and-spoke — orchestrator is the only agent with full plan context; satellites only know their specific task

## Key Decisions

<!-- Decisions that constrain future work. Add throughout project lifecycle. -->

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Fork Crush rather than build from scratch | Crush provides 80% of needed infrastructure (TUI, tools, LLM abstraction, session management). Multi-agent support is partially scaffolded in the codebase. | — Pending |
| Use Claude Code and Codex CLIs as subprocess backends | Avoids needing separate API keys — piggybacks on existing subscriptions. Both support multi-turn via session/thread IDs. | — Pending |
| OpenAI-compatible API for local LLMs | Universal protocol — Ollama, LM Studio, llama.cpp all expose this. One integration covers all local backends. | — Pending |
| Hub-and-spoke agent architecture | Orchestrator holds full context, satellites are stateless workers. Keeps satellite agents simple and focused. Orchestrator handles inter-agent coordination. | — Pending |
| DAG-based task scheduling | Dependency resolution enables maximum parallelism (run independent tasks simultaneously) while respecting ordering constraints. | — Pending |
| Split-pane TUI for parallel agents | User can monitor all running agents simultaneously. Natural fit for Bubble Tea's component model. | — Pending |

---
*Last updated: 2026-02-10 after initialization*
