# Orchestrator

## What This Is

A standalone multi-agent orchestrator for agentic coding that coordinates specialized AI agents across multiple backends — Claude Code, OpenAI Codex, and Goose (for local LLMs via Ollama/LM Studio/llama.cpp). Built in Go with a Bubble Tea TUI, it uses hub-and-spoke multi-agent orchestration: one orchestrator agent holds the full plan context and spawns satellite agents for sub-tasks, routing each task to the best backend and model for the job. All backends are subprocess-based — the orchestrator communicates with agent CLIs that already have tools, multi-turn support, and structured output built in.

## Core Value

The orchestrator enables a single developer to plan a task with an AI, then have that AI autonomously decompose and execute the plan across multiple specialized agents running in parallel — coordinating them, answering their questions, and ensuring quality through review and testing agents — all from a terminal TUI.

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

(None yet — ship to validate)

### Active

<!-- Current scope. Building toward these. -->

- [ ] Providers defined in JSON config — transport layer (CLI command, args, base config)
- [ ] Agents defined in JSON config — role layer (provider, model, system prompt, tools per role)
- [ ] Orchestrator agent holds full plan context and manages task execution
- [ ] Orchestrator decomposes a plan into a DAG of sub-tasks with dependencies
- [ ] Orchestrator spawns satellite agents for each sub-task
- [ ] Orchestrator routes tasks to the appropriate backend (Claude Code, Codex, or Goose)
- [ ] Orchestrator selects the best agent for each task based on role config (e.g., Opus for reviews, GPT for CSS, Qwen for HTML)
- [ ] Satellite agents using Claude Code communicate via CLI (`claude -p` with `--session-id`/`--resume` for multi-turn)
- [ ] Satellite agents using Codex communicate via CLI (`codex exec` with `resume <THREAD_ID>` for multi-turn)
- [ ] Satellite agents using Goose communicate via CLI (`goose run` with `--session-id`/`--resume` for multi-turn)
- [ ] Goose backend supports local LLMs (Ollama, LM Studio, llama.cpp) via its built-in provider system
- [ ] Parallel execution of independent tasks (task A depends on B & C; B depends on D; so D & C run simultaneously)
- [ ] Orchestrator answers satellite agent questions/clarifications using its full plan context
- [ ] After each task completes, orchestrator can spawn follow-up agents (reviewer, tester) per workflow config
- [ ] TUI displays split panes showing parallel agent activity simultaneously
- [ ] Predefined workflows configurable (e.g., code -> review -> test pipeline)
- [ ] Orchestrator is a configurable agent definition — provider and model are swappable, nothing hardcoded
- [ ] All multi-turn conversations maintain full context across turns

### Out of Scope

- Mobile or web UI — terminal TUI only
- Building custom LLM provider abstraction — agent CLIs handle LLM communication
- Building custom tool systems — agent CLIs have tools built in
- Cloud hosting or SaaS deployment — this is a local developer tool
- Real-time collaboration between multiple human users
- Custom model training or fine-tuning integration

## Context

**Architecture:** Standalone Go binary with Bubble Tea TUI. All LLM interaction happens through subprocess agent CLIs — the orchestrator never calls LLM APIs directly. Each agent CLI already provides tools (file edit, shell, etc.), model selection, multi-turn sessions, and structured output. The orchestrator's job is coordination: planning, scheduling, monitoring, and quality control.

**Subprocess backends and their CLI capabilities:**
- Claude Code: `claude -p "prompt" --session-id <uuid>` creates session, `--resume <session-id>` continues it, `--output-format json` for structured output, `--system-prompt` for role customization
- Codex CLI: `codex exec --json` returns `thread_id`, `codex exec resume <THREAD_ID>` continues conversation
- Goose: `goose run --text "prompt" --output-format json -q` for non-interactive, `--system` for custom system prompt, `--session-id`/`--resume` for multi-turn, `--model`/`--provider` for model selection, supports Ollama/LM Studio/llama.cpp

**Key advantage of subprocess approach:**
- No API keys needed for Claude Code or Codex — piggybacks on existing subscriptions
- No custom tool implementation — each agent CLI has file edit, shell, grep, etc. built in
- No LLM provider abstraction needed — each CLI handles its own model communication
- Multi-turn support comes free from each CLI's session management
- Can add new agent CLI backends (OpenCode, Cline, etc.) by implementing a thin subprocess adapter

**Inspiration and prior art:**
- Crush (Charmbracelet) — TUI design inspiration (Bubble Tea, split panes, vim navigation)
- Claude Code — multi-turn non-interactive mode with session management
- OpenAI Codex — thread-based conversations
- Goose (Block) — local LLM support with full agentic capabilities
- The GSD workflow system — demonstrates effective orchestrator/satellite agent patterns

## Constraints

- **Tech stack**: Go + Bubble Tea for TUI — proven ecosystem for terminal applications
- **Backend CLIs**: Claude Code CLI, Codex CLI, and Goose must be installed separately; local LLM servers (Ollama, LM Studio, llama.cpp) must be running for Goose's local backend
- **Architecture**: Hub-and-spoke — orchestrator is the only agent with full plan context; satellites only know their specific task
- **All backends are subprocesses**: The orchestrator never calls LLM APIs directly; it delegates to agent CLIs

## Key Decisions

<!-- Decisions that constrain future work. Add throughout project lifecycle. -->

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Build standalone rather than fork Crush | Avoids fork maintenance burden. Agent CLIs (Claude Code, Codex, Goose) already provide tools, multi-turn, and structured output — no need to reuse Crush internals. We only need Crush's TUI design as inspiration. | — Pending |
| Use agent CLIs as subprocess backends | Each CLI already has tools, model selection, multi-turn sessions, and structured output. No need to build LLM provider abstraction or tool systems. Adding new backends = implementing a thin adapter. | — Pending |
| Goose for local LLMs | Goose supports Ollama/LM Studio/llama.cpp with full agentic tools, custom system prompts, multi-turn, and JSON output — all from CLI. No need to build a local LLM integration ourselves. | — Pending |
| Hub-and-spoke agent architecture | Orchestrator holds full context, satellites are stateless workers. Keeps satellite agents simple and focused. Orchestrator handles inter-agent coordination. | — Pending |
| DAG-based task scheduling | Dependency resolution enables maximum parallelism (run independent tasks simultaneously) while respecting ordering constraints. | — Pending |
| Split-pane TUI for parallel agents | User can monitor all running agents simultaneously. Natural fit for Bubble Tea's component model. | — Pending |

---
*Last updated: 2026-02-10 after architecture pivot to standalone (no Crush fork)*
