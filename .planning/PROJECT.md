# Orchestrator

## What This Is

A standalone multi-agent orchestrator for agentic coding that coordinates specialized AI agents across multiple backends — Claude Code, OpenAI Codex, and Goose (for local LLMs via Ollama/LM Studio/llama.cpp). Built in Go with a Bubble Tea TUI, it uses hub-and-spoke multi-agent orchestration: one orchestrator agent holds the full plan context and spawns satellite agents for sub-tasks, routing each task to the best backend and model for the job. All backends are subprocess-based — the orchestrator communicates with agent CLIs that already have tools, multi-turn support, and structured output built in.

## Core Value

The orchestrator enables a single developer to plan a task with an AI, then have that AI autonomously decompose and execute the plan across multiple specialized agents running in parallel — coordinating them, answering their questions, and ensuring quality through review and testing agents — all from a terminal TUI.

## Requirements

### Validated

- ✓ Providers defined in JSON config — transport layer (CLI command, args, base config) — v1.0
- ✓ Agents defined in JSON config — role layer (provider, model, system prompt, tools per role) — v1.0
- ✓ Orchestrator agent holds full plan context and manages task execution — v1.0
- ✓ Orchestrator decomposes a plan into a DAG of sub-tasks with dependencies — v1.0
- ✓ Orchestrator spawns satellite agents for each sub-task — v1.0
- ✓ Orchestrator routes tasks to the appropriate backend (Claude Code, Codex, or Goose) — v1.0
- ✓ Orchestrator selects the best agent for each task based on role config — v1.0
- ✓ Satellite agents using Claude Code communicate via CLI with session management — v1.0
- ✓ Satellite agents using Codex communicate via CLI with thread management — v1.0
- ✓ Satellite agents using Goose communicate via CLI with session management — v1.0
- ✓ Goose backend supports local LLMs (Ollama, LM Studio, llama.cpp) — v1.0
- ✓ Parallel execution of independent tasks with bounded concurrency — v1.0
- ✓ Orchestrator answers satellite agent questions via non-blocking Q&A channel — v1.0
- ✓ Follow-up agents (reviewer, tester) spawned per workflow config — v1.0
- ✓ TUI displays split panes showing parallel agent activity simultaneously — v1.0
- ✓ Predefined workflows configurable (code -> review -> test pipeline) — v1.0
- ✓ Orchestrator is a configurable agent definition — provider and model are swappable — v1.0
- ✓ All multi-turn conversations maintain full context across turns — v1.0
- ✓ Task state persists to SQLite — survives crashes and restarts — v1.0
- ✓ Conversation history per agent stored and recoverable — v1.0
- ✓ Checkpoint/resume from last completed task — v1.0
- ✓ Session IDs persisted for conversation continuity — v1.0
- ✓ Transient failures retried with exponential backoff — v1.0
- ✓ Circuit breakers prevent repeated calls to failing backends — v1.0
- ✓ One agent's failure does not cascade to unrelated agents — v1.0
- ✓ Graceful shutdown on Ctrl+C with subprocess cleanup — v1.0

### Active

(Fresh for next milestone — define via `/gsd:new-milestone`)

### Out of Scope

- Mobile or web UI — terminal TUI only, this is a local developer tool
- Building custom LLM provider abstraction — agent CLIs handle LLM communication
- Building custom tool systems — agent CLIs have tools built in
- Cloud hosting or SaaS deployment — this is a local developer tool
- Real-time collaboration between multiple human users
- Custom model training or fine-tuning integration
- Real-time multi-agent file editing — use task-level parallelism with file ownership instead
- Unlimited agent spawning — bound to 2-4 concurrent agents (coordination complexity grows O(n^2))

## Context

**Current state:** v1.0 shipped. 11,828 lines of Go across 6 phases.

**Tech stack:** Go, Bubble Tea v1.x, Lip Gloss, Huh (forms), modernc.org/sqlite (pure-Go), cenkalti/backoff, sony/gobreaker, gammazero/toposort.

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

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Build standalone rather than fork Crush | Avoids fork maintenance burden. Agent CLIs provide tools, multi-turn, and structured output. | ✓ Good |
| Use agent CLIs as subprocess backends | Each CLI has tools, model selection, multi-turn sessions, and structured output. Adding new backends = thin adapter. | ✓ Good |
| Goose for local LLMs | Supports Ollama/LM Studio/llama.cpp with full CLI capabilities. | ✓ Good |
| Hub-and-spoke agent architecture | Orchestrator holds full context, satellites are stateless workers. | ✓ Good |
| DAG-based task scheduling | Dependency resolution enables maximum parallelism while respecting ordering. | ✓ Good |
| Split-pane TUI for parallel agents | User monitors all running agents simultaneously. Natural fit for Bubble Tea. | ✓ Good |
| Bubble Tea v1.x (not v2 beta) | Production reliability over bleeding-edge features. | ✓ Good |
| modernc.org/sqlite (pure Go) | No CGO dependency, simpler cross-compilation. | ✓ Good |
| Plain errgroup.Group for failure isolation | One task's failure doesn't cancel siblings — correct behavior for independent agents. | ✓ Good |
| Per-backend-type circuit breakers | Right granularity — if Claude is down, still try Codex. Per-task too fine, global too coarse. | ✓ Good |
| WAL mode + busy_timeout for SQLite | Handles concurrent agent writes without contention. | ✓ Good |

---
*Last updated: 2026-02-10 after v1.0 milestone*
