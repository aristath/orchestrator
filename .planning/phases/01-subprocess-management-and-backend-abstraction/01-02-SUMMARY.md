---
phase: 01-subprocess-management-and-backend-abstraction
plan: 02
subsystem: backend-adapters
tags: [claude-adapter, subprocess-cli, session-management, json-parsing]
dependency_graph:
  requires:
    - backend.Backend interface (from 01-01)
    - executeCommand with concurrent pipes (from 01-01)
    - newCommand with process groups (from 01-01)
    - ProcessManager (from 01-01)
  provides:
    - ClaudeAdapter implementing Backend
    - NewClaudeAdapter factory function
    - Claude Code CLI command construction
    - Claude Code JSON response parser
    - UUID v4 generation for session IDs
  affects:
    - Factory function in backend.go
    - executeCommand signature (added ProcessManager parameter)
tech_stack:
  added:
    - crypto/rand for UUID generation
    - encoding/json for response parsing
    - Claude Code CLI integration
  patterns:
    - Session management with --session-id vs --resume
    - JSON parsing with structured types
    - Optional ProcessManager for subprocess tracking
key_files:
  created:
    - internal/backend/claude.go
    - internal/backend/claude_test.go
  modified:
    - internal/backend/backend.go (wired ClaudeAdapter into factory)
    - internal/backend/process.go (added ProcessManager parameter to executeCommand)
decisions:
  - decision: "Generate UUIDs without external dependencies"
    rationale: "Implemented simple v4 UUID generation using crypto/rand to avoid adding dependencies"
    impact: "Self-contained UUID generation, no external package needed"
  - decision: "Track subprocesses via optional ProcessManager in executeCommand"
    rationale: "Modified executeCommand to accept optional ProcessManager, enabling graceful shutdown"
    impact: "Adapters can opt into subprocess tracking for lifecycle management"
  - decision: "Parse Claude Code JSON with private structs"
    rationale: "Used claudeResponse struct to unmarshal nested JSON structure"
    impact: "Clean separation between CLI format and Backend Response format"
metrics:
  duration_seconds: 231
  tasks_completed: 2
  files_created: 2
  files_modified: 2
  commits: 2
  completed_date: "2026-02-10"
---

# Phase 01 Plan 02: Claude Code Adapter Implementation Summary

**One-liner:** Claude Code CLI adapter with session management (--session-id/--resume), JSON response parsing, optional ProcessManager tracking, and comprehensive unit tests.

## Plan Overview

**Objective:** Implement the Claude Code CLI adapter that can start sessions, send prompts, receive JSON responses, and resume conversations.

**Outcome:** Successfully implemented ClaudeAdapter with full Backend interface support. Command construction produces correct flags for new sessions vs resuming. JSON parser handles Claude Code output format. All 9 unit tests pass.

## Tasks Completed

### Task 1: Implement Claude Code adapter
**Status:** ✓ Complete
**Commit:** 0ab680b
**Files:** internal/backend/claude.go, internal/backend/backend.go, internal/backend/process.go

**Implementation Details:**
- Created `ClaudeAdapter` struct with fields:
  - `sessionID string` - UUID for session management
  - `workDir string` - working directory for CLI execution
  - `model string` - optional model override
  - `systemPrompt string` - optional system prompt override
  - `started bool` - tracks first message (determines --session-id vs --resume)
  - `procMgr *ProcessManager` - optional subprocess tracking

- Implemented `NewClaudeAdapter(cfg Config, procMgr *ProcessManager)`:
  - Auto-generates UUID v4 if cfg.SessionID is empty
  - Defaults workDir to current directory if not provided
  - Stores model and systemPrompt from config
  - ProcessManager is optional (nil safe)

- Implemented `Send(ctx context.Context, msg Message) (Response, error)`:
  - Builds args via `buildArgs(msg, started)`
  - Creates command with `newCommand()` for process group isolation
  - Executes via `executeCommand()` with optional ProcessManager tracking
  - Parses JSON response via `parseClaudeResponse()`
  - Sets `started = true` after first successful call
  - Returns Response with extracted content and session ID

- Implemented `buildArgs(msg Message, isResume bool) []string`:
  - Base args: `["-p", msg.Content, "--output-format", "json"]`
  - First call: appends `["--session-id", sessionID]`
  - Resume call: appends `["--resume", sessionID]`
  - Optional: appends `["--model", model]` if configured
  - Optional: appends `["--system-prompt", systemPrompt]` if configured

- Implemented `parseClaudeResponse(data []byte) (Response, error)`:
  - Unmarshals JSON into private `claudeResponse` struct
  - Extracts text content from `result.content[]` array
  - Handles multiple text items by concatenating
  - Maps to Backend Response struct

- Implemented `generateUUID() (string, error)`:
  - Uses `crypto/rand` to generate 16 random bytes
  - Sets version 4 bits and variant bits per RFC 4122
  - Formats as `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`

- Updated `executeCommand` signature:
  - Added `pm *ProcessManager` parameter (optional, nil-safe)
  - Tracks subprocess after Start if pm is non-nil
  - Untracks via defer after Wait

- Updated `backend.go` factory:
  - Added `pm *ProcessManager` parameter to `New()` function
  - Wired `case "claude": return NewClaudeAdapter(cfg, pm)`

### Task 2: Add Claude Code adapter unit tests
**Status:** ✓ Complete
**Commit:** a5c4e60
**Files:** internal/backend/claude_test.go

**Test Coverage (9 tests, all passing):**

1. **TestNewClaudeAdapter_GeneratesSessionID**
   - Verifies auto-generation when SessionID not provided
   - Validates UUID v4 format via regex
   - Pattern: `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`

2. **TestNewClaudeAdapter_UsesProvidedSessionID**
   - Verifies provided SessionID is used exactly
   - Tests SessionID passthrough from config

3. **TestClaudeAdapter_BuildsFirstMessageCommand**
   - Tests `buildArgs(msg, false)` for first message
   - Verifies args contain `--session-id`
   - Verifies args do NOT contain `--resume`

4. **TestClaudeAdapter_BuildsResumeCommand**
   - Tests `buildArgs(msg, true)` for resume
   - Verifies args contain `--resume`
   - Verifies args do NOT contain `--session-id`

5. **TestClaudeAdapter_IncludesModel**
   - Tests model flag inclusion when configured
   - Verifies `--model` flag and value in args

6. **TestClaudeAdapter_IncludesSystemPrompt**
   - Tests system prompt flag when configured
   - Verifies `--system-prompt` flag and value in args

7. **TestClaudeAdapter_ParsesJSONResponse**
   - Table-driven test with 6 cases:
     - Valid response with single text content
     - Valid response with multiple text content (concatenation)
     - Valid response with mixed content types (filters text)
     - Empty content array (returns empty string)
     - Invalid JSON (returns error)
     - Malformed structure (parses gracefully)

8. **TestClaudeAdapter_Close**
   - Verifies Close() returns nil (no-op)
   - Tests subprocess-per-invocation model

9. **TestClaudeAdapter_Send_MarksAsStarted**
   - Tests started flag behavior
   - Verifies first call uses --session-id
   - Verifies subsequent calls use --resume

## Deviations from Plan

None - plan executed exactly as written.

## Technical Decisions

### 1. Self-Contained UUID Generation
**Context:** Need session IDs but want to avoid external dependencies.

**Decision:** Implemented `generateUUID()` using `crypto/rand` to generate v4 UUIDs.

**Rationale:** Simple 16-byte random generation + bit manipulation is sufficient. No need for external UUID library.

**Impact:** Zero external dependencies for session management. Generated UUIDs are cryptographically random and RFC 4122 compliant.

### 2. Optional ProcessManager Parameter
**Context:** Adapters need subprocess tracking for graceful shutdown, but not all use cases require it.

**Decision:** Modified `executeCommand()` to accept optional `*ProcessManager`. If non-nil, Track after Start, Untrack via defer.

**Rationale:** Keeps the pattern clean - adapters pass their ProcessManager to executeCommand, which handles tracking lifecycle automatically.

**Impact:** Adapters can opt into subprocess tracking. Tests can pass nil. Production orchestrator will provide ProcessManager for KillAll on shutdown.

### 3. Separate buildArgs Method for Testability
**Context:** Need to test command construction without spawning subprocesses.

**Decision:** Extracted `buildArgs(msg Message, isResume bool) []string` as a testable method.

**Rationale:** Pure function that builds args based on state. Easy to test without I/O.

**Impact:** Unit tests can verify --session-id vs --resume logic, model/system-prompt inclusion, without executing CLI.

### 4. JSON Parsing with Private Structs
**Context:** Claude Code returns nested JSON: `{"session_id": "...", "result": {"content": [{"type": "text", "text": "..."}]}}`.

**Decision:** Defined private `claudeResponse` struct to unmarshal, then map to Backend Response.

**Rationale:** Clean separation between CLI output format and internal Response format. Adapters own their parsing logic.

**Impact:** Backend interface remains CLI-agnostic. Each adapter handles its own JSON structure.

## Verification Results

✓ `go test ./internal/backend/ -v -run TestClaude` - all 9 tests pass
✓ `go build ./...` - compiles cleanly
✓ ClaudeAdapter satisfies Backend interface (compile-time check)
✓ buildArgs produces correct flags for new session vs resume
✓ JSON parser handles Claude Code response format with 6 test cases
✓ UUID generation produces valid v4 UUIDs
✓ ProcessManager integration works (Track/Untrack via executeCommand)

## Next Steps

This plan provides the Claude Code adapter for the orchestrator:

1. **Plan 01-03:** Implement Codex adapter (in progress - codex.go exists but uncommitted)
2. **Plan 01-04:** Implement Goose adapter (completed - 01-04-SUMMARY.md exists)
3. **Plan 01-05:** Integration tests to verify all adapters work end-to-end

Claude Code adapter is now ready to use:
```go
pm := backend.NewProcessManager()
cfg := backend.Config{
    Type: "claude",
    WorkDir: "/path/to/project",
    Model: "claude-opus-4",
}
adapter, _ := backend.New(cfg, pm)
resp, _ := adapter.Send(ctx, backend.Message{Content: "Hello"})
```

## Self-Check: PASSED

All claimed artifacts verified:

**Files created:**
- ✓ internal/backend/claude.go (167 lines)
- ✓ internal/backend/claude_test.go (322 lines)

**Files modified:**
- ✓ internal/backend/backend.go (wired ClaudeAdapter)
- ✓ internal/backend/process.go (added ProcessManager parameter)

**Commits:**
- ✓ Commit 0ab680b (feat: implement Claude Code adapter)
- ✓ Commit a5c4e60 (test: add unit tests)

**Tests:**
- ✓ All 9 tests pass
- ✓ Coverage: session ID generation, command construction, JSON parsing, flag inclusion

**Build:**
- ✓ `go build ./...` succeeds
- ✓ `go vet ./...` clean
