---
phase: 01-subprocess-management-and-backend-abstraction
plan: 01
subsystem: backend-foundation
tags: [backend-interface, subprocess-management, process-groups, concurrent-pipes]
dependency_graph:
  requires: []
  provides:
    - backend.Backend interface
    - backend.Message and backend.Response types
    - backend.Config type
    - executeCommand with concurrent pipe reading
    - newCommand with process group isolation
    - ProcessManager for subprocess lifecycle
  affects:
    - All future adapter implementations (01-02, 01-03, 01-04)
tech_stack:
  added:
    - Go 1.x module system
    - syscall.SysProcAttr for process groups
    - sync.WaitGroup for concurrent pipe reading
  patterns:
    - Factory pattern (New() function)
    - Concurrent pipe reading to prevent deadlocks
    - Process group isolation for signal propagation
key_files:
  created:
    - go.mod
    - internal/backend/types.go
    - internal/backend/backend.go
    - internal/backend/process.go
  modified: []
decisions:
  - decision: "Use Setpgid: true for all subprocesses"
    rationale: "Enables clean termination of entire process trees via killProcessGroup"
    impact: "All child processes can be terminated together, preventing orphaned subprocesses"
  - decision: "Read pipes concurrently before cmd.Wait()"
    rationale: "Prevents deadlocks when subprocess output exceeds pipe buffer capacity"
    impact: "Robust subprocess execution even with large output volumes"
  - decision: "ProcessManager tracks all subprocesses centrally"
    rationale: "Enables graceful shutdown of all backends on SIGINT/SIGTERM"
    impact: "Clean orchestrator shutdown without zombie processes"
metrics:
  duration_seconds: 96
  tasks_completed: 2
  files_created: 4
  commits: 2
  completed_date: "2026-02-10"
---

# Phase 01 Plan 01: Subprocess Management and Backend Abstraction Summary

**One-liner:** Go module with Backend interface, Message/Response types, concurrent pipe reading, process groups, and ProcessManager for subprocess lifecycle management.

## Plan Overview

**Objective:** Set up the Go module, define the Backend interface, shared types, and subprocess management utilities that all adapters will use.

**Outcome:** Successfully established foundational abstraction and subprocess infrastructure. All adapters (Claude Code, Codex, Goose) can now build on this shared interface and subprocess utilities.

## Tasks Completed

### Task 1: Initialize Go module and define shared types
**Status:** ✓ Complete
**Commit:** 4316778
**Files:** go.mod, internal/backend/types.go

- Initialized `github.com/aristath/orchestrator` Go module
- Defined `Message` struct with `Content` and `Role` fields
- Defined `Response` struct with `Content`, `SessionID`, and `Error` fields
- Defined `Config` struct with `Type`, `WorkDir`, `SessionID`, `Model`, `Provider`, and `SystemPrompt` fields
- All types compile cleanly and are ready for adapter use

### Task 2: Define Backend interface and subprocess utilities
**Status:** ✓ Complete
**Commit:** 1b05071
**Files:** internal/backend/backend.go, internal/backend/process.go

- Defined `Backend` interface with `Send(ctx, Message) (Response, error)`, `Close() error`, and `SessionID() string` methods
- Created `New(cfg Config) (Backend, error)` factory function with placeholder cases for claude, codex, and goose adapters
- Implemented `newCommand()` with `SysProcAttr{Setpgid: true}` for process group isolation
- Implemented `executeCommand()` with concurrent pipe reading pattern:
  - Creates stdout/stderr pipes
  - Starts command
  - Reads both pipes concurrently in goroutines
  - Waits for pipe readers to complete (wg.Wait)
  - Then calls cmd.Wait() to prevent deadlocks
- Implemented `killProcessGroup()` to terminate entire process groups via `syscall.Kill(-pid, SIGKILL)`
- Created `ProcessManager` struct with:
  - `Track(cmd)` - registers subprocess after Start
  - `Untrack(cmd)` - removes subprocess after Wait
  - `KillAll()` - terminates all tracked subprocesses
  - `Count()` - returns number of tracked processes

## Deviations from Plan

None - plan executed exactly as written.

## Technical Decisions

### 1. Process Group Isolation via Setpgid
**Context:** Subprocesses may spawn child processes (e.g., Claude Code spawning editor, Goose spawning local LLM server).

**Decision:** Set `SysProcAttr{Setpgid: true}` on all commands via `newCommand()`.

**Rationale:** Creates a new process group for each subprocess. When terminating, we can kill the entire group via `syscall.Kill(-pid, SIGKILL)`, ensuring all descendants are cleaned up.

**Impact:** Clean shutdown without orphaned subprocesses. Addresses BACK-06 requirement.

### 2. Concurrent Pipe Reading Pattern
**Context:** Large subprocess output can fill pipe buffers, causing deadlocks if not read before `cmd.Wait()`.

**Decision:** Implement `executeCommand()` to read stdout/stderr concurrently in goroutines, wait for both to complete, then call `cmd.Wait()`.

**Rationale:** This is the standard Go pattern for preventing subprocess deadlocks. Goroutines drain pipes while subprocess runs, ensuring buffers never fill.

**Impact:** Robust subprocess execution even with large output. Addresses BACK-05 requirement.

### 3. Centralized ProcessManager
**Context:** Multiple backend instances may be running concurrently, each with its own subprocess.

**Decision:** Create `ProcessManager` to track all subprocesses centrally, with `KillAll()` method for shutdown.

**Rationale:** Enables graceful orchestrator shutdown on SIGINT/SIGTERM by terminating all backends. `Track()`/`Untrack()` lifecycle ensures accurate tracking.

**Impact:** Clean shutdown path for orchestrator. Addresses BACK-07 requirement. Sets up pattern for future signal handling in main.

## Verification Results

✓ `go mod tidy` - succeeded
✓ `go vet ./...` - no warnings
✓ `go build ./...` - compiles cleanly
✓ Backend interface has Send, Close, SessionID methods
✓ executeCommand reads pipes concurrently before cmd.Wait()
✓ newCommand sets Setpgid: true
✓ ProcessManager has Track, Untrack, KillAll, Count methods
✓ New() factory exists with cases for claude, codex, goose

## Next Steps

This plan provides the foundation for adapter implementations:

1. **Plan 01-02:** Implement Claude Code adapter using Backend interface
2. **Plan 01-03:** Implement Codex adapter using Backend interface
3. **Plan 01-04:** Implement Goose adapter using Backend interface
4. **Plan 01-05:** Integration tests to verify all adapters work with subprocess utilities

All adapters will:
- Implement the `Backend` interface
- Use `newCommand()` for process group isolation
- Use `executeCommand()` for robust subprocess execution
- Register with `ProcessManager` for lifecycle tracking

## Self-Check: PASSED

All claimed artifacts verified:

- ✓ go.mod
- ✓ internal/backend/types.go
- ✓ internal/backend/backend.go
- ✓ internal/backend/process.go
- ✓ Commit 4316778
- ✓ Commit 1b05071
