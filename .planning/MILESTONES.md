# Milestones

## v1.0 MVP (Shipped: 2026-02-10)

**Phases completed:** 6 phases, 21 plans, 42 tasks
**Lines of code:** 11,828 Go
**Timeline:** 2026-02-10 (single day)

**Key accomplishments:**
- Unified Backend interface with subprocess adapters for Claude Code, Codex, and Goose CLIs
- DAG scheduler with topological sort, cycle detection, resource locking, and workflow follow-ups
- Parallel task execution in isolated git worktrees with bounded concurrency and merge-back
- Split-pane Bubble Tea TUI with real-time agent viewports, DAG progress, and vim navigation
- SQLite persistence for task state, session IDs, and conversation history with crash recovery
- Resilience layer with exponential backoff retry, per-backend circuit breakers, and graceful shutdown

---

