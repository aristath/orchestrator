# Requirements: Orchestrator v1.1

**Defined:** 2026-02-11
**Core Value:** A single developer plans a task with an AI, then that AI autonomously decomposes and executes the plan across multiple specialized agents running in parallel.

## v1.1 Requirements

Requirements for Settings & Configuration Refactor. Each maps to roadmap phases.

### Config Model

- [ ] **CONF-01**: User can define backends as config objects with name, command, args, and type fields
- [ ] **CONF-02**: User can define roles as config objects with name, backend reference, model, system prompt, and tools
- [ ] **CONF-03**: User can define workflows as named pipelines of role references
- [ ] **CONF-04**: Config loads from JSON with two-tier merge (global + project)
- [ ] **CONF-05**: Config validates referential integrity (roles reference existing backends, workflows reference existing roles)
- [ ] **CONF-06**: Config CRUD methods exist for add, update, and delete operations on all three types
- [ ] **CONF-07**: Deleting a backend that is referenced by a role produces an error

### Dialog System

- [ ] **DLGS-01**: User can open modal dialogs that overlay the main TUI panes
- [ ] **DLGS-02**: Modal dialogs capture all keyboard input (no events leak to background panes)
- [ ] **DLGS-03**: User can close any dialog with ESC
- [ ] **DLGS-04**: Dialog stack supports multiple overlapping dialogs (e.g., CRUD list opens edit form)
- [ ] **DLGS-05**: Confirmation dialog appears before destructive actions (delete)
- [ ] **DLGS-06**: Dialog renders centered with correct borders at any terminal size (80x24 to 200x60)

### Theme System

- [ ] **THEM-01**: All TUI components use styles from a centralized theme
- [ ] **THEM-02**: Theme uses lipgloss AdaptiveColor for light/dark terminal support
- [ ] **THEM-03**: Styles are cached and recomputed only on size/focus changes (not every render)
- [ ] **THEM-04**: Existing panes (agent, DAG) use theme styles instead of inline lipgloss definitions

### Backend CRUD

- [ ] **BCRD-01**: User can view a list of all configured backends
- [ ] **BCRD-02**: User can create a new backend via form dialog
- [ ] **BCRD-03**: User can edit an existing backend via form dialog
- [ ] **BCRD-04**: User can delete a backend (with confirmation)
- [ ] **BCRD-05**: Backend list supports keyboard navigation (j/k, Enter, ESC)
- [ ] **BCRD-06**: Backend list supports search/filter

### Role CRUD

- [ ] **RCRD-01**: User can view a list of all configured roles
- [ ] **RCRD-02**: User can create a new role via form dialog (selecting backend, model, writing system prompt)
- [ ] **RCRD-03**: User can edit an existing role via form dialog
- [ ] **RCRD-04**: User can delete a role (with confirmation)
- [ ] **RCRD-05**: Role list supports keyboard navigation (j/k, Enter, ESC)
- [ ] **RCRD-06**: Role list supports search/filter

### Workflow CRUD

- [ ] **WCRD-01**: User can view a list of all configured workflows
- [ ] **WCRD-02**: User can create a new workflow by selecting roles for each step
- [ ] **WCRD-03**: User can edit an existing workflow
- [ ] **WCRD-04**: User can delete a workflow (with confirmation)
- [ ] **WCRD-05**: Workflow list supports keyboard navigation (j/k, Enter, ESC)
- [ ] **WCRD-06**: Workflow list supports search/filter

### Settings Integration

- [ ] **SETT-01**: User can choose to save config to global or project scope
- [ ] **SETT-02**: User can access settings via keyboard shortcut from main TUI
- [ ] **SETT-03**: Help overlay (?) shows available keybindings in current context
- [ ] **SETT-04**: Runner and scheduler consume new config types (backends/roles replace providers/agents)

## v2 Requirements

Deferred to future release.

### Config Management

- **CFGM-01**: Config changes hot-reload without app restart (ConfigChangedMsg propagation)
- **CFGM-02**: Import/export role templates as shareable JSON files
- **CFGM-03**: Undo/redo on config changes
- **CFGM-04**: Keyboard shortcut customization

### Workflow Enhancements

- **WKFL-01**: Live preview of workflow pipeline as ASCII diagram
- **WKFL-02**: Workflow versioning with history

## Out of Scope

| Feature | Reason |
|---------|--------|
| Mouse support for dialogs | Breaks keyboard-first TUI paradigm, accessibility issues |
| Real-time validation during typing | Interrupts flow, premature errors — validate on submit only |
| Drag-and-drop list reordering | Mouse dependency, complex state — use keyboard shortcuts instead |
| Auto-save on every change | Race conditions, unclear when config applies — explicit save only |
| Config schema migration from v1.0 | User explicitly chose no migration; users reconfigure manually |
| Nested dialog modals beyond 2 levels | Confusing UX, ESC ambiguity — keep stack shallow |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| (populated by roadmapper) | | |

**Coverage:**
- v1.1 requirements: 31 total
- Mapped to phases: 0
- Unmapped: 31 ⚠️

---
*Requirements defined: 2026-02-11*
*Last updated: 2026-02-11 after initial definition*
