# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-10)

**Core value:** A single developer plans a task with an AI, then that AI autonomously decomposes and executes the plan across multiple specialized agents running in parallel -- coordinating them, answering their questions, and ensuring quality.
**Current focus:** Phase 2 - Agent Definitions and DAG Scheduler

## Current Position

Phase: 2 of 6 (Agent Definitions and DAG Scheduler)
Plan: 4 of 5 in current phase
Status: In progress
Last activity: 2026-02-10 -- Completed 02-04-PLAN.md (Workflow engine for follow-up task spawning)

Progress: [████████░░] 80%

## Performance Metrics

**Velocity:**
- Total plans completed: 8
- Average duration: 156 seconds
- Total execution time: 0.35 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 5 | 910s | 182s |
| 02 | 3 | 405s | 135s |

**Recent Trend:**
- Last 5 plans: 238s, 334s, 97s, 162s, 146s (avg: 195s)
- Trend: Phase 2 maintaining efficient pace

*Updated after each plan completion*

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 01 P01 | 96s | 2 tasks | 4 files |
| Phase 01 P02 | 142s | 2 tasks | 4 files |
| Phase 01 P04 | 100s | 2 tasks | 2 files |
| Phase 01 P03 | 238s | 2 tasks | 2 files |
| Phase 01 P05 | 334s | 2 tasks | 3 files |
| Phase 02 P01 | 97s | 2 tasks | 4 files |
| Phase 02 P02 | 162s | 2 tasks | 3 files |
| Phase 02 P04 | 146s | 2 tasks | 2 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Architecture]: Build standalone orchestrator instead of forking Crush -- avoids fork maintenance, agent CLIs provide tools/multi-turn/output
- [Architecture]: Use agent CLIs (Claude Code, Codex, Goose) as subprocess backends -- all backends are subprocess-based
- [Architecture]: Goose for local LLMs -- supports Ollama/LM Studio/llama.cpp with full CLI capabilities
- [Roadmap]: 6 phases (1-6) covering subprocess management, agent definitions, parallel execution, TUI, persistence, resilience
- [01-01]: Use Setpgid: true for all subprocesses -- enables clean termination of entire process trees
- [01-01]: Read pipes concurrently before cmd.Wait() -- prevents deadlocks with large output
- [01-01]: ProcessManager tracks all subprocesses centrally -- enables graceful shutdown
- [01-03]: Use "codex exec" for first message, "codex resume <THREAD_ID>" for subsequent -- matches Codex CLI semantics
- [01-03]: Parse newline-delimited JSON with bufio.Scanner -- clean event stream parsing for ThreadStarted and TurnCompleted
- [01-03]: Store thread ID from first ThreadStarted event -- thread ID comes from Codex response, not pre-generated
- [01-04]: Goose session names use "orchestrator-{random-hex}" format -- human-readable and unique
- [01-04]: Pass --provider and --model directly to Goose CLI -- simple local LLM support via passthrough
- [01-04]: Flexible JSON parsing with ndjson and plain text fallbacks -- robust handling of varied Goose output
- [Phase 01-02]: Generate UUIDs without external dependencies using crypto/rand for self-contained session management
- [Phase 01-02]: Track subprocesses via optional ProcessManager in executeCommand for graceful shutdown
- [01-05]: Use mock CLI script for subprocess testing -- enables tests without actual CLI installations
- [01-05]: Test with 256KB output for deadlock prevention -- proves concurrent pipe reading works under stress
- [01-05]: Use 15 sequential invocations for zombie test -- exceeds 10+ requirement, validates cleanup at scale
- [02-01]: Map-level merge for config enables independent provider/agent/workflow overrides
- [02-01]: Project config highest precedence (defaults -> global -> project) matches user expectations
- [02-01]: Missing config files not errors enables zero-config usage with graceful degradation
- [02-01]: Zero external config libraries using stdlib encoding/json for lean binary
- [02-02]: Use gammazero/toposort for cycle detection via Kahn's algorithm
- [02-02]: FailureMode controls dependency resolution: FailHard blocks, FailSoft allows, FailSkip treats as success
- [02-02]: Validate all dependencies exist before topological sort
- [02-02]: Track disconnected components by verifying sorted result contains all tasks
- [02-04]: Follow-up task ID format: {originalID}-{agentRole} for clear lineage
- [02-04]: Review tasks use FailSoft (code can proceed), test tasks use FailHard (blocks on failure)
- [02-04]: Simple prompt template for follow-ups: 'Review the output of task X: Y' (Phase 3+ will refine)
- [02-04]: Multiple workflows can share same agent roles - all workflows spawn follow-ups

### Pending Todos

None yet.

### Blockers/Concerns

**Phase 2 In Progress - No Blockers**

Phase 1 complete. Phase 2 Plans 01-02-04 complete:
- Config type system implemented with three-tier merge (defaults -> global -> project)
- DAG scheduler with task types, topological sort, and cycle detection
- Dependency resolution respects FailHard/FailSoft/FailSkip failure modes
- WorkflowManager spawns follow-up tasks (code -> review -> test) with DAG validation
- All tests passing (8 config tests, 23 DAG tests, 8 workflow tests with -race flag)

## Session Continuity

Last session: 2026-02-10
Stopped at: Completed 02-04-PLAN.md (Workflow engine for follow-up task spawning)
Resume file: None
