---
phase: 02-agent-definitions-and-dag-scheduler
plan: 01
subsystem: Configuration Management
tags: [config, types, loader, defaults, json]
dependency_graph:
  requires: []
  provides: [config-types, config-loader, default-agents]
  affects: [agent-registry, workflow-execution]
tech_stack:
  added: [encoding/json, os.UserHomeDir]
  patterns: [map-merge, config-hierarchy, zero-dependency]
key_files:
  created:
    - internal/config/types.go
    - internal/config/defaults.go
    - internal/config/loader.go
    - internal/config/loader_test.go
  modified: []
decisions:
  - Map-level merge for providers/agents/workflows enables independent overrides
  - Project config has highest precedence, then global, then defaults
  - Missing config files are not errors, enabling graceful degradation
  - Zero external config libraries, using stdlib encoding/json
metrics:
  duration: 97
  completed: 2026-02-10T16:51:32Z
---

# Phase 02 Plan 01: Configuration System Summary

**One-liner:** Type-safe config loader with three-tier hierarchy (defaults -> global -> project) and map-level merge semantics for providers, agents, and workflows.

## What Was Built

Implemented the configuration foundation for the orchestrator system with support for:

1. **Type definitions** for the entire config hierarchy:
   - `ProviderConfig`: CLI command and backend type (claude, codex, goose)
   - `AgentConfig`: Role-specific provider, model, system prompt, and tools
   - `WorkflowConfig`: Pipeline of agent steps
   - `OrchestratorConfig`: Top-level container with Providers, Agents, Workflows maps

2. **Default configuration** with production-ready built-ins:
   - 3 providers: claude, codex, goose
   - 4 agents: orchestrator, coder, reviewer, tester
   - 1 workflow: standard (coder -> reviewer -> tester)

3. **Config loader** with merge semantics:
   - Three-tier hierarchy: defaults -> global (~/.orchestrator/config.json) -> project (.orchestrator/config.json)
   - Map-level merge: each map (Providers, Agents, Workflows) merged independently
   - Missing files handled gracefully (not errors)
   - Malformed JSON returns descriptive errors

4. **Comprehensive test coverage** with 8 test cases:
   - No config files (defaults only)
   - Global-only (adds new agent)
   - Project-only (overrides existing agent)
   - Both with merge (global adds, project overrides)
   - Project overrides global (project wins)
   - Malformed JSON error handling
   - Missing files not errors

## Verification Results

**All verification steps passed:**

- `go build ./internal/config/` - compiles without errors
- `go test ./internal/config/ -v -count=1` - all 8 test cases pass
- `go vet ./internal/config/` - no issues

**Test output:**
```
=== RUN   TestLoad
=== RUN   TestLoad/No_config_files_-_returns_defaults
=== RUN   TestLoad/Global_only_-_adds_new_agent
=== RUN   TestLoad/Project_only_-_overrides_agent_provider
=== RUN   TestLoad/Both_with_merge_-_global_adds,_project_overrides
=== RUN   TestLoad/Project_overrides_global_-_project_wins
--- PASS: TestLoad (0.00s)
=== RUN   TestLoad_MalformedJSON
--- PASS: TestLoad_MalformedJSON (0.00s)
=== RUN   TestLoad_MissingFilesNotError
--- PASS: TestLoad_MissingFilesNotError (0.00s)
PASS
ok      github.com/aristath/orchestrator/internal/config        0.511s
```

## Task Breakdown

### Task 1: Config types and default definitions
**Status:** Complete
**Commit:** b4887ea
**Files:** internal/config/types.go, internal/config/defaults.go

Created type hierarchy for configuration system:
- ProviderConfig with Command, Args, Type fields
- AgentConfig with Provider, Model, SystemPrompt, Tools fields
- WorkflowConfig and WorkflowStepConfig for pipeline definitions
- OrchestratorConfig as top-level container

Implemented DefaultConfig() with 3 providers, 4 agents, 1 workflow. All built-in agents use claude provider by default with concise role-specific prompts.

### Task 2: Config loader with global/project merge and tests
**Status:** Complete
**Commit:** f0cb39e
**Files:** internal/config/loader.go, internal/config/loader_test.go

Implemented Load() and LoadDefault() functions with three-tier merge:
1. Start with DefaultConfig()
2. Merge global config if exists
3. Merge project config if exists (highest precedence)

Map-level merge enables independent override of providers, agents, and workflows. Project can add new agent without redefining all agents.

Test suite covers all edge cases including defaults-only, global-only, project-only, merge precedence, malformed JSON, and missing files.

## Deviations from Plan

None - plan executed exactly as written.

## Key Decisions

1. **Map-level merge semantics** - Each map (Providers, Agents, Workflows) merged independently rather than full struct replacement. This enables granular overrides: project config can add one new agent without redefining all agents.

2. **Project config highest precedence** - Order: defaults -> global -> project. Project overrides global, global overrides defaults. This matches user expectations for config hierarchy.

3. **Missing files not errors** - Non-existent config files are silently skipped, enabling graceful degradation to defaults. Only malformed JSON is an error. This supports zero-config usage.

4. **Zero external dependencies** - Used stdlib encoding/json for parsing, consistent with Phase 1's zero-dependency approach. Keeps binary lean and avoids config library lock-in.

## Integration Points

**Provides to downstream systems:**
- Type definitions for all Phase 2 subsystems (agent registry needs AgentConfig, workflow engine needs WorkflowConfig)
- Config loading with merge semantics for global/project customization
- Default agents and workflows for zero-config usage

**Dependency relationships:**
- Phase 2 Plan 2 (Agent Registry) will consume AgentConfig and ProviderConfig types
- Phase 2 Plan 3 (DAG Scheduler) will consume WorkflowConfig for pipeline execution
- Phase 1 Backend subsystem will be referenced via ProviderConfig.Type field

## Performance Notes

- Config loading is fast: all tests complete in 0.511s
- Small memory footprint: defaults include only 4 agents, 3 providers, 1 workflow
- No external dependencies or heavy parsing libraries

## Next Steps

This completes Plan 01. Next plan (02-02) will build the Agent Registry that consumes these config types to instantiate agents.

The config system provides the foundation for all Phase 2 agent and workflow management.

## Self-Check: PASSED

**Files created:**
- FOUND: internal/config/types.go
- FOUND: internal/config/defaults.go
- FOUND: internal/config/loader.go
- FOUND: internal/config/loader_test.go

**Commits verified:**
- FOUND: b4887ea (Task 1: Config types and defaults)
- FOUND: f0cb39e (Task 2: Config loader with tests)

**Tests verified:**
- All 8 test cases pass
- go build compiles without errors
- go vet reports no issues
