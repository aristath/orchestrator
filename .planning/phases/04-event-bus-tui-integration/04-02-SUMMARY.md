---
phase: 04-event-bus-tui-integration
plan: 02
subsystem: ui
tags: [bubbletea, lipgloss, tui, terminal-ui, split-pane, viewport]

# Dependency graph
requires:
  - phase: 04-event-bus-tui-integration
    plan: 01
    provides: Event bus with SubscribeAll for cross-topic consumption
provides:
  - Bubble Tea TUI with split-pane layout (agent list, agent output, DAG progress)
  - Real-time event consumption and display
  - Vim-style keyboard navigation between panes
  - Demo entry point with fake events
affects: [04-03-interactive-controls]

# Tech tracking
tech-stack:
  added: [bubbletea@v1.3, lipgloss@v1.0, bubbles@v1.0]
  patterns:
    - Bubble Tea Model-Update-View pattern for TUI
    - Split-pane layout with lipgloss JoinHorizontal/JoinVertical
    - Event-driven UI updates via tea.Msg type switch
    - Viewport for scrollable output with debounced updates

key-files:
  created:
    - internal/tui/model.go
    - internal/tui/agent_pane.go
    - internal/tui/dag_pane.go
    - internal/tui/styles.go
    - internal/tui/keys.go
    - cmd/orchestrator/main.go
  modified: []

key-decisions:
  - "Use stable bubbletea v1.x instead of v2 beta for production reliability"
  - "Combine agent list and output viewport in single left pane (35% width)"
  - "DAG progress pane in right-bottom (30% height) with text-based progress bar"
  - "Debounce viewport updates with 50ms tick to prevent render thrashing"
  - "Auto-scroll viewport to bottom on new output for selected agent"

patterns-established:
  - "SubscribeAll pattern for TUI consuming all event topics on single channel"
  - "waitForEvent cmd pattern to recursively listen for next event"
  - "Focus state propagation via SetFocused methods on child panes"
  - "Layout computation in computeLayout method, dimensions passed via SetSize"

# Metrics
duration: 230s
completed: 2026-02-10
---

# Phase 04 Plan 02: TUI Display Summary

**Bubble Tea split-pane TUI with agent list, scrollable output viewport, DAG progress, and vim-style keyboard navigation consuming events in real-time**

## Performance

- **Duration:** 3 min 50 sec
- **Started:** 2026-02-10T18:41:34Z
- **Completed:** 2026-02-10T18:45:24Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Split-pane TUI layout with agent list (left), agent output viewport (embedded), and DAG progress (right-bottom)
- Real-time event consumption from event bus using SubscribeAll pattern
- Vim-style keyboard navigation (Tab/Shift+Tab/1/2/3, j/k for scrolling)
- Demo entry point that simulates orchestrator run with fake events
- Status indicators for agents (running/completed/failed) with color-coded display
- Text-based DAG progress bar with real-time task counts

## Task Commits

Each task was committed atomically:

1. **Task 1: TUI root model, styles, keybindings, and pane models** - `63f1076` (feat)
2. **Task 2: Main entry point and manual smoke test** - `08a6ca9` (feat)

## Files Created/Modified
- `internal/tui/model.go` - Root Bubble Tea model with split-pane layout and event routing
- `internal/tui/agent_pane.go` - Agent list and output viewport pane with debounced updates
- `internal/tui/dag_pane.go` - DAG progress pane with status counts and progress bar
- `internal/tui/styles.go` - Lipgloss style definitions for borders and status indicators
- `internal/tui/keys.go` - Keybinding constants and help view
- `cmd/orchestrator/main.go` - Application entry point with event bus and fake event demo

## Decisions Made

**1. Use stable bubbletea v1.x instead of v2 beta**
Initially attempted v2 (charm.land/bubbletea/v2) but encountered version compatibility issues with lipgloss v2 beta. Switched to stable v1.x releases for production reliability.

**2. Combine agent list and output in single left pane**
Rather than separate panes, combined agent list (narrow column) and output viewport (wide column) horizontally within the left pane. Simpler layout and better use of space.

**3. Debounce viewport updates with 50ms tick**
High-frequency TaskOutputEvent publishing could cause render thrashing. Implemented debouncing with updateTag counter and tea.Tick to batch viewport updates.

**4. Auto-scroll viewport to bottom**
New output automatically scrolls to bottom for better UX when following real-time task execution. User can still scroll up manually.

## Deviations from Plan

**Auto-fixed Issue: Dependency version compatibility**

**[Rule 3 - Blocking] Switched from v2 beta to v1 stable releases**
- **Found during:** Task 1 (Initial TUI compilation)
- **Issue:** Bubble Tea v2 (charm.land/bubbletea/v2) and Lipgloss v2 (github.com/charmbracelet/lipgloss/v2) beta versions had API compatibility issues with underlying x/ansi dependencies. Build failed with method signature mismatches.
- **Fix:** Ran `go get github.com/charmbracelet/bubbletea@latest` to install stable v1.3, updated import paths from `charm.land/bubbletea/v2` to `github.com/charmbracelet/bubbletea`, and from `lipgloss/v2` to `lipgloss` throughout all TUI files.
- **Files modified:** internal/tui/model.go, internal/tui/agent_pane.go, internal/tui/dag_pane.go, internal/tui/styles.go
- **Verification:** `go build ./internal/tui/...` succeeded, all tests passed with -race flag
- **Committed in:** 63f1076 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking dependency issue)
**Impact on plan:** Necessary fix to unblock compilation. Stable v1.x is more appropriate for production than beta v2. No functional scope changes.

## Issues Encountered

None - aside from the dependency version compatibility issue (documented in Deviations).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- TUI foundation complete with event consumption and display working
- Ready for Phase 04 Plan 03 (Interactive controls and settings panel)
- Placeholder for settings panel (key 's') already reserved in help text
- Event routing infrastructure ready for user input commands

## Self-Check

All created files exist:
- internal/tui/model.go
- internal/tui/agent_pane.go
- internal/tui/dag_pane.go
- internal/tui/styles.go
- internal/tui/keys.go
- cmd/orchestrator/main.go

All commits exist:
- 63f1076 (Task 1)
- 08a6ca9 (Task 2)

All tests pass with -race flag:
```
ok  	github.com/aristath/orchestrator/internal/backend	(cached)
ok  	github.com/aristath/orchestrator/internal/config	(cached)
ok  	github.com/aristath/orchestrator/internal/events	(cached)
ok  	github.com/aristath/orchestrator/internal/orchestrator	(cached)
ok  	github.com/aristath/orchestrator/internal/scheduler	(cached)
ok  	github.com/aristath/orchestrator/internal/worktree	(cached)
```

Binary builds successfully:
```
-rwxr-xr-x  1 aristath  staff  4.0M /tmp/orchestrator-demo
```

**Self-Check: PASSED**

---
*Phase: 04-event-bus-tui-integration*
*Completed: 2026-02-10*
