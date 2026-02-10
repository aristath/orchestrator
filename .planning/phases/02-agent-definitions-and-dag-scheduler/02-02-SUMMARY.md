---
phase: 02-agent-definitions-and-dag-scheduler
plan: 02
subsystem: scheduler
tags: [dag, dependency-resolution, cycle-detection, task-management]
dependency_graph:
  requires: []
  provides:
    - "internal/scheduler/task.go - Task struct with status, failure modes, dependencies"
    - "internal/scheduler/dag.go - DAG with topological sort and cycle detection"
    - "internal/scheduler/dag_test.go - Comprehensive table-driven tests"
  affects: []
tech_stack:
  added:
    - "github.com/gammazero/toposort@v0.1.1 - Kahn's algorithm for topological sort with cycle detection"
  patterns:
    - "Table-driven tests with subtests for comprehensive DAG validation"
    - "FailureMode enum for controlling downstream task eligibility"
    - "Topological sort with disconnected component validation"
key_files:
  created:
    - internal/scheduler/task.go
    - internal/scheduler/dag.go
    - internal/scheduler/dag_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - "Use gammazero/toposort for cycle detection via Kahn's algorithm"
  - "FailureMode controls dependency resolution: FailHard blocks, FailSoft allows, FailSkip treats as success"
  - "Validate all dependencies exist before topological sort"
  - "Track disconnected components by verifying sorted result contains all tasks"
metrics:
  duration: 162
  tasks: 2
  files: 3
  completed: "2026-02-10T16:52:44Z"
---

# Phase 02 Plan 02: DAG Scheduler Core Summary

**One-liner:** Task dependency graph with topological sort using Kahn's algorithm, cycle detection, and failure-mode-aware dependency resolution

## What Was Built

Implemented the core DAG scheduler data structures and algorithms. The DAG stores tasks as nodes with explicit dependencies as edges. Topological sort using gammazero/toposort validates the graph, rejects cycles with clear error messages, and produces a valid execution order. Dependency resolution respects three failure modes: FailHard blocks all dependents, FailSoft allows dependents to run anyway, and FailSkip treats failures as successes for dependency purposes.

**Task types:**
- TaskStatus enum: Pending → Eligible → Running → Completed/Failed/Skipped
- FailureMode enum: FailHard, FailSoft, FailSkip
- Task struct with ID, dependencies, status, failure mode, file writes, result, and error

**DAG implementation:**
- NewDAG() creates empty graph
- AddTask() adds tasks, builds reverse dependency map
- Validate() runs topological sort, detects cycles, verifies dependencies exist
- Eligible() returns tasks ready to run (all dependencies resolved)
- MarkRunning/Completed/Failed() manage task lifecycle
- Get(), Tasks(), Order() provide query methods

**Cycle detection:**
- Uses Kahn's algorithm via gammazero/toposort
- Detects direct cycles (A→B→A)
- Detects transitive cycles (A→B→C→A)
- Detects self-loops (A→A)
- Error messages identify cycle participants

**Disconnected components:**
- Tasks with no dependencies added as edges from nil
- Validates sorted result contains all task IDs
- Catches disconnected graph components that would otherwise be lost

## Tests

Created 23+ test cases across 4 test functions covering:

**Validation (9 cases):**
1. Valid linear chain (A→B→C)
2. Valid parallel tasks (A,B→C)
3. Single task with no deps
4. Direct cycle detection
5. Transitive cycle detection
6. Self-loop detection
7. Missing dependency detection
8. Duplicate task ID rejection
9. Disconnected components handling

**Eligibility (7 cases):**
1. Initial eligible tasks (no dependencies)
2. Completion unlocks dependents
3. Partial completion keeps waiting
4. Hard failure blocks dependents
5. Soft failure allows dependents
6. Skipped tasks treated as success
7. Failed with FailSkip treated as success

**State transitions (7 cases):**
1. MarkRunning succeeds
2. MarkCompleted stores result
3. MarkFailed stores error
4. MarkRunning on nonexistent returns error
5. Get returns task and exists flag
6. Tasks returns all tasks
7. Order returns same as Validate

**Complex scenarios (2 cases):**
1. Diamond dependency pattern (A→B,C→D)
2. Mixed failure modes (FailHard blocks, FailSoft/FailSkip allow)

All tests pass with `-race` flag and `go vet` reports no issues.

## Deviations from Plan

None - plan executed exactly as written.

## Key Decisions

1. **Use gammazero/toposort for topological sort**: Kahn's algorithm provides natural cycle detection via in-degree tracking. DFS-based approaches require careful coloring (white/gray/black) and are more error-prone. The library handles edge cases like isolated nodes and provides clear cycle error messages.

2. **FailureMode controls dependency resolution**:
   - FailHard: Block ALL dependents (e.g., build failed, can't test)
   - FailSoft: Dependents CAN run (e.g., linting failed, but tests should still run)
   - FailSkip: Treat as success (e.g., conditional task skipped)

   This matches Airflow's failure handling patterns and provides flexible workflow control.

3. **Validate all dependencies before sort**: Catches missing task references early with clear error messages. Without this check, topological sort would silently treat missing tasks as orphaned nodes.

4. **Track disconnected components**: Tasks with no incoming or outgoing edges could be lost during sort. Solution: Add edges from nil for root tasks, then verify sorted result contains all task IDs. This catches disconnected graph components.

## What This Enables

- **Phase 02-03**: Agent role definitions can reference task failure modes in workflow configs
- **Phase 02-04**: Executor can query Eligible() to find tasks ready for parallel execution
- **Phase 02-05**: Resource lock manager can use Task.WritesFiles for file-level locking
- **Phase 03**: Parallel task execution respects DAG ordering and failure propagation

The DAG is now the central scheduling data structure. Tasks flow through the lifecycle: Pending → Eligible → Running → Completed/Failed. Dependency resolution happens automatically - completing tasks unlocks their dependents, failures propagate according to FailureMode.

## File Changes

**Created:**
- `internal/scheduler/task.go` (33 lines) - Task struct, TaskStatus enum, FailureMode enum
- `internal/scheduler/dag.go` (213 lines) - DAG implementation with validation and dependency resolution
- `internal/scheduler/dag_test.go` (485 lines) - Comprehensive table-driven tests

**Modified:**
- `go.mod` - Added gammazero/toposort@v0.1.1
- `go.sum` - Dependency checksums

## Verification Results

✓ `go build ./internal/scheduler/` compiles successfully
✓ `go test ./internal/scheduler/ -v -count=1 -race` all 23 tests pass
✓ `go vet ./internal/scheduler/` no issues
✓ Cycle detection identifies cycle participants in error messages
✓ Disconnected DAG components included in sorted output

## Self-Check: PASSED

**Created files verified:**
```
FOUND: internal/scheduler/task.go
FOUND: internal/scheduler/dag.go
FOUND: internal/scheduler/dag_test.go
```

**Commits verified:**
```
FOUND: 5d0f8f6 (feat(02-02): implement DAG with task types and cycle detection)
FOUND: 4b5b3e4 (test(02-02): add comprehensive DAG table-driven tests)
```

All claimed files exist, all commits are in git history, all tests pass.
