# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-10)

**Core value:** A single developer plans a task with an AI, then that AI autonomously decomposes and executes the plan across multiple specialized agents running in parallel -- coordinating them, answering their questions, and ensuring quality.
**Current focus:** Phase 5 in progress — persistence layer

## Current Position

Phase: 5 of 6 (State Management and Session Persistence)
Plan: 3 of 3 in current phase
Status: Phase 5 complete
Last activity: 2026-02-10 -- Phase 5 Plan 3 complete

Progress: [█████████████] 80%

## Performance Metrics

**Velocity:**
- Total plans completed: 17
- Average duration: 251 seconds
- Total execution time: 1.18 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 5 | 910s | 182s |
| 02 | 3 | 405s | 135s |
| 03 | 3 | 1706s | 569s |
| 04 | 3 | 659s | 220s |
| 05 | 3 | 739s | 246s |

**Recent Trend:**
- Last 5 plans: 205s, 351s, 159s, 229s (avg: 236s)
- Trend: Phase 05 complete with consistent ~230s average for P02-P03

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
| Phase 03 P02 | 144s | 2 tasks | 2 files |
| Phase 03 P01 | 228s | 2 tasks | 3 files |
| Phase 03 P03 | 1334s | 2 tasks | 3 files |
| Phase 04 P01 | 224 | 2 tasks | 5 files |
| Phase 04 P02 | 230 | 2 tasks | 6 files |
| Phase 04 P03 | 205 | 2 tasks | 7 files |
| Phase 05 P01 | 351s | 2 tasks | 4 files |
| Phase 05 P02 | 159s | 2 tasks | 4 files |
| Phase 05 P03 | 229 | 2 tasks | 2 files |

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
- [03-01]: Use git merge-tree --write-tree for dry-run conflict detection before merge
- [03-01]: Always checkout base branch before merge to ensure correct merge target
- [03-01]: Map MergeStrategy to git CLI strategy names (recursive/ours/theirs)
- [03-01]: Worktree naming pattern: .worktrees/{taskID} with branch task/{taskID}
- [03-01]: Best-effort cleanup with force retry on failure for robust cleanup paths
- [03-02]: Buffer size configurable by caller (recommended 2x concurrency)
- [03-02]: Per-question response channels prevent cross-talk without mutex
- [03-02]: Serial question processing by single handler goroutine
- [03-02]: Double select in Ask prevents goroutine leak on cancellation
- [03-03]: Serialize merge operations with mutex to prevent git lock conflicts
- [03-03]: BackendFactory pattern enables mock injection for testing
- [03-03]: Task success independent of merge success (isolation principle)
- [03-03]: Task errors tracked in DAG, not errgroup return value
- [03-03]: Wave-based execution loop naturally handles DAG dependencies
- [Phase 04]: Non-blocking publish with select/default prevents slow subscribers from blocking execution
- [Phase 04]: SubscribeAll via dedicated allSubs slice enables single-channel multi-topic consumption
- [04-02]: Use stable bubbletea v1.x instead of v2 beta for production reliability
- [04-02]: Debounce viewport updates with 50ms tick to prevent render thrashing from high-frequency events
- [04-02]: Auto-scroll viewport to bottom on new output for better real-time UX
- [04-02]: Split-pane layout with agent list+viewport (35%) and DAG progress (30% bottom-right)
- [04-03]: Settings panel is modal overlay blocking normal TUI when open
- [04-03]: Save creates parent directories automatically with os.MkdirAll
- [04-03]: Form values bound to local strings, copied to config on completion
- [04-03]: Settings panel hides itself after successful save
- [05-01]: Use modernc.org/sqlite for pure-Go SQLite (no CGO dependency)
- [05-01]: WAL mode with busy_timeout=5000ms and synchronous=NORMAL for performance
- [05-01]: MaxOpenConns=2 prevents ListTasks deadlock while maintaining write safety
- [05-01]: Shared cache for in-memory test databases (cache=shared parameter)
- [05-01]: Explicit foreign key checks in SaveTask for clear error messages
- [05-01]: Store WritesFiles as comma-separated string for simplicity
- [05-02]: Move session methods from tasks.go to sessions.go for proper code organization
- [05-02]: Use PRAGMA foreign_keys = ON instead of connection string parameter (modernc.org/sqlite requirement)
- [05-02]: Return empty slice instead of nil for GetHistory when no messages exist
- [05-02]: Add id-based tiebreaker to ORDER BY clause to handle same-second timestamp insertions
- [05-03]: Store is optional in ParallelRunnerConfig (nil disables persistence)
- [05-03]: Checkpoint errors logged but don't halt execution (data loss better than total failure)
- [05-03]: Persist full DAG at Run start for reliable resume capability
- [05-03]: Sessions loaded in Resume but not yet used in createBackend (future multi-turn support)

### Pending Todos

None yet.

### Blockers/Concerns

**Phase 5 Complete - No Blockers**

Phase 1-5 complete. Phase 6 next.
Phase 5 accomplishments (3 of 3 plans complete):
- Plan 01: Persistence layer foundation with Store interface, SQLite schema, task DAG persistence
  - All 7 tests passing with -race flag
  - Foreign key enforcement working correctly
  - Shared cache solution for in-memory test databases
  - Must-have truths all verified
- Plan 02: Session and conversation history persistence
  - Session ID and backend type upsert working correctly
  - Conversation history preserves chronological order
  - Fixed foreign key enforcement via PRAGMA for modernc.org/sqlite
  - All 15 tests passing with -race flag (7 from P01 + 8 from P02)
- Plan 03: ParallelRunner Store integration and Resume method
  - Checkpoint on every task state transition (Running, Completed, Failed)
  - Save session IDs, backend types, and conversation messages
  - Resume reconstructs DAG from Store and skips completed tasks
  - All 20 tests passing with -race flag (15 from P01-P02 + 5 new)

**Next: Phase 6** - Resilience and Error Recovery

## Session Continuity

Last session: 2026-02-10
Stopped at: Completed Phase 05 Plan 03 - ParallelRunner Store integration and Resume method
Resume file: None
