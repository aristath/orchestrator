# Requirements: Orchestrator

**Defined:** 2026-02-10
**Core Value:** A single developer plans a task with an AI, then that AI autonomously decomposes and executes the plan across multiple specialized agents running in parallel — coordinating them, answering their questions, and ensuring quality.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Fork Strategy (FORK)

- [ ] **FORK-01**: Crush source code is forked with a clean extension architecture that wraps rather than modifies core files
- [ ] **FORK-02**: Extension points are documented — interfaces for injecting orchestrator behavior without touching Crush internals
- [ ] **FORK-03**: Fork can rebase cleanly against upstream Crush (weekly rebase target)

### Backend Abstraction (BACK)

- [ ] **BACK-01**: A unified `Backend` interface abstracts all LLM communication (in-process, subprocess, network)
- [ ] **BACK-02**: Fantasy adapter wraps Crush's existing `charm.land/fantasy` LLM calls for in-process backends
- [ ] **BACK-03**: Subprocess adapter executes Claude Code CLI with `--session-id`/`--resume` for multi-turn conversations
- [ ] **BACK-04**: Subprocess adapter executes Codex CLI with `resume <THREAD_ID>` for multi-turn conversations
- [ ] **BACK-05**: Subprocess pipes (stdout/stderr) are read concurrently via goroutines before `cmd.Wait()` to prevent deadlocks
- [ ] **BACK-06**: Process groups (`syscall.SysProcAttr{Setpgid: true}`) ensure signal propagation kills entire subprocess trees
- [ ] **BACK-07**: Zombie process prevention — `cmd.Wait()` is always called, orphan detection runs on shutdown

### Agent Definitions (AGNT)

- [ ] **AGNT-01**: User defines agent roles via YAML config (system prompt, model, backend, tool set per role)
- [ ] **AGNT-02**: Predefined roles ship as defaults: orchestrator, coder, reviewer, tester
- [ ] **AGNT-03**: Each agent role maps to a specific backend and model selection
- [ ] **AGNT-04**: Agent tool sets are configurable per role (e.g., reviewer gets read-only tools, coder gets full tools)
- [ ] **AGNT-05**: Orchestrator agent holds full plan context and manages the entire task execution lifecycle

### DAG Scheduling (SCHED)

- [ ] **SCHED-01**: Orchestrator decomposes a plan into a DAG of sub-tasks with explicit dependencies
- [ ] **SCHED-02**: DAG validates against circular dependencies via topological sort before execution
- [ ] **SCHED-03**: Independent tasks (no unresolved dependencies) are eligible for parallel execution
- [ ] **SCHED-04**: Task completion triggers dependency resolution — downstream tasks become eligible when all upstream tasks complete
- [ ] **SCHED-05**: File-level resource locking prevents multiple agents from writing the same file simultaneously
- [ ] **SCHED-06**: Failure classification (hard/soft/skip) determines whether failures block dependents or allow continuation

### Parallel Execution (EXEC)

- [ ] **EXEC-01**: 2-4 agents execute concurrently with bounded concurrency via `errgroup.SetLimit()`
- [ ] **EXEC-02**: Each parallel agent operates in an isolated git worktree to prevent file conflicts
- [ ] **EXEC-03**: Completed agent work is merged back from worktree branches via configurable merge strategy
- [ ] **EXEC-04**: Orchestrator answers satellite agent questions/clarifications using its full plan context

### TUI (TUI)

- [ ] **TUI-01**: Split-pane layout displays parallel agent activity simultaneously
- [ ] **TUI-02**: Each agent has a dedicated viewport showing real-time output (logs, status, progress)
- [ ] **TUI-03**: Vim-style navigation for switching focus between agent panes
- [ ] **TUI-04**: Agent status indicators visible at a glance (working, paused, failed, complete)
- [ ] **TUI-05**: Orchestrator pane shows DAG progress and overall task status

### State Management (STATE)

- [ ] **STATE-01**: Task state (DAG, status, results) persists to SQLite — survives crashes and restarts
- [ ] **STATE-02**: Conversation history per agent is stored and recoverable
- [ ] **STATE-03**: Completed tasks are checkpointed — resume picks up from last checkpoint, not from scratch
- [ ] **STATE-04**: Multi-turn session IDs (Claude Code, Codex) are persisted for conversation continuity across restarts

### Resilience (RESIL)

- [ ] **RESIL-01**: Transient failures (API timeouts, rate limits) retry with exponential backoff and jitter
- [ ] **RESIL-02**: Circuit breakers prevent repeated calls to a failing backend
- [ ] **RESIL-03**: One agent's failure does not cascade to abort unrelated parallel agents
- [ ] **RESIL-04**: Graceful shutdown on Ctrl+C — all subprocess trees are killed, partial work is checkpointed

### Workflows (WORK)

- [ ] **WORK-01**: Predefined workflows are configurable (e.g., code -> review -> test pipeline)
- [ ] **WORK-02**: After each task completes, orchestrator can spawn follow-up agents (reviewer, tester) per workflow config
- [ ] **WORK-03**: Orchestrator itself can run on any backend (Claude Code, Codex, or local LLM — configurable)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Local LLM Backend

- **LOCAL-01**: Network backend adapter for OpenAI-compatible API (Ollama, LM Studio, llama.cpp)
- **LOCAL-02**: Local LLM agents have full tool access (file read/write, shell, grep, glob) via built-in agentic tool loop
- **LOCAL-03**: Orchestrator manages conversation history directly for local LLM agents (no external session management)

### Advanced Features

- **ADV-01**: Inter-agent message passing via pub-sub beyond orchestrator relay
- **ADV-02**: Smart task rebalancing when agents block — redistribute pending work
- **ADV-03**: Context compression for long sessions to prevent token overflow
- **ADV-04**: Quality gate orchestration with configurable thresholds (coverage, security scan)
- **ADV-05**: Dynamic agent spawning based on DAG state (create agents on-demand as tasks become ready)
- **ADV-06**: Specialist-agent auto-review panels (multi-model review for correctness, security, performance)
- **ADV-07**: Hierarchical memory (user/session/agent levels) with cross-session recall
- **ADV-08**: MCP + A2A protocol interoperability for external agent systems
- **ADV-09**: Cross-session project memory via git history analysis and structured notes
- **ADV-10**: Multi-backend routing per individual agent (cost optimization)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Mobile or web UI | Terminal TUI only — this is a local developer tool |
| New LLM provider abstraction | Reuse Crush's `charm.land/fantasy` — already supports all needed providers |
| Cloud hosting / SaaS | Local tool, not a service |
| Real-time multi-user collaboration | Single developer tool |
| Model training / fine-tuning | Out of domain — use existing models |
| Real-time multi-agent file editing | Race conditions and merge conflicts destroy efficiency — use task-level parallelism with file ownership instead |
| Unlimited agent spawning | Coordination complexity grows O(n^2) — bound to 2-4 concurrent agents in v1 |
| Full conversation history between agents | Token costs explode, agents lose focus — use summarized task context instead |
| Fully autonomous operation without human gates | Silent failures compound — require human approval at critical gates |
| Agent personality traits | Noise in professional workflows — stick to role, goal, constraints |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FORK-01 | Phase 0 | Pending |
| FORK-02 | Phase 0 | Pending |
| FORK-03 | Phase 0 | Pending |
| BACK-01 | Phase 1 | Pending |
| BACK-02 | Phase 1 | Pending |
| BACK-03 | Phase 1 | Pending |
| BACK-04 | Phase 1 | Pending |
| BACK-05 | Phase 1 | Pending |
| BACK-06 | Phase 1 | Pending |
| BACK-07 | Phase 1 | Pending |
| AGNT-01 | Phase 2 | Pending |
| AGNT-02 | Phase 2 | Pending |
| AGNT-03 | Phase 2 | Pending |
| AGNT-04 | Phase 2 | Pending |
| AGNT-05 | Phase 2 | Pending |
| SCHED-01 | Phase 2 | Pending |
| SCHED-02 | Phase 2 | Pending |
| SCHED-03 | Phase 2 | Pending |
| SCHED-04 | Phase 2 | Pending |
| SCHED-05 | Phase 2 | Pending |
| SCHED-06 | Phase 2 | Pending |
| WORK-01 | Phase 2 | Pending |
| WORK-02 | Phase 2 | Pending |
| WORK-03 | Phase 2 | Pending |
| EXEC-01 | Phase 3 | Pending |
| EXEC-02 | Phase 3 | Pending |
| EXEC-03 | Phase 3 | Pending |
| EXEC-04 | Phase 3 | Pending |
| TUI-01 | Phase 4 | Pending |
| TUI-02 | Phase 4 | Pending |
| TUI-03 | Phase 4 | Pending |
| TUI-04 | Phase 4 | Pending |
| TUI-05 | Phase 4 | Pending |
| STATE-01 | Phase 5 | Pending |
| STATE-02 | Phase 5 | Pending |
| STATE-03 | Phase 5 | Pending |
| STATE-04 | Phase 5 | Pending |
| RESIL-01 | Phase 6 | Pending |
| RESIL-02 | Phase 6 | Pending |
| RESIL-03 | Phase 6 | Pending |
| RESIL-04 | Phase 6 | Pending |

**Coverage:**
- v1 requirements: 41 total
- Mapped to phases: 41
- Unmapped: 0
- v2 requirements (Phase 7 stretch): LOCAL-01, LOCAL-02, LOCAL-03

---
*Requirements defined: 2026-02-10*
*Last updated: 2026-02-10 after roadmap creation*
