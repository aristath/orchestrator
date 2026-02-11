# Stack Research: Dialog Overlay + CRUD for Config Management

**Domain:** Dialog-based TUI config management system
**Researched:** 2026-02-11
**Confidence:** MEDIUM-HIGH

## Context: Existing Stack (DO NOT re-add)

**Already validated and in use:**
- Bubble Tea v2 (charm.land/bubbletea/v2@v2.0.0-rc.2)
- Lipgloss v2 (charm.land/lipgloss/v2@v2.0.0-beta1)
- Huh forms (github.com/charmbracelet/huh@v0.8.0)
- Bubbles v1 (github.com/charmbracelet/bubbles@v1.0.0)
- modernc.org/sqlite (persistence)
- cenkalti/backoff, sony/gobreaker (resilience)

**This research focuses ONLY on additions for:** Dialog overlay system, list/table components for CRUD operations, reusable theme system, dynamic config management patterns.

---

## Recommended Stack Additions

### Dialog Overlay System

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **Custom implementation** | N/A | Modal dialog infrastructure with message routing | Crush uses custom `dialogs.DialogCmp` with dialog stack (`[]DialogModel`). External libraries (quickphosphat/bubbletea-overlay) provide basic compositing but lack dialog-specific features (stack management, focus capture, ESC handling). Build custom: (1) Dialog stack field in root model, (2) DialogModel interface with Init/Update/View, (3) Priority input routing (dialogs intercept before panes), (4) lipgloss.Layer for rendering overlay, (5) Position helpers (Center, etc.). Pattern proven in Crush. |

### List/Table Components for CRUD

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **github.com/charmbracelet/bubbles/list** | v1.0.0 (already installed) | List component with filtering, pagination, selection | Official Charmbracelet component. Features: fuzzy filtering (sahilm/fuzzy), pagination, custom ItemDelegate for rendering, runtime delegate swapping via SetDelegate(). Handles CRUD list view (show items, select for edit/delete). Supports custom Item interface (FilterValue() method) and DefaultItem (Title()/Description()). Compatible with Bubble Tea v2. **Use this for Backend/Role/Workflow lists.** |
| **github.com/Evertras/bubble-table** | v0.19.2 | Table component with sorting, filtering, row selection | Community table component if tabular data needed. Features: column sorting, row selection, pagination, hierarchical styling (global → column → row → cell), hidden metadata (RowData) for CRUD operations. **Caveat:** No explicit Bubble Tea v2 compatibility confirmed in release notes (latest 2024-09-06). Verify compatibility before use. **Only add if table layout preferred over list.** |

**Recommendation:** Start with `bubbles/list` (already installed, officially supported). Only add `bubble-table` if tabular display becomes requirement.

### Theme/Style System

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **Centralized style package** | N/A | Reusable lipgloss styles with adaptive colors | Current `internal/tui/styles.go` has ad-hoc styles. Expand to centralized theme: (1) Define color palette with lipgloss.AdaptiveColor for light/dark terminals, (2) Create named style variables (DialogBorder, ListSelected, ListNormal, etc.), (3) Export from `tui.Styles` struct, (4) Pass to components via constructor. Pattern: Crush uses this, purpleclay/lipgloss-theme demonstrates similar approach. **Do NOT add external theme library** (adds dependency for simple need). |
| **github.com/purpleclay/lipgloss-theme** | Latest (optional) | Pre-built theme system with adaptive colors, styled glyphs | ONLY if standardized color scheme across PurpleClay TUI ecosystem is desired. Provides: 11 purple shades (S950-S50), accent colors (green/amber/red), styled text (H1-H6, Mark, Tick/Cross/Bang glyphs), table helpers. **Trade-off:** External dependency for styling logic. **Skip unless theme consistency with other tools is priority.** |

**Recommendation:** Expand `internal/tui/styles.go` with centralized theme. Skip external library.

### Config Management Patterns

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **In-memory config model** | N/A | Runtime config state for Backends/Roles/Workflows | Current: Config loaded from JSON, stored in `config.OrchestratorConfig`. Extend with: (1) CRUD methods (AddBackend, DeleteRole, UpdateWorkflow), (2) Validation (e.g., ensure backend referenced by role exists), (3) Dirty flag tracking for unsaved changes, (4) Save to global vs project path selection. All config mutations happen in-memory, persist on save. |
| **Existing config.Save()** | N/A (stdlib encoding/json) | Persist config to JSON | Already implemented in `internal/config/save.go`. Uses `json.MarshalIndent` for human-readable JSON. **No changes needed.** |
| **Dialog-driven CRUD flow** | N/A | User interaction pattern | List view (bubbles/list) → Select item → Open dialog (huh form or custom) → Edit/Delete → Update in-memory config → Save button triggers config.Save(). Dialogs modal (block background interaction). Pattern: Settings already uses huh form in modal, extend for multi-item CRUD. |

**Recommendation:** Extend existing `internal/config` with CRUD methods. Use bubbles/list for item display, huh forms in dialogs for editing.

---

## Installation (NEW dependencies only)

```bash
# List component (already installed via bubbles@v1.0.0)
# No new installation needed

# Optional: Table component (only if required)
go get github.com/Evertras/bubble-table@v0.19.2

# Optional: External theme library (not recommended)
go get github.com/purpleclay/lipgloss-theme@latest
```

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative | Confidence |
|-------------|-------------|-------------------------|------------|
| **Custom dialog infrastructure** | quickphosphat/bubbletea-overlay | Simple overlay compositing without dialog stack, focus management, or input routing. Use if only single modal needed and don't need stack/ESC handling. | MEDIUM |
| **Custom dialog infrastructure** | jsdoublel/bubbletea-overlay (fork) | Exposes internal `composite()` function for more control. Use if need custom compositing logic beyond positioning. | LOW (fork maintenance unclear) |
| **bubbles/list** | Evertras/bubble-table | Tabular data with multi-column sorting/filtering. Use if columns > 2 and sorting critical. Verify Bubble Tea v2 compatibility first. | MEDIUM |
| **Centralized styles in tui package** | purpleclay/lipgloss-theme | Standardized purple-based theme with pre-built glyphs. Use if visual consistency with other PurpleClay TUI tools required. | LOW (adds dependency for simple need) |
| **In-memory config with CRUD methods** | Direct file editing (viper, koanf) | Real-time config reload, complex config hierarchies, env var overrides. Overkill for this use case (simple JSON, TUI-driven edits). | HIGH |

---

## What NOT to Use

| Avoid | Why | Use Instead | Confidence |
|-------|-----|-------------|------------|
| **bubbletea-overlay libraries** | Provide only basic compositing (foreground on background). Lack dialog-specific features: stack management, modal input routing, ESC key handling, dialog lifecycle. | Custom dialog infrastructure (stack + DialogModel interface + input routing) | HIGH |
| **Evertras/bubble-table (initially)** | No confirmed Bubble Tea v2 compatibility. Latest release 2024-09-06 predates v2 stable. Risk of API incompatibility with v2 message types (KeyPressMsg vs KeyMsg). | bubbles/list (official, v2 compatible) unless tabular layout proven necessary | MEDIUM |
| **Config libraries (viper, koanf)** | Designed for complex app config (env vars, remote config, watch mode). Adds unnecessary complexity for TUI-driven JSON editing. | Extend existing config.Save() with in-memory CRUD methods | HIGH |
| **Direct lipgloss.Layer for all overlays** | Low-level positioning primitive. Requires manual offset calculation, no dialog semantics. Tedious for multiple dialogs. | Custom dialog infrastructure wrapping lipgloss.Layer with position helpers | MEDIUM |

---

## Stack Patterns for Dialog CRUD System

### Pattern 1: Dialog Stack Management

```go
// In root TUI model
type Model struct {
    // ... existing fields
    dialogStack []DialogModel  // LIFO stack
}

type DialogModel interface {
    tea.Model
    ShouldClose() bool  // Return true when dialog completes
}

// Update() method
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Priority: Dialog stack first
    if len(m.dialogStack) > 0 {
        top := len(m.dialogStack) - 1
        dialog, cmd := m.dialogStack[top].Update(msg)
        m.dialogStack[top] = dialog.(DialogModel)

        if m.dialogStack[top].ShouldClose() {
            m.dialogStack = m.dialogStack[:top]  // Pop
        }
        return m, cmd
    }

    // Normal pane handling...
}

// View() method
func (m Model) View() string {
    base := m.renderPanes()

    if len(m.dialogStack) > 0 {
        top := m.dialogStack[len(m.dialogStack)-1]
        return lipgloss.Layer(base, top.View(),
            lipgloss.Center, lipgloss.Center, 0, 0)
    }

    return base
}
```

### Pattern 2: List-Driven CRUD with Dialogs

```go
// Backend list component
type BackendListModel struct {
    list     list.Model
    config   *config.OrchestratorConfig
}

// Items implement list.Item interface
type BackendItem struct {
    backend config.Backend
}

func (i BackendItem) FilterValue() string { return i.backend.Name }
func (i BackendItem) Title() string       { return i.backend.Name }
func (i BackendItem) Description() string {
    return fmt.Sprintf("%s - %s", i.backend.Provider, i.backend.Model)
}

// Update handles selection
func (m BackendListModel) Update(msg tea.Msg) (BackendListModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "enter":
            // Open edit dialog for selected item
            selected := m.list.SelectedItem().(BackendItem)
            return m, openEditBackendDialog(selected.backend)

        case "d":
            // Open delete confirmation dialog
            selected := m.list.SelectedItem().(BackendItem)
            return m, openDeleteConfirmDialog(selected.backend.Name)

        case "n":
            // Open new backend dialog
            return m, openNewBackendDialog()
        }
    }

    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}
```

### Pattern 3: Centralized Theme System

```go
// internal/tui/theme.go
package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
    // Colors (adaptive for light/dark terminals)
    Primary   lipgloss.AdaptiveColor
    Secondary lipgloss.AdaptiveColor
    Success   lipgloss.AdaptiveColor
    Warning   lipgloss.AdaptiveColor
    Error     lipgloss.AdaptiveColor
    Muted     lipgloss.AdaptiveColor

    // Styles
    DialogBorder     lipgloss.Style
    DialogTitle      lipgloss.Style
    ListSelected     lipgloss.Style
    ListNormal       lipgloss.Style
    ButtonFocused    lipgloss.Style
    ButtonBlurred    lipgloss.Style
}

var DefaultTheme = Theme{
    Primary:   lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7D79F6"},
    Secondary: lipgloss.AdaptiveColor{Light: "#6C757D", Dark: "#ADB5BD"},
    Success:   lipgloss.AdaptiveColor{Light: "#198754", Dark: "#75B798"},
    Warning:   lipgloss.AdaptiveColor{Light: "#FFC107", Dark: "#FFD666"},
    Error:     lipgloss.AdaptiveColor{Light: "#DC3545", Dark: "#F28B82"},
    Muted:     lipgloss.AdaptiveColor{Light: "#6C757D", Dark: "#495057"},

    DialogBorder: lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7D79F6"}).
        Padding(1, 2),

    // ... other styles
}

// Pass theme to components
func NewBackendListModel(config *config.OrchestratorConfig, theme Theme) BackendListModel {
    delegate := list.NewDefaultDelegate()
    delegate.Styles.SelectedTitle = theme.ListSelected
    delegate.Styles.NormalTitle = theme.ListNormal
    // ...
}
```

### Pattern 4: Config CRUD Methods

```go
// internal/config/crud.go
package config

import "errors"

// AddBackend adds a new backend, returns error if name exists
func (c *OrchestratorConfig) AddBackend(name string, backend Backend) error {
    if _, exists := c.Backends[name]; exists {
        return errors.New("backend already exists")
    }
    c.Backends[name] = backend
    return nil
}

// UpdateBackend updates existing backend
func (c *OrchestratorConfig) UpdateBackend(name string, backend Backend) error {
    if _, exists := c.Backends[name]; !exists {
        return errors.New("backend not found")
    }
    c.Backends[name] = backend
    return nil
}

// DeleteBackend removes backend, validates no roles reference it
func (c *OrchestratorConfig) DeleteBackend(name string) error {
    // Check if any role uses this backend
    for roleName, role := range c.Roles {
        if role.Backend == name {
            return fmt.Errorf("backend in use by role: %s", roleName)
        }
    }
    delete(c.Backends, name)
    return nil
}

// Similar for Roles, Workflows...
```

---

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| **bubbles/list v1.0.0** | Bubble Tea v2.0.0-rc.2 | Official Charmbracelet library, designed for v2. Uses standard tea.Model interface. |
| **Evertras/bubble-table v0.19.2** | Bubble Tea v1 (unconfirmed for v2) | Latest release Sep 2024, predates v2 stable. No v2 mention in release notes. Test before adopting. |
| **lipgloss v2.0.0-beta1** | Bubble Tea v2.0.0-rc.2 | Already validated compatible (in use). |
| **huh v0.8.0** | Bubble Tea v2.0.0-rc.2 | Already validated compatible (in use for settings). |

---

## Architecture Integration

### Current State
- Settings pane: `SettingsPaneModel` with huh form, modal overlay (blocks input)
- Hardcoded config: Provider commands, agent provider/model
- Simple show/hide visibility toggle

### Target State (Dialog + CRUD)
1. **Dialog system:** Stack-based (multiple dialogs possible), modal input routing, ESC/completion handling
2. **List views:** Backends list, Roles list, Workflows list (bubbles/list component)
3. **CRUD dialogs:** Edit/Delete/New dialogs using huh forms
4. **Theme system:** Centralized `Theme` struct, passed to all components
5. **Config model:** CRUD methods (Add/Update/Delete), validation, dirty tracking

### Migration Path
1. Extract dialog logic from `SettingsPaneModel` into generic `DialogStack` manager
2. Create `BackendListModel`, `RoleListModel`, `WorkflowListModel` using bubbles/list
3. Build dialog components for each CRUD operation (EditBackendDialog, NewRoleDialog, etc.)
4. Expand `internal/tui/styles.go` into `internal/tui/theme.go` with adaptive colors
5. Add CRUD methods to `internal/config/types.go` with validation

---

## Confidence Assessment

| Area | Confidence | Rationale |
|------|------------|-----------|
| **bubbles/list for CRUD lists** | HIGH | Official component, v2 compatible, already installed, proven in ecosystem |
| **Custom dialog infrastructure** | HIGH | Crush pattern proven, simple stack + interface approach, avoids external deps |
| **Centralized theme system** | HIGH | Standard Go pattern, lipgloss.AdaptiveColor documented, Crush uses similar |
| **Config CRUD methods** | HIGH | Straightforward extension of existing config package, stdlib only |
| **bubble-table compatibility** | LOW | No v2 confirmation in releases, message type changes in v2 could break |
| **bubbletea-overlay libraries** | MEDIUM | Work for basic compositing, but lack dialog semantics needed for stack/routing |
| **purpleclay/lipgloss-theme necessity** | LOW | External dependency not justified for simple theme needs |

---

## Open Questions & Validation Needed

| Question | Priority | How to Resolve |
|----------|----------|----------------|
| Bubble Tea v2 compatibility for bubble-table? | MEDIUM | Test import with v2, check message handling (KeyPressMsg vs KeyMsg), or wait for official v2 release notes |
| Should Backends/Roles/Workflows use list or table? | LOW | Prototype both with bubbles/list and bubble-table (if compatible), evaluate UX |
| Dialog stack vs single dialog at a time? | LOW | Start with stack (more flexible), simplify to single if complexity unnecessary |
| Dirty config tracking for unsaved changes warning? | LOW | Implement if users request "unsaved changes" prompt on quit |

---

## Sources

### High Confidence (Official Docs)
- [bubbles/list package](https://pkg.go.dev/github.com/charmbracelet/bubbles/list) — Official list component API
- [bubbles/list README](https://github.com/charmbracelet/bubbles/blob/master/list/README.md) — ItemDelegate, filtering, pagination
- [Bubble Tea v2 Discussion #1374](https://github.com/charmbracelet/bubbletea/discussions/1374) — v2 breaking changes, message types
- [Crush TUI Architecture](https://deepwiki.com/charmbracelet/crush/5.1-tui-architecture) — Dialog system pattern (stack, modal routing)

### Medium Confidence (Community Libraries)
- [Evertras/bubble-table](https://github.com/Evertras/bubble-table) — Table component features, API
- [Evertras/bubble-table releases](https://github.com/Evertras/bubble-table/releases) — Latest version v0.19.2, no v2 mention
- [quickphosphat/bubbletea-overlay](https://pkg.go.dev/github.com/quickphosphat/bubbletea-overlay) — Basic overlay compositing
- [purpleclay/lipgloss-theme](https://pkg.go.dev/github.com/purpleclay/lipgloss-theme) — Theme library pattern, adaptive colors

### Low Confidence (Web Search Only)
- Crush dialog implementation details — Mentioned in DeepWiki but source code not directly verified
- bubble-table v2 compatibility — Unconfirmed, needs testing

---

*Stack research for: Dialog Overlay + CRUD Config Management*
*Researched: 2026-02-11*
*Confidence: MEDIUM-HIGH*
