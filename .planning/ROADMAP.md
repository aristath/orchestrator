# Roadmap: Orchestrator

## Overview

This roadmap delivers a standalone multi-agent orchestrator for agentic coding, built in Go with a Bubble Tea TUI. All LLM interaction happens through subprocess agent CLIs (Claude Code, Codex, Goose) — the orchestrator never calls LLM APIs directly. The journey starts with subprocess management and backend abstraction (Phase 1), then builds upward through agent definitions, parallel execution, TUI integration, state persistence, and resilience hardening. Phase ordering is driven by dependency chains: subprocess management before agent logic, DAG scheduling before parallel execution, working agents before TUI display, state checkpointing before resilience patterns.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3, ...): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Subprocess Management and Backend Abstraction** - Unified Backend interface with subprocess adapters for Claude Code, Codex, and Goose, solving pipe deadlocks and process lifecycle
- [x] **Phase 2: Agent Definitions and DAG Scheduler** - JSON agent config, DAG construction with topological sort, resource locking, failure classification, and workflow definitions
- [x] **Phase 3: Parallel Execution with Git Isolation** - Bounded concurrent agents in isolated git worktrees with merge-back and orchestrator Q&A
- [x] **Phase 4: Event Bus and TUI Integration** - Split-pane Bubble Tea TUI with per-agent viewports, vim navigation, status indicators, and DAG progress
- [ ] **Phase 5: State Management and Session Persistence** - SQLite persistence for task state, conversation history, checkpointing, and session ID continuity
- [ ] **Phase 6: Resilience and Production Hardening** - Retry with backoff, circuit breakers, failure isolation, and graceful shutdown

## Phase Details

### Phase 1: Subprocess Management and Backend Abstraction
**Goal**: Any agent CLI (Claude Code, Codex, Goose) can be called through a single Backend interface, with subprocess execution that never deadlocks, leaks processes, or fails to propagate signals
**Depends on**: Nothing (first phase)
**Requirements**: BACK-01, BACK-02, BACK-03, BACK-04, BACK-05, BACK-06, BACK-07
**Key Risks**: Subprocess pipe deadlocks (reading stdout/stderr must happen concurrently before cmd.Wait). Zombie processes from missed cmd.Wait calls. Signal propagation failure leaving orphaned agent CLI processes consuming API credits. Each CLI has slightly different JSON output formats requiring per-adapter parsing.
**Success Criteria** (what must be TRUE):
  1. A Backend interface exists with Send/Receive methods that abstract over all subprocess CLI communication
  2. Claude Code adapter can start a session, send a prompt, receive JSON response, and continue the conversation via `--resume`
  3. Codex adapter can start a thread, send a prompt, receive JSON response, and continue the conversation via `resume <THREAD_ID>`
  4. Goose adapter can start a session, send a prompt, receive JSON response, and continue the conversation via `--resume`
  5. Goose adapter supports local LLMs by passing `--model`/`--provider` for Ollama/LM Studio/llama.cpp
  6. Killing the orchestrator process (Ctrl+C or SIGTERM) kills all spawned subprocess trees with no orphaned processes remaining
  7. A stress test of 10+ sequential subprocess invocations leaves zero zombie processes
**Plans**: 5 plans

Plans:
- [x] 01-01-PLAN.md — Go module, Backend interface, shared types, subprocess utilities (ProcessManager, concurrent pipe reading, process groups)
- [x] 01-02-PLAN.md — Claude Code adapter implementation and unit tests
- [x] 01-03-PLAN.md — Codex adapter implementation and unit tests
- [x] 01-04-PLAN.md — Goose adapter with local LLM support and unit tests
- [x] 01-05-PLAN.md — Integration stress tests (zombie prevention, deadlock prevention, signal propagation, factory)

### Phase 2: Agent Definitions and DAG Scheduler
**Goal**: Users can define agent roles via JSON config, and the orchestrator can decompose a plan into a validated DAG of tasks with dependency resolution, resource locking, and failure classification
**Depends on**: Phase 1
**Requirements**: AGNT-01, AGNT-02, AGNT-03, AGNT-04, AGNT-05, AGNT-06, CONF-01, CONF-02, CONF-03, SCHED-01, SCHED-02, SCHED-03, SCHED-04, SCHED-05, SCHED-06, WORK-01, WORK-02, WORK-03
**Key Risks**: DAG cycle detection must be bulletproof — a cycle causes infinite blocking. File-level resource locking adds complexity but is essential before parallel execution. Workflow config (code->review->test) must compose cleanly with DAG scheduling.
**Success Criteria** (what must be TRUE):
  1. Providers are defined in JSON config (CLI command, args, transport config) separately from agents
  2. Agents are defined in JSON config (provider, model, system prompt, tools per role) — e.g., Opus for reviews, GPT for CSS, Qwen for HTML
  3. Default roles (orchestrator, coder, reviewer, tester) ship out of the box and are usable without custom config
  3. Orchestrator agent decomposes a plan into a DAG where each node is a task and edges represent dependencies
  4. DAG rejects circular dependencies at construction time with a clear error message identifying the cycle
  5. Tasks that have no unresolved dependencies are marked eligible for execution, and completing a task triggers downstream dependency resolution
  6. File-level resource locks prevent scheduling two tasks that write the same file concurrently
  7. Global config (`~/.orchestrator/config.json`) and per-project config (`.orchestrator/config.json`) are loaded and merged — project overrides global
  8. Predefined workflows (e.g., code -> review -> test) can be configured and the orchestrator spawns follow-up agents per workflow config
**Plans**: 5 plans

Plans:
- [x] 02-01-PLAN.md — Config types (ProviderConfig, AgentConfig, WorkflowConfig), default definitions, and config loader with global/project merge
- [x] 02-02-PLAN.md — DAG core: task types, graph construction, topological sort with cycle detection, dependency resolution
- [x] 02-03-PLAN.md — Resource lock manager (per-file keyed mutex) and task executor bridging DAG to backends
- [x] 02-04-PLAN.md — Workflow engine: follow-up task spawning after completion per workflow config
- [x] 02-05-PLAN.md — Integration tests validating full pipeline (config -> DAG -> execute -> workflow)

### Phase 3: Parallel Execution with Git Isolation
**Goal**: Multiple agents execute tasks concurrently in isolated git worktrees, with results merged back and the orchestrator answering satellite questions in real time
**Depends on**: Phase 2
**Requirements**: EXEC-01, EXEC-02, EXEC-03, EXEC-04
**Key Risks**: Git worktree merge conflicts when agents modify adjacent code. Bounded concurrency (errgroup.SetLimit) must prevent resource exhaustion. Orchestrator Q&A channel must not block agent execution.
**Success Criteria** (what must be TRUE):
  1. 2-4 agents execute concurrently with bounded concurrency — no more than the configured limit run simultaneously
  2. Each running agent operates in its own git worktree, isolated from other agents' file changes
  3. When an agent completes, its worktree branch is merged back to the main branch via the configured merge strategy
  4. A satellite agent can ask the orchestrator a clarifying question and receive an answer from the orchestrator's full plan context without blocking other running agents
**Plans**: 3 plans

Plans:
- [x] 03-01-PLAN.md — Git worktree lifecycle manager: create, merge, cleanup, list, prune with configurable merge strategy
- [x] 03-02-PLAN.md — Non-blocking Q&A channel for satellite agent to orchestrator communication
- [x] 03-03-PLAN.md — Parallel runner wiring errgroup, worktrees, Q&A channel, and DAG executor with integration tests

### Phase 4: Event Bus and TUI Integration
**Goal**: User can monitor all running agents in a split-pane Bubble Tea TUI with real-time output, navigate between panes, and see overall DAG progress at a glance
**Depends on**: Phase 3
**Requirements**: TUI-01, TUI-02, TUI-03, TUI-04, TUI-05, CONF-04, CONF-05
**Key Risks**: Bubble Tea v2 split-pane layouts are not extensively documented — may need prototyping. TUI state desync between actual agent state and displayed state. High-frequency agent output overwhelming the TUI render loop.
**Research Flags**: Bubble Tea v2 multi-pane layout patterns need prototyping. Crush's TUI can serve as design reference.
**Success Criteria** (what must be TRUE):
  1. TUI displays a split-pane layout where each running agent has a dedicated viewport showing its real-time output
  2. User can switch focus between agent panes using vim-style keybindings (hjkl or similar)
  3. Each agent pane shows a status indicator (working, paused, failed, complete) visible without switching focus to that pane
  4. An orchestrator pane displays overall DAG progress — which tasks are complete, running, and pending
  5. TUI has a settings panel for editing global config (providers, agents, defaults) and per-project config (overrides)
**Plans**: 3 plans

Plans:
- [x] 04-01-PLAN.md — Event bus (channel-based pubsub) and ParallelRunner instrumentation to publish task lifecycle events
- [x] 04-02-PLAN.md — Bubble Tea TUI with split-pane layout, agent viewports, DAG progress pane, and vim-style navigation
- [x] 04-03-PLAN.md — Config save function and Huh-based settings panel integrated into TUI

### Phase 5: State Management and Session Persistence
**Goal**: All task state, conversation history, and session IDs survive crashes and restarts — the orchestrator can resume from the last checkpoint without re-executing completed work
**Depends on**: Phase 4
**Requirements**: STATE-01, STATE-02, STATE-03, STATE-04
**Key Risks**: SQLite write contention from concurrent agents (WAL mode required). Checkpoint granularity — too coarse wastes work on resume, too fine adds overhead. Session ID persistence must handle Claude Code, Codex, and Goose session formats.
**Success Criteria** (what must be TRUE):
  1. Task state (DAG structure, task statuses, results) persists to SQLite and survives an orchestrator crash
  2. Per-agent conversation history is stored and can be retrieved after restart
  3. Killing and restarting the orchestrator resumes from the last completed task checkpoint — completed tasks are not re-executed
  4. Multi-turn session IDs (Claude Code session-id, Codex thread-id, Goose session-id) are persisted so conversations can continue across restarts
**Plans**: 3 plans

Plans:
- [ ] 05-01-PLAN.md — Store interface, SQLite schema, and task DAG persistence methods with tests
- [ ] 05-02-PLAN.md — Session ID and conversation history persistence methods with tests
- [ ] 05-03-PLAN.md — Wire Store into ParallelRunner for checkpointing and implement Resume from persisted state

### Phase 6: Resilience and Production Hardening
**Goal**: Transient failures are retried automatically, persistently failing backends are circuit-broken, one agent's failure does not cascade to unrelated agents, and shutdown is graceful
**Depends on**: Phase 5
**Requirements**: RESIL-01, RESIL-02, RESIL-03, RESIL-04
**Key Risks**: Retry logic interacting badly with rate limits (exponential backoff must include jitter). Circuit breaker thresholds need tuning — too sensitive causes false trips, too lenient allows cascading failures.
**Success Criteria** (what must be TRUE):
  1. A transient API failure (timeout, rate limit) triggers automatic retry with exponential backoff and jitter — the task eventually succeeds without user intervention
  2. A persistently failing backend trips a circuit breaker that stops new requests to that backend until it recovers
  3. One agent failing does not cause unrelated parallel agents to abort — they continue executing independently
  4. Pressing Ctrl+C triggers graceful shutdown: all subprocess trees are killed, partial work is checkpointed, and the orchestrator exits cleanly
**Plans**: TBD

Plans:
- [ ] 06-01: TBD
- [ ] 06-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5 -> 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Subprocess Management and Backend Abstraction | 5/5 | ✓ Complete | 2026-02-10 |
| 2. Agent Definitions and DAG Scheduler | 5/5 | ✓ Complete | 2026-02-10 |
| 3. Parallel Execution with Git Isolation | 3/3 | ✓ Complete | 2026-02-10 |
| 4. Event Bus and TUI Integration | 3/3 | ✓ Complete | 2026-02-10 |
| 5. State Management and Session Persistence | 0/3 | Not started | - |
| 6. Resilience and Production Hardening | 0/TBD | Not started | - |
