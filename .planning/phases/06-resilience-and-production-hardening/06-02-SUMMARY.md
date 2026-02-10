---
phase: 06-resilience-and-production-hardening
plan: 02
subsystem: orchestrator-lifecycle
tags: [signal-handling, graceful-shutdown, subprocess-management, resilience]
dependency_graph:
  requires:
    - "01-01 (ProcessManager with KillAll support)"
    - "04-01 (EventBus with Close method)"
  provides:
    - "Signal-aware main entry point with graceful shutdown"
    - "Subprocess cleanup on Ctrl+C"
    - "10-second shutdown timeout"
  affects:
    - "Future DAG runner integration point"
tech_stack:
  added: [os/signal, signal.NotifyContext]
  patterns: [signal-aware-context, graceful-shutdown, double-ctrl-c-force-exit]
key_files:
  created:
    - cmd/orchestrator/main_test.go
  modified:
    - cmd/orchestrator/main.go
decisions:
  - "Use signal.NotifyContext for clean signal handling"
  - "Call stop() after ctx.Done() to enable double Ctrl+C force exit"
  - "Run TUI in goroutine to allow main thread to handle signals"
  - "10-second shutdown timeout prevents hanging"
metrics:
  duration: 70s
  tasks: 2
  files: 2
  tests_added: 3
  completed: 2026-02-10
---

# Phase 06 Plan 02: Graceful Shutdown with Signal Handling Summary

**One-liner:** Signal-aware orchestrator entry point with SIGINT/SIGTERM handling, ProcessManager integration for subprocess cleanup, 10-second shutdown timeout, and double Ctrl+C force exit support.

## What Was Built

Replaced the demo-focused main.go with a production-ready entry point that handles graceful shutdown:

1. **Signal handling** via `signal.NotifyContext` - Ctrl+C and SIGTERM trigger context cancellation
2. **ProcessManager integration** - All subprocesses tracked and killed on shutdown
3. **TUI lifecycle management** - TUI runs in goroutine, main thread handles signals
4. **Shutdown timeout** - 10-second deadline prevents hanging if cleanup stalls
5. **Double Ctrl+C force exit** - Calling `stop()` after `ctx.Done()` restores default signal handling
6. **Clean entry point** - Removed fake demo events, added TODO for ParallelRunner integration

## Architecture

**Shutdown flow:**
1. User presses Ctrl+C → `ctx.Done()` fires
2. Main calls `stop()` to restore default signal behavior (second Ctrl+C kills immediately)
3. ProcessManager.KillAll() sends SIGKILL to all subprocess process groups
4. TUI.Quit() called to terminate Bubble Tea program
5. Wait for TUI exit with 10-second timeout
6. Clean exit

**Key pattern:** Signal-aware context created at start, passed through system, enables coordinated shutdown.

## Testing

Created 3 integration tests in `cmd/orchestrator/main_test.go`:

1. **TestProcessManagerKillAllOnShutdown** - Verifies KillAll() terminates tracked sleep process
2. **TestSignalContextCancellation** - Verifies signal.NotifyContext cancels on SIGUSR1
3. **TestShutdownTimeout** - Verifies context timeout fires correctly

All tests pass with `-race` flag.

## Deviations from Plan

None - plan executed exactly as written.

## Integration Points

**Upstream dependencies:**
- ProcessManager.KillAll() from Phase 01-01
- EventBus.Close() from Phase 04-01

**Downstream impact:**
- ParallelRunner will be wired in at the TODO comment
- Runner will inherit signal-aware context for coordinated shutdown
- Runner already supports context cancellation for checkpoint behavior

## Files Changed

**Created:**
- `cmd/orchestrator/main_test.go` (127 lines) - Shutdown integration tests

**Modified:**
- `cmd/orchestrator/main.go` - Replaced 115-line demo with 107-line production entry point

## Verification Results

- `go test -race -v ./cmd/orchestrator/... -count=1` - All 3 tests pass
- `go build ./cmd/orchestrator/...` - Compiles without errors
- `go vet ./cmd/orchestrator/...` - No warnings
- Code inspection confirms `signal.NotifyContext`, `pm.KillAll()`, and `stop()` present

## Success Criteria Met

- [x] Ctrl+C triggers graceful shutdown: context cancels, subprocesses killed, clean exit (RESIL-04)
- [x] 10-second timeout prevents hanging on stalled cleanup
- [x] Double Ctrl+C forces immediate exit
- [x] ProcessManager integrated in main for subprocess lifecycle
- [x] Shutdown tests pass with -race flag

## Must-Haves Verified

**Truths:**
- [x] Pressing Ctrl+C triggers graceful shutdown: context cancels, subprocess trees are killed, orchestrator exits cleanly
- [x] A 10-second shutdown timeout prevents the orchestrator from hanging if cleanup stalls
- [x] Double Ctrl+C forces immediate exit via standard Go signal behavior
- [x] Database connections and event bus are closed during shutdown (via defer bus.Close())

**Artifacts:**
- [x] `cmd/orchestrator/main.go` provides signal handling with graceful shutdown, ProcessManager integration
- [x] Contains `signal.NotifyContext`

**Key Links:**
- [x] `cmd/orchestrator/main.go` → `ProcessManager.KillAll` via shutdown handler calls KillAll on signal
- [x] `cmd/orchestrator/main.go` → `signal.NotifyContext` via context cancellation on SIGINT/SIGTERM

## Self-Check: PASSED

**Created files exist:**
- FOUND: cmd/orchestrator/main_test.go

**Modified files exist:**
- FOUND: cmd/orchestrator/main.go

**Commits exist:**
- FOUND: 1ab5098 (Task 1: graceful shutdown implementation)
- FOUND: 1fe367c (Task 2: integration tests)

**Pattern verification:**
- signal.NotifyContext: FOUND at line 23
- pm.KillAll: FOUND at line 81
- stop() calls: FOUND at lines 24, 76

All claims verified.
