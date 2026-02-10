---
phase: 01-subprocess-management-and-backend-abstraction
plan: 04
subsystem: goose-adapter
tags: [goose-cli, local-llm, session-management, json-parsing]
dependency_graph:
  requires:
    - backend.Backend interface (from 01-01)
    - executeCommand and newCommand (from 01-01)
    - ProcessManager (from 01-01)
  provides:
    - GooseAdapter implementing Backend interface
    - Goose CLI command construction with session management
    - Local LLM support via provider/model passthrough
    - JSON response parsing with plain text fallback
  affects:
    - Integration tests (01-05)
tech_stack:
  added:
    - crypto/rand for session ID generation
    - encoding/json for response parsing
    - Goose CLI integration patterns
  patterns:
    - Session management with --name (first) and --resume (subsequent)
    - Command builder pattern for testable CLI construction
    - Flexible JSON parsing with ndjson and plain text fallbacks
key_files:
  created:
    - internal/backend/goose.go
    - internal/backend/goose_test.go
  modified:
    - internal/backend/backend.go (factory wiring)
decisions:
  - decision: "Use --name for first message, --resume for subsequent messages"
    rationale: "Goose uses session names (not UUIDs) and --resume automatically picks up the previous session"
    impact: "Session continuity across multiple Send() calls; needs verification in integration tests"
  - decision: "Generate session names as 'orchestrator-{random-hex}'"
    rationale: "Matches Goose naming convention and provides uniqueness without UUID complexity"
    impact: "Human-readable session names in Goose session list"
  - decision: "Pass --provider and --model directly to Goose CLI"
    rationale: "Goose handles all LLM provider communication; adapter just needs to pass configuration"
    impact: "Simple local LLM support for Ollama, LM Studio, llama.cpp without adapter complexity"
  - decision: "Parse JSON with fallback to plain text"
    rationale: "Goose JSON output format may vary or --output-format json may not be fully supported"
    impact: "Robust response handling; degrades gracefully if JSON parsing fails"
metrics:
  duration_seconds: 142
  tasks_completed: 2
  files_created: 2
  commits: 2
  completed_date: "2026-02-10"
---

# Phase 01 Plan 04: Goose CLI Adapter with Local LLM Support Summary

**One-liner:** GooseAdapter implementation with session management (--name/--resume), local LLM provider passthrough (--provider/--model), and flexible JSON response parsing.

## Plan Overview

**Objective:** Implement the Goose CLI adapter that can start sessions, send prompts, receive JSON responses, resume conversations, and configure local LLM providers.

**Outcome:** Successfully implemented GooseAdapter with full Backend interface compliance. Goose now provides local LLM capabilities (Ollama, LM Studio, llama.cpp) through simple CLI flag passthrough. All unit tests pass, confirming command construction and JSON parsing logic.

## Tasks Completed

### Task 1: Implement Goose adapter with local LLM support
**Status:** ✓ Complete
**Commit:** bf91e99
**Files:** internal/backend/goose.go, internal/backend/backend.go

- Created `GooseAdapter` struct with fields:
  - `sessionName` - generated as "orchestrator-{random-hex}" if not provided
  - `workDir` - working directory for CLI invocation
  - `model` and `provider` - local LLM configuration
  - `systemPrompt` - custom system instructions
  - `started` - tracks whether first message has been sent
  - `procMgr` - reference to shared ProcessManager
- Implemented `NewGooseAdapter(cfg, procMgr)` with session name generation
- Implemented `Send(ctx, msg)`:
  - First call uses `--name` flag to start new session
  - Subsequent calls use `--resume` flag to continue session
  - Passes `--provider` and `--model` for local LLM support
  - Passes `--system` for custom system prompts
  - Uses `--output-format json` for structured responses
  - Parses JSON with fallback to plain text if parsing fails
- Implemented `buildArgs(msg)` as testable method for CLI construction
- Implemented `parseGooseResponse(data)`:
  - Tries single JSON object parsing first
  - Falls back to newline-delimited JSON (ndjson) format
  - Returns error if both fail (triggers plain text fallback in Send)
- Implemented `Close()` as no-op (per-invocation subprocess)
- Implemented `SessionID()` returning session name
- Wired up `NewGooseAdapter` in factory function for type "goose"

### Task 2: Add Goose adapter unit tests
**Status:** ✓ Complete
**Commit:** 66037f2
**Files:** internal/backend/goose_test.go

Created comprehensive test suite covering:

1. **TestNewGooseAdapter_GeneratesSessionName**: Verifies auto-generated session names have "orchestrator-" prefix with hex suffix
2. **TestNewGooseAdapter_UsesProvidedSessionName**: Verifies custom session names are preserved
3. **TestGooseAdapter_BuildsFirstRunCommand**: Verifies first call uses `--name` flag and NOT `--resume`
4. **TestGooseAdapter_BuildsResumeCommand**: Verifies subsequent calls use `--resume` flag and NOT `--name`
5. **TestGooseAdapter_IncludesProvider**: Verifies `--provider` flag is included when configured
6. **TestGooseAdapter_IncludesModel**: Verifies `--model` flag is included when configured
7. **TestGooseAdapter_IncludesSystemPrompt**: Verifies `--system` flag is included when configured
8. **TestGooseAdapter_LocalLLMConfig**: Verifies full local LLM scenario with both provider and model
9. **TestGooseAdapter_ParsesJSONResponse**: Verifies parsing of single JSON object response
10. **TestGooseAdapter_ParsesNewlineDelimitedJSON**: Verifies parsing of ndjson stream format
11. **TestGooseAdapter_ParsesPlainTextFallback**: Verifies error handling when JSON parsing fails
12. **TestGooseAdapter_Close**: Verifies Close() returns nil (no-op)
13. **TestGooseAdapter_ImplementsBackendInterface**: Compile-time verification of interface implementation

Added helper functions:
- `sliceContains(slice, value)` - checks if slice contains value
- `sliceContainsSequence(slice, sequence)` - checks if slice contains sequence in order

All tests pass successfully.

## Deviations from Plan

None - plan executed exactly as written.

## Technical Decisions

### 1. Session Name Format: "orchestrator-{random-hex}"

**Context:** Goose uses session names (not UUIDs) for session management. Plan specified this format.

**Decision:** Generate session names with format "orchestrator-{random-hex}" using 4 random bytes (8 hex characters).

**Rationale:** Human-readable names that are still unique enough for local development. Matches Goose naming conventions. Simpler than full UUID.

**Impact:** Session names are recognizable in `goose list` output. Format is consistent across orchestrator. Adequate uniqueness for single-developer use case.

### 2. Session Management: --name vs --resume

**Context:** Goose CLI uses `--name` to start a new session and `--resume` to continue the previous session.

**Decision:** Track `started` boolean. Use `--name` flag on first `Send()` call, `--resume` on subsequent calls.

**Rationale:** Follows Goose CLI semantics. First call establishes the session, subsequent calls continue it. The plan's note indicates this needs verification during integration testing (Plan 05) since Goose documentation is sparse.

**Impact:** Session continuity across multiple messages. Assumption that `--resume` automatically picks up the session established by `--name` needs hands-on verification in integration tests.

### 3. Local LLM Support via Flag Passthrough

**Context:** BACK-04 requirement specifies support for Ollama, LM Studio, and llama.cpp.

**Decision:** Pass `--provider` and `--model` flags directly to Goose CLI without any adapter-level LLM handling.

**Rationale:** Goose CLI already handles all LLM provider communication. The adapter's job is just configuration passthrough. This keeps the adapter simple and delegates complexity to Goose.

**Impact:** Clean separation of concerns. Local LLM support requires zero additional code beyond flag construction. Goose handles all provider-specific protocols (Ollama HTTP, LM Studio, llama.cpp).

**Examples:**
- Ollama: `--provider ollama --model llama2`
- LM Studio: `--provider lmstudio --model <model-name>`
- llama.cpp: `--provider llamacpp --model <model-path>`

### 4. Flexible JSON Parsing with Fallbacks

**Context:** Goose JSON output format is less documented. GitHub issue #4419 suggests `--output-format json` may not be fully supported.

**Decision:** Implement three-tier parsing strategy:
1. Try parsing as single JSON object
2. Try parsing as newline-delimited JSON (ndjson)
3. Fall back to treating entire stdout as plain text (in `Send()` method)

**Rationale:** Robust handling of different Goose output formats. Degrades gracefully if JSON parsing fails. Same ndjson approach as Codex adapter (from 01-03).

**Impact:** Adapter works regardless of Goose output format. If `--output-format json` is broken, plain text fallback ensures functionality. Integration tests will reveal actual Goose behavior.

## Verification Results

✓ `go test ./internal/backend/ -v -run TestGoose` - all 13 tests pass
✓ `go build ./...` - compiles cleanly
✓ GooseAdapter implements Backend interface (compile-time check)
✓ Command construction includes `--name` for first call, `--resume` for subsequent
✓ Local LLM flags (`--provider`, `--model`) correctly included when configured
✓ System prompt flag (`--system`) correctly included when configured
✓ JSON parsing handles single object, ndjson, and returns error for invalid JSON
✓ Factory function creates GooseAdapter for type "goose"

## Blockers/Concerns

**From STATE.md:** "Goose session management (`--session-id`/`--resume`) needs hands-on verification"

**Status:** Partially addressed. Unit tests verify command construction logic. However, actual Goose CLI behavior with `--name` + `--resume` combination needs verification in integration tests (Plan 01-05). Specifically:

1. Does `--resume` pick up the session started with `--name`?
2. Does Goose require `--name` on resume, or is `--resume` alone sufficient?
3. What happens if session doesn't exist?

These questions will be answered in integration testing.

## Next Steps

**Plan 01-05:** Integration tests to verify all adapters (Claude Code, Codex, Goose) work with real CLI invocations. For Goose specifically:

1. Verify `--name` creates a new session
2. Verify `--resume` continues the session
3. Test local LLM configuration with real Ollama/LM Studio setup
4. Verify JSON output format with real Goose CLI
5. Test fallback to plain text if JSON parsing fails

## Self-Check: PASSED

All claimed artifacts verified:

- ✓ internal/backend/goose.go exists
- ✓ internal/backend/goose_test.go exists
- ✓ internal/backend/backend.go modified (factory wiring)
- ✓ Commit bf91e99 (Task 1 - Goose adapter implementation)
- ✓ Commit 66037f2 (Task 2 - Unit tests)
- ✓ GooseAdapter implements Backend interface
- ✓ All unit tests pass
- ✓ Code compiles cleanly
