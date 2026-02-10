---
phase: 02-agent-definitions-and-dag-scheduler
plan: 04
subsystem: scheduler

tags: [workflow, dag, task-spawning, dependency-management]

# Dependency graph
requires:
  - phase: 02-01
    provides: "Config types with WorkflowConfig definitions"
  - phase: 02-03
    provides: "DAG scheduler with task types and validation"
provides:
  - "WorkflowManager that spawns follow-up tasks based on workflow configuration"
  - "Automatic creation of reviewer and tester tasks after coder task completion"
  - "DAG re-validation to catch cycles introduced by workflow tasks"
  - "Support for multiple workflows containing the same agent role"
affects: [02-05, executor, parallel-execution]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Workflow-driven task spawning with automatic follow-up creation"
    - "DAG re-validation after dynamic task addition"

key-files:
  created:
    - "internal/scheduler/workflow.go"
    - "internal/scheduler/workflow_test.go"
  modified: []

key-decisions:
  - "Follow-up task ID format: {originalID}-{agentRole} for clear lineage"
  - "Review tasks use FailSoft (code can proceed), test tasks use FailHard (blocks on failure)"
  - "Simple prompt template for follow-ups: 'Review the output of task X: Y' (Phase 3+ will refine)"
  - "Multiple workflows can share same agent roles - all workflows spawn follow-ups"

patterns-established:
  - "OnTaskCompleted hook called after task completion to trigger follow-ups"
  - "DAG validation happens after each task addition to catch cycles early"
  - "Follow-up tasks inherit WritesFiles from parent task (reviewer/tester may write same files)"

# Metrics
duration: 146s
completed: 2026-02-10
---

# Phase 2 Plan 4: Workflow Engine Summary

**WorkflowManager spawns follow-up tasks (code -> review -> test) automatically, adds them to DAG with proper dependencies, and validates for cycles**

## Performance

- **Duration:** 2min 26s
- **Started:** 2026-02-10T17:00:51Z
- **Completed:** 2026-02-10T17:03:17Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- WorkflowManager creates follow-up tasks when completed task's agent role matches workflow step
- Follow-up tasks added to same DAG (not separate queue) with proper dependency chains
- DAG re-validation catches cycles introduced by workflow-added tasks
- Support for multiple workflows containing same agent role (e.g., standard + security workflows both using "coder")
- Last-step and non-workflow tasks produce no follow-ups (graceful handling)

## Task Commits

Each task was committed atomically:

1. **Task 1: WorkflowManager for follow-up task spawning** - `d0d5d4a` (feat)
2. **Task 2: WorkflowManager tests** - `b1da9ab` (test)

## Files Created/Modified
- `internal/scheduler/workflow.go` - WorkflowManager with OnTaskCompleted hook for spawning follow-up tasks
- `internal/scheduler/workflow_test.go` - 8 comprehensive tests covering standard workflow, edge cases, and multiple workflows

## Decisions Made

1. **Follow-up task ID format**: `{originalID}-{agentRole}` (e.g., "task-1-reviewer") provides clear lineage and prevents accidental collisions
2. **Failure modes**: Review tasks use FailSoft (code can proceed even if review fails), test tasks use FailHard (test failure blocks dependents)
3. **Simple prompt template**: "Review the output of task {ID}: {result}" for now - Phase 3+ will introduce more sophisticated prompt templates
4. **Multiple workflow support**: Same agent role can appear in multiple workflows (e.g., "coder" in both "standard" and "security" workflows) - all matching workflows spawn follow-ups

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

**1. Module import path correction**
- **Issue:** Initial import used `github.com/aristath-ai/orchestrator/internal/config` (incorrect org name)
- **Resolution:** Corrected to `github.com/aristath/orchestrator/internal/config` by checking go.mod
- **Impact:** Build compilation error resolved immediately

**2. Test compilation unused variables**
- **Issue:** Test code had unused `task2` and `dag` variables from commented-out test logic
- **Resolution:** Removed unused variable declarations
- **Impact:** Test compilation successful, all 8 tests pass

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

WorkflowManager complete and tested. Ready for:
- Phase 2 Plan 5: Resource locking or agent integration
- Executor integration to call OnTaskCompleted hook after task execution
- Parallel execution that can spawn and schedule follow-up tasks dynamically

All tests pass under -race flag. DAG validation ensures workflow tasks don't introduce cycles.

## Self-Check: PASSED

Files verified:
- FOUND: internal/scheduler/workflow.go
- FOUND: internal/scheduler/workflow_test.go

Commits verified (d0d5d4a, b1da9ab) exist in git history.

---
*Phase: 02-agent-definitions-and-dag-scheduler*
*Completed: 2026-02-10*
