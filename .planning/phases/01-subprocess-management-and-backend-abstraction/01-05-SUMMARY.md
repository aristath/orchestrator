---
phase: 01-subprocess-management-and-backend-abstraction
plan: 05
subsystem: backend-testing
tags: [stress-testing, integration-testing, subprocess-validation, factory-testing]
dependency_graph:
  requires:
    - Backend interface and subprocess utilities (01-01)
    - ClaudeAdapter (01-02)
    - CodexAdapter (01-03)
    - GooseAdapter (01-04)
  provides:
    - Comprehensive subprocess infrastructure test suite
    - Factory and cross-backend validation tests
    - Mock CLI for subprocess testing
    - Validation of all Phase 1 success criteria
  affects:
    - Future integration testing patterns
tech_stack:
  added:
    - testdata/mock-cli.sh for subprocess simulation
    - Stress test patterns for deadlock and zombie prevention
  patterns:
    - Mock CLI with configurable behavior (echo, large output, sleep, spawn child)
    - Stress testing for subprocess management
    - Cross-backend factory testing
    - Table-driven tests for multiple backend types
key_files:
  created:
    - testdata/mock-cli.sh
    - internal/backend/process_test.go
    - internal/backend/backend_test.go
  modified: []
decisions:
  - decision: "Use mock CLI script for subprocess testing"
    rationale: "Bash script simulates agent CLI behavior without requiring actual CLI installations"
    impact: "Tests can run in CI without Claude Code, Codex, or Goose installed"
  - decision: "Test with 256KB output for deadlock prevention"
    rationale: "256KB is well above 64KB pipe buffer, ensuring concurrent pipe reading is tested under stress"
    impact: "Proves BACK-05 requirement (no pipe deadlocks) is satisfied"
  - decision: "Use 15 sequential invocations for zombie test"
    rationale: "Exceeds phase success criterion of 10+ invocations, provides safety margin"
    impact: "Validates BACK-07 and proves subprocess cleanup works at scale"
metrics:
  duration_seconds: 334
  tasks_completed: 2
  files_created: 3
  commits: 2
  completed_date: "2026-02-10"
---

# Phase 01 Plan 05: Subprocess Infrastructure Validation Summary

**One-liner:** Comprehensive test suite validating zero zombies after 15+ invocations, no deadlocks on 256KB+ output, process tree termination, and correct factory behavior for all three backend types.

## Plan Overview

**Objective:** Validate the subprocess infrastructure with stress tests for zombie prevention, deadlock prevention, signal propagation, and factory correctness.

**Outcome:** Successfully validated all Phase 1 success criteria through comprehensive testing. All adapters (Claude, Codex, Goose) can be created via factory, subprocess infrastructure handles stress conditions without deadlocks or zombies, and process groups enable clean termination of subprocess trees.

## Tasks Completed

### Task 1: Create mock CLI script and subprocess stress tests
**Status:** Complete
**Commit:** a1ae366
**Files:** testdata/mock-cli.sh, internal/backend/process_test.go

**Mock CLI Implementation:**
- Created `testdata/mock-cli.sh` with five modes:
  - `--echo "text"` - Output JSON to stdout for basic tests
  - `--large-output N` - Generate N KB of output (one JSON object per line)
  - `--sleep N` - Sleep for N seconds with graceful SIGTERM handling
  - `--spawn-child` - Fork a background `sleep 300` child process
  - `--exit-code N` - Exit with specified code
- Made executable with `chmod +x`
- Simulates agent CLI behavior without requiring actual CLI installations

**Test Suite (8 tests, all passing):**

1. **TestExecuteCommand_BasicExecution**
   - Verifies basic command execution works
   - Tests stdout capture with simple echo command
   - Validates empty stderr on successful execution

2. **TestExecuteCommand_ConcurrentPipeReading_LargeOutput**
   - **Critical BACK-05 test** - validates no deadlocks on large output
   - Generates 256KB of output (well above 64KB pipe buffer)
   - Uses 10-second timeout to detect potential deadlocks
   - Successfully processes 13,107 lines in ~60ms
   - Proves concurrent pipe reading pattern prevents deadlocks

3. **TestExecuteCommand_StderrCapture**
   - Verifies both stdout and stderr are captured simultaneously
   - Tests command that writes to both streams
   - Validates concurrent pipe reading works for both pipes

4. **TestExecuteCommand_ContextCancellation**
   - Starts subprocess with 30-second sleep
   - Cancels context after 500ms
   - Verifies subprocess is terminated (receives signal: killed)
   - Validates context-based subprocess lifecycle management

5. **TestProcessManager_TrackAndKillAll**
   - Creates ProcessManager and starts long-running subprocess
   - Verifies Track() increments count to 1
   - Calls KillAll() and verifies subprocess receives signal
   - Verifies Untrack() decrements count to 0
   - Tests basic ProcessManager lifecycle

6. **TestProcessManager_KillsProcessTree**
   - **Critical BACK-06 test** - validates process group signal propagation
   - Starts subprocess that spawns a child process
   - Calls KillAll() on parent
   - Uses `pgrep -P <parent-pid>` to verify children are terminated
   - Proves `Setpgid: true` enables clean termination of entire process trees

7. **TestNoZombieProcesses_StressTest**
   - **Critical BACK-07 test** - validates phase success criterion
   - Runs 15 sequential subprocess invocations (exceeds 10+ requirement)
   - Each invocation starts subprocess, captures output, waits for completion
   - After all complete, waits 1 second for cleanup
   - Scans `ps -eo pid,stat,command` for processes in Z (zombie) state
   - Verifies zero zombies found
   - Proves subprocess cleanup pattern works at scale

8. **TestExecuteCommand_NonZeroExitCode**
   - Tests subprocess that exits with code 1
   - Verifies error is returned and unwrapped properly
   - Validates stdout/stderr are still captured despite error
   - Tests error handling doesn't break output capture

**All tests pass in 31.9 seconds.**

### Task 2: Add factory and cross-backend tests
**Status:** Complete
**Commit:** 47564a2
**Files:** internal/backend/backend_test.go

**Test Suite (7 test functions, all passing):**

1. **TestFactory_CreatesClaudeAdapter**
   - Calls `New(Config{Type: "claude"}, pm)`
   - Verifies no error returned
   - Verifies SessionID() returns non-empty UUID
   - Confirms Claude adapter creation works

2. **TestFactory_CreatesCodexAdapter**
   - Calls `New(Config{Type: "codex"}, pm)`
   - Verifies no error returned
   - Confirms Codex adapter creation works
   - SessionID() is empty until first message (as expected)

3. **TestFactory_CreatesGooseAdapter**
   - Calls `New(Config{Type: "goose"}, pm)`
   - Verifies no error returned
   - Verifies SessionID() returns non-empty session name
   - Validates format: `orchestrator-{hex}`
   - Confirms Goose adapter creation works

4. **TestFactory_UnknownType**
   - Calls `New(Config{Type: "unknown"}, pm)`
   - Verifies error is returned
   - Verifies error message contains "unknown backend type"
   - Verifies nil backend is returned
   - Tests error handling for invalid backend types

5. **TestFactory_AllTypesImplementBackend**
   - Table-driven test for all three backend types
   - For each type (claude, codex, goose):
     - Creates adapter via factory
     - Calls SessionID() to verify interface method works
     - Calls Close() to verify cleanup works
   - Runtime verification that all adapters satisfy Backend interface

6. **TestFactory_PassesConfig**
   - Tests config passthrough for all three backend types
   - Claude test: passes Model field
   - Codex test: passes Model field
   - Goose test: passes Model and Provider fields
   - Verifies adapters are created successfully with custom config
   - Confirms factory properly passes config to constructors

7. **TestAllAdapters_CloseIsIdempotent**
   - For each backend type (claude, codex, goose):
     - Creates adapter
     - Calls Close() twice
     - Verifies no error or panic on second call
   - Validates safe cleanup behavior

**All tests pass in 0.6 seconds.**

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

All verification criteria satisfied:

**Subprocess Infrastructure Tests:**
- Zero zombie processes after 15 sequential invocations (BACK-07)
- No deadlock on 256KB subprocess output (BACK-05)
- ProcessManager kills process trees via process groups (BACK-06)
- Context cancellation terminates subprocesses correctly
- Stdout and stderr captured concurrently without deadlock
- Error handling works while preserving output capture

**Factory and Integration Tests:**
- All three backend types (claude, codex, goose) can be created via factory
- Unknown backend types return clear error message
- All adapters implement Backend interface (compile-time + runtime verification)
- Config fields are passed through to adapters correctly
- Close() is idempotent for all adapters

**Full Test Suite:**
- Total tests: 46 (8 process tests + 7 factory tests + 31 adapter unit tests)
- All tests pass with zero failures
- Test execution time: ~32 seconds (mostly context cancellation timeout)

## Phase 1 Success Criteria Validation

All seven Phase 1 success criteria are now validated by tests:

1. **Backend interface exists with Send/Close/SessionID** - Verified by factory tests and compile-time checks
2. **Claude Code adapter builds correct commands** - Verified by unit tests in 01-02
3. **Codex adapter builds correct commands** - Verified by unit tests in 01-03
4. **Goose adapter supports local LLMs via --provider/--model** - Verified by unit tests in 01-04
5. **No pipe deadlocks on large output (BACK-05)** - Verified by 256KB stress test in this plan
6. **Process group signal propagation kills subprocess trees (BACK-06)** - Verified by KillsProcessTree test in this plan
7. **Zero zombies after 10+ sequential invocations (BACK-07)** - Verified by 15-invocation stress test in this plan

**Phase 1 is complete and fully validated.**

## Technical Decisions

### 1. Mock CLI Script Pattern

**Context:** Need to test subprocess infrastructure without requiring actual agent CLI installations.

**Decision:** Create bash script that simulates agent CLI behavior with configurable modes.

**Rationale:**
- Tests can run in CI without Claude Code, Codex, or Goose installed
- Full control over subprocess behavior (output size, timing, child processes)
- Fast test execution (no network calls or heavy CLI overhead)
- Enables testing edge cases (large output, signal handling, process trees)

**Implementation:** Created `testdata/mock-cli.sh` with five modes controllable via flags. Each mode tests a specific subprocess pattern.

**Impact:** All subprocess tests are fast, reliable, and don't require external dependencies.

### 2. 256KB Output for Deadlock Testing

**Context:** Need to prove concurrent pipe reading prevents deadlocks when output exceeds pipe buffer.

**Decision:** Generate 256KB of output, which is well above the typical 64KB pipe buffer size.

**Rationale:**
- 64KB is the standard pipe buffer size on most systems
- 256KB (4x buffer size) ensures pipes would deadlock without concurrent reading
- Test actually processes 13,107 lines (~262KB) in ~60ms without issue

**Impact:** Definitively proves BACK-05 requirement is satisfied. No risk of deadlocks on large output.

### 3. 15 Sequential Invocations for Zombie Test

**Context:** Phase success criterion requires "10+ sequential subprocess invocations leave zero zombie processes."

**Decision:** Use 15 invocations, exceeding the requirement by 50%.

**Rationale:**
- Provides safety margin above minimum requirement
- Tests subprocess cleanup at realistic scale
- 15 invocations completes in ~1 second, so no performance penalty

**Impact:** Validates BACK-07 with confidence. Proves subprocess cleanup pattern works beyond minimum requirements.

### 4. Table-Driven Tests for Multiple Backends

**Context:** Need to test factory behavior across three different backend types.

**Decision:** Use table-driven tests with `t.Run()` subtests for each backend type.

**Rationale:**
- Clean, maintainable test structure
- Easy to add fourth backend type in future (e.g., Aider)
- Clear test output shows which backend failed if any do
- Reduces code duplication

**Impact:** Factory tests are concise and extensible. Adding new backend types requires minimal test changes.

## Next Steps

Phase 1 is complete. All subprocess infrastructure and backend adapters are implemented and validated.

**Phase 2: Agent Definition and Validation**
- Define agent configuration format (JSON/YAML with capabilities, context, constraints)
- Create agent registry and loader
- Implement agent validator
- Build agent templates for common patterns

**Phase 3: Work Distribution and Parallel Execution**
- Implement task queue with priority and dependencies
- Build parallel executor that spawns multiple backend instances
- Add resource management (concurrency limits, rate limiting)
- Create inter-agent communication patterns

**Phase 4: TUI for Monitoring**
- Build terminal UI with task list, agent status, output viewer
- Implement real-time log streaming from agents
- Add interactive controls (pause, resume, cancel)

**Phase 5: Persistence and Resumability**
- Create checkpoint system for task state
- Implement resume from checkpoint logic
- Add conversation history storage
- Build cleanup for completed tasks

**Phase 6: Error Handling and Resilience**
- Add retry logic with exponential backoff
- Implement circuit breaker for failing agents
- Create fallback strategies (different backend, different model)
- Add health checks and auto-recovery

## Self-Check: PASSED

All claimed artifacts verified:

**Files created:**
```bash
$ ls testdata/mock-cli.sh internal/backend/process_test.go internal/backend/backend_test.go
testdata/mock-cli.sh
internal/backend/process_test.go
internal/backend/backend_test.go
```

**Files are executable/readable:**
```bash
$ test -x testdata/mock-cli.sh && echo "mock-cli.sh is executable"
mock-cli.sh is executable

$ wc -l internal/backend/process_test.go internal/backend/backend_test.go
     319 internal/backend/process_test.go
     235 internal/backend/backend_test.go
```

**Commits exist:**
```bash
$ git log --oneline | head -2
47564a2 test(01-05): add factory and cross-backend tests
a1ae366 test(01-05): add mock CLI and subprocess stress tests
```

**Tests pass:**
```bash
$ go test ./... -v -timeout 120s | grep -E "^(PASS|FAIL|ok)"
PASS
ok  	github.com/aristath/orchestrator/internal/backend	31.618s
```

**Test coverage:**
- Subprocess infrastructure: 8 tests, all passing
- Factory and cross-backend: 7 tests, all passing
- Total Phase 1 tests: 46 tests, all passing

**Phase 1 success criteria:**
- BACK-05 (no deadlocks): Validated by 256KB output test
- BACK-06 (process tree termination): Validated by spawn-child test
- BACK-07 (zero zombies): Validated by 15-invocation stress test
- All three backend types work via factory
- Backend interface implemented and verified

All files exist, all tests pass, all success criteria validated.
