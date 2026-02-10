---
phase: 04-event-bus-tui-integration
plan: 01
subsystem: orchestration
tags: [event-bus, pubsub, channels, concurrency, instrumentation]

# Dependency graph
requires:
  - phase: 03-parallel-execution-with-git-isolation
    provides: ParallelRunner with bounded concurrency and worktree isolation
provides:
  - Channel-based event bus with topic subscriptions and SubscribeAll
  - ParallelRunner instrumented with event publishing for task lifecycle
  - Foundation for TUI rendering without blocking execution
affects: [04-02-tui-display, 04-03-interactive-controls]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Channel-based pub-sub with non-blocking publish (select/default)
    - SubscribeAll pattern for cross-topic event consumption
    - Optional event bus injection (nil disables)

key-files:
  created:
    - internal/events/bus.go
    - internal/events/types.go
    - internal/events/bus_test.go
  modified:
    - internal/orchestrator/runner.go
    - internal/orchestrator/runner_test.go

key-decisions:
  - "Use channel-based pub-sub with non-blocking publish for event delivery"
  - "SubscribeAll returns single channel for cross-topic consumption"
  - "EventBus optional in ParallelRunnerConfig (nil disables)"
  - "Publish DAGProgress after each execution wave"

patterns-established:
  - "Event interface with EventType() and TaskID() methods"
  - "Non-blocking publish with select/default to prevent slow consumers blocking execution"
  - "SubscribeAll via dedicated allSubs slice for multi-topic fan-out"

# Metrics
duration: 224s
completed: 2026-02-10
---

# Phase 04 Plan 01: Event Bus and ParallelRunner Instrumentation Summary

**Channel-based event bus with non-blocking publish and SubscribeAll for cross-topic consumption; ParallelRunner instrumented to publish task lifecycle and DAG progress events**

## Performance

- **Duration:** 3 min 44 sec
- **Started:** 2026-02-10T18:34:46Z
- **Completed:** 2026-02-10T18:38:30Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Channel-based EventBus with topic subscriptions and SubscribeAll for cross-topic events
- Six event types covering task lifecycle (Started, Output, Completed, Failed, Merged) and DAG progress
- ParallelRunner publishes events at all key execution points without blocking tasks
- All tests pass with -race flag (7 event bus tests, 8 orchestrator tests including new integration test)

## Task Commits

Each task was committed atomically:

1. **Task 1: Event bus and event types** - `6b1116b` (feat)
2. **Task 2: Instrument ParallelRunner with event publishing** - `8a19ecc` (feat)

## Files Created/Modified
- `internal/events/bus.go` - EventBus with Subscribe, SubscribeAll, Publish, Close
- `internal/events/types.go` - Event interface and 6 concrete event types
- `internal/events/bus_test.go` - 7 tests covering publish/subscribe, non-blocking, close, topic isolation, SubscribeAll
- `internal/orchestrator/runner.go` - Added EventBus field, publish() helper, publishProgress() method, event publishing at task lifecycle points
- `internal/orchestrator/runner_test.go` - Added TestEventBusIntegration to verify event delivery

## Decisions Made

**1. Non-blocking publish with select/default**
Prevents slow or full subscriber channels from blocking task execution. Events are dropped if channel is full rather than blocking publisher.

**2. SubscribeAll via dedicated allSubs slice**
Simple implementation: SubscribeAll channels receive events from ALL topics. Publisher sends to both topic-specific and allSubs channels. Enables single-channel consumption for TUI.

**3. EventBus optional in ParallelRunnerConfig**
Setting EventBus=nil disables event publishing entirely. All existing tests pass unchanged with nil EventBus, proving backward compatibility.

**4. Publish DAGProgress after each wave**
After errgroup.Wait() completes each execution wave, runner computes status counts and publishes DAGProgressEvent. Gives TUI granular progress updates.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Event bus foundation complete, ready for TUI integration
- SubscribeAll enables TUI to consume all events on single channel
- Non-blocking publish ensures TUI rendering cannot block task execution
- Ready for Phase 04 Plan 02 (TUI display with Bubble Tea)

## Self-Check: PASSED

All created files exist:
- internal/events/bus.go
- internal/events/types.go
- internal/events/bus_test.go

All commits exist:
- 6b1116b (Task 1)
- 8a19ecc (Task 2)

All tests pass with -race flag.

---
*Phase: 04-event-bus-tui-integration*
*Completed: 2026-02-10*
