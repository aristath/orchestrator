# Project Research Summary

**Project:** Orchestrator — Multi-Agent AI Coding Orchestrator (Crush Fork)
**Domain:** Go-based TUI multi-agent orchestration for agentic coding
**Researched:** 2026-02-10
**Confidence:** HIGH

## Executive Summary

This project is a hub-and-spoke multi-agent orchestrator built as a fork of Crush (Charmbracelet). The research confirms this is a well-charted domain in 2026: the Charm ecosystem (Bubble Tea v2, Fantasy, Lipgloss) provides a production-grade TUI and LLM abstraction foundation, and Crush itself already scaffolds multi-agent support (the `Coordinator` struct, `agents map[string]SessionAgent`, per-agent tool lists). The recommended approach is to extend Crush through clean interface boundaries rather than rewriting its internals, using an adapter-based backend abstraction to unify in-process LLM calls (Fantasy), subprocess CLIs (Claude Code, Codex), and OpenAI-compatible network APIs (local LLMs) behind a single `Backend` interface. DAG-based task scheduling with `errgroup` for bounded parallel execution and Go's standard `os/exec` for subprocess management round out the core stack.

The critical risk is **fork drift from upstream Crush**. Every research dimension reinforces this: the architecture research recommends wrapping Crush's existing code rather than modifying it; the pitfalls research ranks fork drift as a HIGH recovery cost item (months of work if it happens); and the stack research notes that Fantasy compatibility with newer OpenAI SDK versions is already uncertain. The mitigation is a "Phase 0" fork strategy that establishes extension points, keeps Crush core files unmodified, and maintains weekly rebase discipline from day one. The second major risk cluster is subprocess management — pipe deadlocks, zombie processes, signal propagation failure, and orphaned agents are all well-documented failure modes that must be solved in the very first implementation phase before any multi-agent logic is built.

The feature research validates the MVP scope: agent role definitions, basic DAG scheduling, parallel execution (2-4 agents), split-pane TUI, backend abstraction (cloud + local), error recovery, session persistence, and git worktree isolation. This is a focused, achievable v1 that proves the core hypothesis — hub-and-spoke orchestration outperforms serial single-agent coding. Advanced features (hierarchical memory, dynamic agent spawning, MCP/A2A protocols, specialist auto-review) are correctly deferred to v2+. The anti-features list is equally important: avoid real-time multi-agent file collaboration, unlimited agent spawning, full conversation history passing between agents, and fully autonomous operation without human gates.

## Key Findings

### Recommended Stack

The Charm ecosystem is the clear choice, providing a cohesive, battle-tested toolkit that Crush already uses. Fantasy v0.7.1 handles multi-provider LLM abstraction for OpenAI, Anthropic, Google, Bedrock, Azure, and OpenAI-compatible endpoints. The stack is Go-native with zero C dependencies (CGo-free SQLite via ncruces/go-sqlite3).

**Core technologies:**
- **Bubble Tea v2 + Bubbles v2 + Lipgloss v2**: TUI framework — Elm Architecture, 25k+ apps in production, split-pane layout via viewport component
- **Fantasy v0.7.1**: Multi-provider LLM abstraction — unified API across all required backends, OpenAI-compatible layer for local LLMs
- **errgroup (x/sync)**: Goroutine orchestration — context-based cancellation, bounded concurrency via SetLimit(), idiomatic Go
- **os/exec (stdlib)**: Subprocess management — sufficient for Claude Code/Codex CLI execution with proper pipe handling patterns
- **ncruces/go-sqlite3**: Persistence — CGo-free, cross-platform, matches Crush's existing dependency profile
- **MCP Go SDK v1.2.0**: Tool/context protocol — official SDK maintained with Google, for standardized tool integration
- **AkihiroSuda/go-dag or heimdalr/dag**: DAG scheduling — lightweight, minimal dependencies (prototype both before committing)

**Open question:** Fantasy v0.7.1 uses openai-go v2 (Crush pins v2.7.1), but openai-go v3 is latest. Verify Fantasy compatibility with v3 before upgrading. Also verify whether openai-go v3 supports custom base URLs for local LLM endpoints.

### Expected Features

**Must have (table stakes / MVP):**
- Agent role definitions via YAML config (system prompt, model, backend, tools per role)
- Task queue with DAG scheduling and dependency resolution
- Parallel execution of 2-4 agents with git worktree isolation
- Split-pane TUI with real-time agent monitoring and vim-style navigation
- Backend abstraction covering Claude Code, Codex, and local LLMs
- Error recovery with exponential backoff and circuit breakers
- Session/context persistence surviving crashes and interruptions
- Git integration for agent isolation (worktrees) and work output (commits)

**Should have (v1.x after validation):**
- Local LLM support (Ollama, LM Studio) via OpenAI-compatible API
- Inter-agent message passing (pub-sub beyond orchestrator relay)
- Smart task rebalancing when agents block
- Context compression for long sessions
- Quality gate orchestration (coverage, security scan thresholds)

**Defer (v2+):**
- Specialist-agent auto-review panels (multi-model review)
- Hierarchical memory with knowledge graph
- Dynamic agent spawning based on DAG state
- MCP + A2A protocol interoperability
- Cross-session project memory
- Multi-backend routing per individual agent

### Architecture Approach

The architecture follows a four-layer design: TUI Layer (Bubble Tea MUV) at top, Orchestration Layer (Coordinator hub + Agent spokes + DAG Scheduler) in the middle, Backend Abstraction Layer (adapter pattern: Fantasy in-process, subprocess CLI, network API) below, and State Management (task graph, conversation history, metrics) at the foundation. An Event Bus decouples all layers via pub/sub channels. The project structure isolates concerns into `coordinator/`, `agent/`, `backend/`, `subprocess/`, `state/`, `events/`, `tui/`, and `tools/` packages under `internal/`.

**Major components:**
1. **Coordinator (Hub)** — task decomposition into DAG, agent lifecycle management, dependency resolution, result aggregation
2. **Agent + Backend Adapters** — interface-based abstraction; SessionAgent wraps Backend (Fantasy, Subprocess, Network) with conversation state
3. **DAG Scheduler** — topological sort for execution order, errgroup for bounded parallel goroutines, work-stealing for load balancing
4. **Subprocess Manager** — JSON-RPC 2.0 over stdin/stdout for Claude Code and Codex CLI, process groups for signal propagation
5. **Event Bus** — channel-based pub/sub connecting agents to TUI and metrics without coupling
6. **State Store** — centralized conversation history, task results, and agent metrics with proper locking
7. **TUI Manager** — split-pane layout, per-agent viewport components, focus-based input delegation

### Critical Pitfalls

1. **Subprocess pipe deadlocks** — Always start goroutines reading BOTH stdout and stderr BEFORE `cmd.Start()`. Never call `cmd.Wait()` until pipes are fully consumed. This is the most common Go subprocess bug and will hang the orchestrator silently.

2. **Fork drift from upstream Crush** — Keep Crush core files unmodified. Use interface injection and wrapper packages. Rebase weekly. Document every deviation. Submit upstream PRs for needed hooks. Recovery cost is months of work if this is neglected.

3. **Multi-agent file conflict cascades** — Agents silently overwrite each other's work. Use git worktrees for isolation AND implement file-level lock registry in the DAG scheduler. Pre-declare file targets in task metadata.

4. **Signal propagation failure** — `os.Process.Kill()` only kills the direct child, not its subprocess tree. Use `syscall.SysProcAttr{Setpgid: true}` and kill the entire process group. Without this, Ctrl+C leaves orphaned agents consuming API credits.

5. **Cascading failure without isolation** — One transient API timeout should not abort the entire workflow. Classify failures (hard/soft/skip), isolate dependency chains, implement retry with backoff for soft failures, checkpoint completed work for resume.

## Implications for Roadmap

Based on combined research, the build order is determined by three forces: (1) dependency chains from architecture research, (2) pitfall prevention timing from pitfalls research, and (3) feature groupings from feature research. The architecture research and pitfalls research strongly agree on ordering: subprocess management and fork strategy must come first.

### Phase 0: Fork Strategy and Extension Architecture
**Rationale:** Pitfalls research identifies fork drift as the highest-cost risk (months of recovery). Architecture research recommends wrapping Crush via interfaces, not modifying core files. This must be designed before writing any orchestrator code.
**Delivers:** Extension point architecture, documented fork maintenance process, verified clean rebase
**Addresses:** Fork drift prevention (Pitfall 3)
**Avoids:** Months of rework from diverging too far from upstream Crush

### Phase 1: Core Subprocess Management and Backend Abstraction
**Rationale:** Architecture research shows Backend interface is the foundation everything depends on. Pitfalls research demands subprocess pipe handling, process groups, and signal propagation be solved first (Pitfalls 1, 2, 9). Three of the ten critical pitfalls must be addressed here.
**Delivers:** Backend interface with Fantasy adapter (in-process), Subprocess adapter (Claude Code CLI), process lifecycle management with proper cleanup
**Addresses:** Backend abstraction (table stakes), subprocess communication
**Avoids:** Pipe deadlocks (Pitfall 1), zombie processes (Pitfall 2), orphaned agents (Pitfall 9)
**Uses:** os/exec, Fantasy, syscall.SysProcAttr, context.WithCancel

### Phase 2: Agent Definitions and DAG Scheduler
**Rationale:** Architecture research shows agent abstraction + scheduler enable the core orchestration loop. Feature research identifies agent roles and DAG scheduling as P1 table stakes. Pitfalls research requires cycle detection and file locking before parallel execution.
**Delivers:** YAML-based agent role config, SessionAgent with role metadata, DAG construction with topological sort validation, resource locking, failure classification and isolation
**Addresses:** Agent role definitions, task queue/DAG scheduling, error recovery foundations
**Avoids:** DAG deadlocks from cycles (Pitfall 6), file conflicts (Pitfall 4), cascading failures (Pitfall 7)
**Uses:** errgroup, go-dag or heimdalr/dag, YAML config

### Phase 3: Parallel Execution with Git Isolation
**Rationale:** Feature research identifies parallel execution as the core value proposition. Architecture research shows this requires backend adapters + scheduler + git worktrees working together. Cannot be built without Phase 1-2 foundations.
**Delivers:** Parallel agent execution (2-4 agents), git worktree management, bounded concurrency, task result collection
**Addresses:** Parallel agent execution, git integration for isolation
**Uses:** errgroup.SetLimit(), git worktree commands, work-stealing scheduler (if needed)

### Phase 4: Event Bus and TUI Integration
**Rationale:** Architecture research positions the Event Bus as the decoupling layer between agents and TUI. Feature research lists real-time TUI monitoring as P1. Pitfalls research warns about TUI state desync (Pitfall 8) — solving it requires proper event-driven state synchronization from the start.
**Delivers:** Channel-based event bus, split-pane TUI layout with per-agent viewports, real-time status updates, vim-style navigation, agent status tracking
**Addresses:** Real-time TUI monitoring, agent status tracking
**Avoids:** TUI state desync (Pitfall 8)
**Uses:** Bubble Tea v2, Bubbles viewport, Lipgloss v2 layout

### Phase 5: State Management and Session Persistence
**Rationale:** Feature research lists session persistence as P1 table stakes. Pitfalls research warns about context drift (Pitfall 5) and lost partial work (Pitfall 10). Architecture research defines a centralized State Store with conversation history and task checkpointing.
**Delivers:** Persistent conversation history, task checkpointing, durable state separate from LLM context, crash recovery with resume-from-checkpoint
**Addresses:** Session/context persistence, error recovery (full)
**Avoids:** Context window drift (Pitfall 5), lost partial work (Pitfall 10)
**Uses:** ncruces/go-sqlite3, goose migrations, sqlc

### Phase 6: Resilience and Production Hardening
**Rationale:** Architecture research lists resilience as the final layer (depends on all above). Feature research includes error recovery with exponential backoff as P1. This phase turns a working prototype into a reliable tool.
**Delivers:** Retry logic with exponential backoff and jitter, circuit breakers for failing backends, failure budgets, graceful degradation, partial success handling, token/cost budgets per agent
**Addresses:** Error recovery (production-grade), quality gates (basic)
**Avoids:** Cascading failures in production (Pitfall 7), runaway API costs

### Phase 7: Local LLM Backend and Network Adapter
**Rationale:** Feature research places local LLM support as P2 (after validation). Architecture research defines a Network backend adapter for OpenAI-compatible APIs. This extends the existing Backend interface with a third adapter type.
**Delivers:** Network backend adapter, OpenAI-compatible API integration, Ollama/LM Studio/llama.cpp support, built-in agentic tool loop for local models
**Addresses:** Local LLM support, multi-backend routing foundations
**Uses:** Fantasy openaicompat or direct openai-go client

### Phase Ordering Rationale

- **Phase 0 before everything:** Fork strategy must be established before writing code. Retrofitting an extension architecture is orders of magnitude harder than designing it up front. All four research files reference Crush compatibility.
- **Phase 1 before Phase 2:** Backend interface is a dependency for agent abstraction. Subprocess pitfalls (1, 2, 9) must be solved before any multi-agent logic exists.
- **Phase 2 before Phase 3:** DAG scheduler with cycle detection and resource locking must exist before parallel execution is enabled. Running agents in parallel without these guarantees causes silent data corruption.
- **Phase 4 after Phase 3:** TUI needs working agents to display. Event bus needs publishers (agents) to be useful. But TUI state sync must be designed correctly from the start (not bolted on).
- **Phase 5 before Phase 6:** Resilience patterns (retry, circuit breaker) need state checkpointing to be meaningful. Retrying without checkpoints wastes all previous work.
- **Phase 7 last in MVP:** Local LLM support is P2 — validates after cloud backends prove the architecture. The Backend interface from Phase 1 makes this a clean addition.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 0:** Needs Crush codebase analysis to identify specific extension points. Must map Crush's internal package boundaries and find interface injection opportunities.
- **Phase 1:** Needs investigation of Claude Code CLI's exact JSON-RPC protocol format and Codex CLI's `exec` command output format. Subprocess communication protocols are under-documented.
- **Phase 4:** Bubble Tea v2 split-pane layouts are not extensively documented. The john-marinelli/panes component and leg100 blog post are the main references. May need prototyping.
- **Phase 7:** Fantasy's openaicompat layer and its interaction with custom base URLs needs verification. OpenAI SDK v3 base URL support is unconfirmed.

Phases with standard patterns (skip deep research):
- **Phase 2:** DAG scheduling and topological sort are well-documented CS patterns. errgroup usage is idiomatic Go with extensive examples.
- **Phase 3:** Git worktree management is well-documented. Parallel goroutine execution with errgroup is standard.
- **Phase 5:** SQLite persistence, goose migrations, and sqlc code generation are all well-established patterns used by Crush itself.
- **Phase 6:** Retry with backoff, circuit breakers (sony/gobreaker), and failure classification are well-documented resilience patterns.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Charm ecosystem is proven, Crush validates the dependency choices, official documentation is extensive |
| Features | MEDIUM | Feature landscape well-researched across 2026 frameworks (CrewAI, LangGraph, Clark, Ralph TUI), but MVP scope assumptions need user validation |
| Architecture | HIGH | Hub-and-spoke pattern well-documented by Microsoft, LangChain, multiple sources. Adapter pattern, event bus, DAG scheduling are standard Go patterns |
| Pitfalls | MEDIUM-HIGH | Subprocess pitfalls verified against Go stdlib documentation and production war stories. Fork drift risk is real but mitigation strategies are well-known |

**Overall confidence:** HIGH

### Gaps to Address

- **Fantasy v0.7.1 + openai-go v3 compatibility:** Crush pins openai-go v2.7.1. If we want v3 features, Fantasy compatibility must be verified. Resolve during Phase 1 by testing Fantasy against v3.
- **Claude Code CLI JSON-RPC protocol specifics:** The exact message format, streaming behavior, and error codes for non-interactive Claude Code are under-documented. Resolve during Phase 1 by testing against actual CLI.
- **Codex CLI programmatic interface:** Codex `exec` command output format and `resume` semantics need hands-on testing. Resolve during Phase 1.
- **DAG library choice (go-dag vs heimdalr/dag):** No clear winner in research. Resolve during Phase 2 by prototyping both with realistic task graphs. heimdalr/dag is faster and thread-safe with caching; go-dag is simpler.
- **Bubble Tea v2 stability:** All Charm v2 packages are RC/beta. Monitor for breaking changes. Low risk given RC status.
- **Crush fork point:** Which Crush commit to fork from needs analysis. Latest main may include WIP features. Resolve during Phase 0.

## Sources

### Primary (HIGH confidence)
- Crush GitHub repository and go.mod — actual production dependency choices
- Bubble Tea GitHub / pkg.go.dev — TUI framework architecture and v2 API
- Fantasy GitHub — multi-provider LLM abstraction, openaicompat layer
- errgroup pkg.go.dev — goroutine orchestration API
- Go os/exec package documentation — subprocess management patterns
- MCP Go SDK — official SDK maintained with Google
- Microsoft Azure Architecture Center — AI agent orchestration patterns
- ncruces/go-sqlite3 — CGo-free SQLite features and API

### Secondary (MEDIUM confidence)
- CrewAI, LangGraph, AutoGen framework comparisons — feature landscape
- Ralph TUI, Clark, TmuxCC — competitor TUI approaches
- Blog posts on Bubble Tea multi-pane layouts (leg100, shi.foo)
- Fork maintenance best practices (preset.io, ropensci.org)
- Go subprocess deadlock patterns (DoltHub, HackMySQL)
- 2026 Agentic Coding Trends Report (Anthropic)

### Tertiary (LOW confidence)
- openai-go v3 custom base URL support — needs source code verification
- Fantasy v0.7.1 compatibility with openai-go v3 — needs testing
- Performance comparison ncruces vs modernc.org/sqlite — no benchmarks found
- Exact Claude Code CLI and Codex CLI subprocess protocol details — needs hands-on testing

---
*Research completed: 2026-02-10*
*Ready for roadmap: yes*
