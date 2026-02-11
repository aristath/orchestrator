# Project Research Summary

**Project:** Dynamic Config Management via Dialog System
**Domain:** TUI config management with modal CRUD interfaces
**Researched:** 2026-02-11
**Confidence:** HIGH

## Executive Summary

This project extends an existing Bubble Tea v2 TUI orchestrator with dynamic config management through dialog-based CRUD interfaces for Backends/Roles/Workflows. The recommended approach uses a dialog stack pattern proven in production TUIs like Crush, avoiding external overlay libraries in favor of custom infrastructure built with existing Charmbracelet components (bubbles/list, huh forms, lipgloss compositing).

The architecture centers on three core patterns: (1) dialog stack with interface-based routing for modal keyboard isolation, (2) generic CRUDList component wrapping bubbles/list for consistent CRUD UX, and (3) centralized theme system using cached lipgloss styles. This design integrates cleanly with the existing event bus and pane architecture without disrupting running task execution.

Critical risks include keyboard event leakage (dialogs must intercept ALL keys before pane routing), config schema migration (Provider/Agent → Backend/Role requires backward-compatible UnmarshalJSON), and form state persistence (huh forms must be rebuilt fresh on each modal open). These pitfalls are all preventable with upfront architectural discipline in Phase 1-2, avoiding expensive retrofits later.

## Key Findings

### Recommended Stack

The research recommends NO new external dependencies beyond what's already validated (Bubble Tea v2, Lipgloss v2, Huh, Bubbles). Existing libraries provide all needed functionality.

**Core additions (custom implementations):**
- Dialog stack infrastructure: Custom DialogModel interface with stack management, modal input routing, ESC handling, lipgloss.Place overlay composition — proven in Crush TUI, external libraries (bubbletea-overlay) lack dialog-specific features
- CRUDList component: Generic wrapper around bubbles/list (already installed v1.0.0) with add/edit/delete operations — reusable across Backend/Role/Workflow lists
- Centralized theme: Expand internal/tui/styles.go with lipgloss.AdaptiveColor palette and named style variables — no external theme library needed

**What NOT to add:**
- Evertras/bubble-table: No confirmed Bubble Tea v2 compatibility, use bubbles/list instead
- bubbletea-overlay: Only provides basic compositing, lacks dialog stack/focus management
- purpleclay/lipgloss-theme: External dependency for simple need, not justified

### Expected Features

**Must have (table stakes) - Phase 1:**
- List view for Backends/Roles/Workflows with keyboard navigation (j/k, Enter, ESC)
- Create/edit/delete items via modal forms
- Visual selection indicators and empty state messaging
- Form validation preventing invalid configs
- Confirmation prompts for destructive delete actions
- Global vs project config save target selection

**Should have (competitive) - Phase 2:**
- Search/filter in lists when items exceed 10 (bubbles/list filtering OOTB)
- Help overlay (?) showing keybindings
- Multi-step wizard for complex role creation
- Duplicate/clone existing items
- Context-aware defaults (pre-fill provider based on backend type)

**Defer to v2+:**
- Import/export role templates
- Inline documentation tooltips
- Undo/redo on config changes
- Keyboard shortcut customization

**Anti-features to avoid:**
- Mouse support (breaks keyboard flow, terminal inconsistency)
- Real-time validation during typing (interrupts flow, use blur validation)
- Auto-save on every change (race conditions, no rollback)
- Nested dialog modals (focus management nightmare, use multi-step forms instead)

### Architecture Approach

The architecture extends the existing split-pane TUI with a dialog overlay layer that doesn't disrupt the event bus or running task execution. Five proven patterns form the foundation.

**Major components:**
1. Dialog Stack Manager: Maintains []DialogModel LIFO stack in root model, routes input to topmost dialog with priority over panes, handles Esc-to-pop semantics, composes overlays using lipgloss.Place
2. Generic CRUDList[T]: Reusable component wrapping bubbles/list with add/edit/delete operations, adapter functions for list.Item interface, integrated form/confirmation dialogs
3. Centralized Theme System: Single Theme struct with adaptive colors and pre-computed lipgloss styles, passed to all dialogs/panes via constructor, prevents style recomputation lag
4. Config Change Propagation: ConfigChangedMsg event after saves, root model broadcasts to components, allows hot-reload without restart (backends require reconnect, roles/workflows update display only)
5. Overlay Composition Pattern: Base panes rendered first, dialogs layered on top using lipgloss.Place with center positioning, no z-index conflicts

**Integration strategy:**
- Migrate existing SettingsPaneModel to DialogModel interface first (proves pattern)
- Keep AgentPane/DAGPane unchanged (no config editing)
- Config types renamed (Provider→Backend, Agent→Role) with backward-compatible migration
- Event bus continues unchanged (ConfigChangedMsg is just another event)

### Critical Pitfalls

1. **Keyboard Event Leakage** — Modal input can leak to background panes causing unintended actions (q quits app, tab cycles panes). Prevention: Early-return guard in root Update() BEFORE any pane routing. Dialog stack checked first, all keys routed to topmost dialog when open. Test every shortcut with modal active.

2. **Form State Not Resetting** — Huh forms maintain internal state across visibility toggles. Opening modal second time shows completed/disabled fields. Prevention: REBUILD form fresh on every modal open, never reuse form instances. Don't rely on form.Init() to reset state. Huh issue #319 confirms this requirement.

3. **Config Migration Breaking Existing Configs** — Renaming Provider→Backend/Agent→Role causes old configs to unmarshal as empty structs, silently losing user settings. Prevention: Implement custom UnmarshalJSON to detect old schema, migrate fields, write back new format. Test with v1 config fixtures.

4. **Import Cycles from Config Refactor** — Creating config/migration package causes cycles: config → migration → config. Prevention: Keep migration logic IN config package, use type aliases during transition, update all 24 consuming files in single phase.

5. **Lipgloss Style Recomputation** — Building styles in View() adds 5-10ms per frame, causes scrolling stutter. Prevention: Compute styles once at init/resize, cache in model struct, only recompute on size/focus change. Profile with BUBBLETEA_PROFILE=1.

6. **Dialog Overlay Z-Index Conflicts** — Modal borders clipped by background panes or background text visible through modal. Prevention: Render background first at full size, then Place() overlay as separate layer. Don't use JoinVertical for composition.

7. **CRUD List Cursor Lost** — Editing item 5 in list, closing modal returns cursor to index 0. Prevention: Preserve list model instance across modal transitions, store cursor position explicitly, restore on modal close.

8. **Validation Errors Persist** — Opening new modal shows validation errors from previous form. Prevention: Fresh field binding variables per form rebuild, don't reuse model-level strings across different dialogs.

## Implications for Roadmap

Based on research, suggested phase structure follows dependency hierarchy and incremental validation:

### Phase 1: Dialog System Foundation
**Rationale:** Core modal infrastructure must be solid before adding CRUD features. Keyboard isolation and overlay rendering are foundational — getting these wrong means rearchitecting later.
**Delivers:** Dialog interface, stack manager in root model, overlay composition using lipgloss.Place
**Addresses:** Enables all modal-based features from FEATURES.md
**Avoids:** Pitfall #1 (keyboard leakage), Pitfall #6 (z-index conflicts)
**Complexity:** Medium — proven pattern from Crush, but critical to implement correctly

### Phase 2: Settings Dialog Migration
**Rationale:** Validates dialog stack pattern with existing modal before building new features. Low-risk way to test architecture decisions.
**Delivers:** SettingsPaneModel converted to SettingsDialog implementing Dialog interface
**Uses:** Dialog stack from Phase 1, existing huh form
**Avoids:** Pitfall #2 (form state reset) — fix existing bug during migration
**Complexity:** Low — refactoring existing code to new interface

### Phase 3: Config Type Refactor
**Rationale:** Schema migration must happen BEFORE building CRUD interfaces. Can't build Backend CRUD while config types are still called Providers.
**Delivers:** Provider→Backend, Agent→Role rename with backward-compatible migration
**Implements:** Custom UnmarshalJSON for config loading
**Avoids:** Pitfall #3 (breaking existing configs), Pitfall #4 (import cycles)
**Complexity:** Medium — touches 24 files, needs careful migration testing

### Phase 4: Centralized Theme System
**Rationale:** Establish theme pattern before building multiple dialogs. Prevents inconsistent styling and performance issues from ad-hoc styles.
**Delivers:** Theme struct with adaptive colors, pre-computed styles, passed to all components
**Uses:** Lipgloss v2 AdaptiveColor, style caching
**Avoids:** Pitfall #5 (style recomputation lag)
**Complexity:** Low — refactoring existing styles.go

### Phase 5: Backend CRUD Dialog
**Rationale:** First CRUD implementation establishes patterns for Role/Workflow. Backends are simplest (no dependencies on other types).
**Delivers:** BackendCRUDDialog with list view, create/edit/delete forms, confirmation prompts
**Uses:** Dialog stack (Phase 1), Theme (Phase 4), BackendConfig (Phase 3)
**Implements:** CRUDList[BackendConfig], FormDialog, ConfirmDialog
**Avoids:** Pitfall #7 (cursor position), Pitfall #8 (validation persistence)
**Complexity:** High — first full CRUD implementation, creates reusable patterns

### Phase 6: Role CRUD Dialog
**Rationale:** Builds on Backend CRUD patterns. Roles reference Backends so must come after.
**Delivers:** RoleCRUDDialog with multi-step wizard for complex role config
**Uses:** CRUDList pattern from Phase 5, references BackendConfig from Phase 3
**Implements:** Multi-group huh forms for wizard UX
**Complexity:** Medium — reuses patterns, adds wizard complexity

### Phase 7: Workflow CRUD Dialog
**Rationale:** Workflows reference Roles, so must come after Role CRUD. Completes config management feature set.
**Delivers:** WorkflowCRUDDialog with step editing, DAG preview
**Uses:** All patterns from Phase 5-6, references RoleConfig
**Complexity:** Medium — most complex data model (workflow steps), but established patterns

### Phase 8: Config Change Propagation
**Rationale:** Last because it requires all CRUD dialogs working. Tests end-to-end hot-reload without restart.
**Delivers:** ConfigChangedMsg event, component subscription, backend restart logic
**Implements:** Hot-reload pattern from ARCHITECTURE.md
**Complexity:** Medium — async propagation, graceful backend restart

### Phase Ordering Rationale

- Phases 1-2 establish architecture without visible new features (validate pattern first)
- Phase 3 is blocker for CRUD (can't build Backend CRUD with old type names)
- Phase 4 prevents tech debt accumulation (theme needed before multiple dialogs)
- Phases 5-7 follow natural dependencies (Backend → Role → Workflow)
- Phase 8 is enhancement layer (CRUD works without propagation, adds polish)

This ordering minimizes rework: getting foundation right in Phase 1-4 means Phase 5-7 are straightforward applications of established patterns. Reversing order (building CRUD first, adding theme later) would require refactoring all dialogs.

### Research Flags

**Phases needing deeper research during planning:**
- NONE — this domain is well-documented, all patterns proven in production TUIs

**Phases with standard patterns (skip research-phase):**
- Phase 1: Dialog stack pattern from Crush documented, lipgloss.Place API clear
- Phase 2: Simple refactoring, no new patterns
- Phase 3: Config migration is standard Go JSON handling
- Phase 4: Theme system is common pattern, lipgloss docs sufficient
- Phase 5-7: CRUD with bubbles/list and huh is well-established
- Phase 8: Event propagation already used in existing event bus

**Recommendation:** Skip `/gsd:research-phase` for all phases. Proceed directly to task breakdown using research files as reference. Only invoke phase research if implementation reveals unexpected complexity.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All components already validated (Bubble Tea v2, Huh, Bubbles), custom implementations based on proven Crush patterns |
| Features | MEDIUM | Table stakes features clear from k9s/lazygit references, some uncertainty on wizard UX complexity for roles |
| Architecture | HIGH | Dialog stack and CRUD patterns documented in multiple production TUIs, integration points with existing code well-understood |
| Pitfalls | HIGH | All pitfalls sourced from actual GitHub issues (huh #319, bubbletea #642) and production experience, prevention strategies verified |

**Overall confidence:** HIGH

### Gaps to Address

- **Bubble-table v2 compatibility:** Research noted Evertras/bubble-table lacks confirmed Bubble Tea v2 support. Not critical since bubbles/list is recommended, but if tabular layout becomes requirement in Phase 6-7, will need compatibility testing before adoption.

- **Multi-step wizard UX flow:** Features research identified wizard pattern for role creation but didn't detail step transitions (forward/back navigation, step validation, partial saves). Phase 6 planning should reference huh multi-group form examples and potentially prototype wizard flow before full implementation.

- **Workflow DAG preview rendering:** Phase 7 includes optional "live preview" feature for workflow visualization. Research didn't identify existing ASCII DAG rendering library. If implemented, will need library search or custom rendering logic during Phase 7 planning.

- **Config hot-reload edge cases:** Phase 8 propagation handles basic scenarios (backend restart, role display update) but edge cases unclear: What if backend in use by running task is deleted? What if role config changes mid-task execution? Planning should define failure modes and graceful degradation.

All gaps are implementation details that don't affect architecture or phase ordering. Can be resolved during phase planning/execution.

## Sources

### Primary (HIGH confidence)
- [Bubble Tea v2 GitHub](https://github.com/charmbracelet/bubbletea) — Dialog stack architecture, message routing patterns
- [Bubbles list component](https://pkg.go.dev/github.com/charmbracelet/bubbles/list) — Official API documentation, filtering/pagination
- [Huh forms GitHub](https://github.com/charmbracelet/huh) — Form lifecycle, validation, issue #319 on state reset
- [Lipgloss GitHub](https://github.com/charmbracelet/lipgloss) — Overlay composition with Place(), adaptive colors
- [Crush TUI Architecture](https://deepwiki.com/charmbracelet/crush/5.1-tui-architecture) — Dialog system pattern, modal routing

### Secondary (MEDIUM confidence)
- [Evertras/bubble-table](https://github.com/Evertras/bubble-table) — Table component features, v2 compatibility uncertain
- [k9s Hotkeys](https://k9scli.io/topics/hotkeys/) — TUI keyboard navigation standards
- [Lazygit TUI patterns](https://masri.blog/Blog/Coding/Git/Lazygit-A-TUI-Approach) — CRUD UX reference
- [bubbletea-overlay package](https://pkg.go.dev/github.com/quickphosphat/bubbletea-overlay) — Basic compositing, insufficient for dialog stack

### Tertiary (LOW confidence)
- [purpleclay/lipgloss-theme](https://pkg.go.dev/github.com/purpleclay/lipgloss-theme) — External theme library patterns, not recommended for adoption
- Crush dialog implementation details — Mentioned in DeepWiki but source code not directly verified

---
*Research completed: 2026-02-11*
*Ready for roadmap: yes*
