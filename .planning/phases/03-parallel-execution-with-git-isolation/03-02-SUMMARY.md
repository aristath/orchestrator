---
phase: 03-parallel-execution-with-git-isolation
plan: 02
subsystem: orchestrator
tags: [concurrency, channels, q&a, non-blocking]
dependency_graph:
  requires: []
  provides:
    - QAChannel for agent-orchestrator communication
    - Non-blocking question routing
  affects:
    - Plan 03 (parallel runner will wire in QAChannel)
tech_stack:
  added:
    - Go channels with buffered communication
    - Context-based cancellation
  patterns:
    - Per-question response channel for routing
    - Double select for cancellation safety
key_files:
  created:
    - internal/orchestrator/qa_channel.go
    - internal/orchestrator/qa_channel_test.go
  modified: []
decisions:
  - Buffer size configurable by caller (recommended 2x concurrency)
  - Per-question response channels prevent cross-talk without mutex
  - Serial question processing by single handler goroutine
  - Double select in Ask prevents goroutine leak on cancellation
metrics:
  duration: 144
  completed: 2026-02-10T17:33:21Z
---

# Phase 03 Plan 02: Non-Blocking Q&A Channel Summary

**One-liner:** Buffered Q&A channel enabling satellite agents to ask orchestrator questions without blocking other agents, using per-question response routing and context cancellation.

## Overview

Created `internal/orchestrator` package with `QAChannel` type that manages non-blocking communication between satellite agents and the orchestrator. The channel allows multiple agents to ask questions concurrently while one agent waits for an answer without stalling others.

## What Was Built

### Core Implementation

**QAChannel type** (`qa_channel.go`):
- `Question` struct with TaskID, Content, and private responseCh
- `Answer` struct with Content and Error
- `AnswerFunc` callback type for orchestrator's answer logic
- Buffered question channel (size configurable via constructor)
- Handler goroutine that processes questions serially until context cancelled
- `Ask()` method with double select for cancellation at send and receive stages
- `Start()` launches handler, `Stop()` blocks until handler exits

**Routing mechanism:**
- Each question carries its own buffered response channel (capacity 1)
- Handler sends answer back on the question's response channel
- No shared state, no mutex needed -- routing is intrinsic

**Context handling:**
- Handler respects `ctx.Done()` in select loop
- If context cancelled during answer generation, sends `ctx.Err()` to caller
- `Ask()` has two cancellation points: during send and during receive

### Test Coverage

**7 comprehensive tests** (`qa_channel_test.go`):
1. **TestAskAndReceive:** Happy path -- ask question, get answer
2. **TestMultipleConcurrentAskers:** 4 goroutines asking simultaneously, verify correct routing (no cross-talk)
3. **TestContextCancellation_AskBlocked:** Cancel context while trying to send, verify prompt return (<100ms)
4. **TestContextCancellation_StopsHandler:** Cancel context, verify handler exits cleanly
5. **TestSlowAnswer_DoesNotBlockOthers:** Slow answer (200ms) doesn't block other callers from sending questions
6. **TestAnswerError:** Errors from answer function propagate correctly
7. **TestAskAfterStop:** Asking on cancelled context returns error

All tests pass under `-race` flag.

## Deviations from Plan

None - plan executed exactly as written.

## Technical Decisions

**1. Buffer size configurable:**
- Caller specifies buffer size in `NewQAChannel(bufferSize, answerFn)`
- Research recommends 2x concurrency limit to prevent blocking at send stage
- Plan 03 will configure this based on max parallel agents

**2. Per-question response channels:**
- Each `Question` carries its own response channel
- Handler sends answer back on that specific channel
- Eliminates need for response map with mutex (simpler, safer)

**3. Serial question processing:**
- Single handler goroutine processes questions one at a time
- Sufficient for current use case (questions are rare)
- If parallel answering needed later, handler can spawn goroutines per question
- Test 5 verifies non-blocking send even with slow answers

**4. Double select pattern:**
- `Ask()` has two select statements: one for send, one for receive
- Prevents goroutine leak if context cancelled after send but before receive
- Standard Go pattern for cancellable channel operations

## Verification Results

- `go build ./internal/orchestrator/...` compiles cleanly
- `go vet ./internal/orchestrator/...` reports no issues
- All 7 QAChannel tests pass under `-race` flag
- Existing internal tests still pass: backend (33s), config (1.5s), scheduler (2.5s)

## Integration Points

**For Plan 03 (Parallel Runner):**
- Create `QAChannel` with buffer size 2x max concurrency
- Provide `AnswerFunc` that uses orchestrator's plan context to answer questions
- Call `Start(ctx)` before launching satellite agents
- Pass `QAChannel` reference to each satellite agent
- Agents call `qac.Ask(ctx, taskID, question)` when they need clarification
- Call `Stop()` after all agents finish

## Success Criteria Met

- [x] Satellite agents can ask questions via `Ask()` and receive answers
- [x] Multiple concurrent askers do not block each other at the send level
- [x] Context cancellation stops the handler and unblocks pending asks
- [x] Answer routing is correct (no cross-talk between task IDs)
- [x] All 7 tests pass under `-race`

## Performance Notes

- Test durations: 2s (basic), 2s (concurrent), 2s (cancel blocked), 0s (stop), 3s (slow answer), 2s (error), 0s (after stop)
- Total test time: ~12.5s with race detector
- Handler goroutine has negligible overhead when idle
- Per-question response channels add ~24 bytes per question (chan pointer)

## Self-Check: PASSED

**Created files exist:**
- FOUND: internal/orchestrator/qa_channel.go
- FOUND: internal/orchestrator/qa_channel_test.go

**Commits exist:**
- FOUND: b44f9a1 (feat: implement QAChannel)
- FOUND: a40945d (test: add comprehensive tests)

## Next Steps

Plan 03 will wire this channel into the parallel runner, enabling satellite agents to ask the orchestrator questions during execution without blocking other agents.
