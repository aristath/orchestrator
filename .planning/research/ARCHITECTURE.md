# Architecture Research

**Domain:** Dialog overlay system and dynamic config for Bubble Tea TUI
**Researched:** 2026-02-11
**Confidence:** HIGH

## Existing Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Root Model (tui.Model)                   │
│  - Pane focus state (Tab cycling)                           │
│  - Modal overlay flag (showSettings)                        │
│  - Event bus subscription                                   │
│  - Config reference                                          │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ AgentPane    │  │ DAGPane      │  │ SettingsPane │       │
│  │              │  │              │  │ (Modal)      │       │
│  │ - Agent list │  │ - DAG        │  │ - huh.Form   │       │
│  │ - Output     │  │   progress   │  │ - Overlay    │       │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │
│         │                 │                 │                │
├─────────┴─────────────────┴─────────────────┴────────────────┤
│                       Event Bus                               │
│  TaskStartedEvent → TaskOutputEvent → TaskCompletedEvent     │
│  DAGProgressEvent → TaskFailedEvent → TaskMergedEvent        │
├─────────────────────────────────────────────────────────────┤
│                    Config Layer                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ Providers    │  │ Agents       │  │ Workflows    │       │
│  │ map[string]  │  │ map[string]  │  │ map[string]  │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
├─────────────────────────────────────────────────────────────┤
│                    Backend Factory                           │
│  ProviderConfig → Backend interface (Claude/Codex/Goose)    │
└─────────────────────────────────────────────────────────────┘
```

### Current Component Responsibilities

| Component | Responsibility | Current Implementation |
|-----------|----------------|------------------------|
| **Root Model** | Pane focus management, event routing, modal state | Single `showSettings` bool flag, direct key handling |
| **AgentPaneModel** | Display running agents and output | Viewport-based output, event-driven updates |
| **DAGPaneModel** | DAG visualization and progress | Receives DAGProgressEvent from event bus |
| **SettingsPaneModel** | Config editing overlay | Uses huh.Form, full-screen overlay, manual visibility flag |
| **Event Bus** | Async event propagation | Channel-based broadcast, subscription model |
| **Config Loader** | Two-tier global+project merge | JSON files, static load at startup |
| **Backend Factory** | Provider instantiation | Switch on Type string, creates Backend interface |

## Target Architecture for New Features

### System Structure with Dialog Stack

```
┌─────────────────────────────────────────────────────────────┐
│                    Root Model (tui.Model)                   │
│  + Dialog stack []Dialog (NEW)                              │
│  + Theme *Theme (NEW)                                       │
│  + Config hot-reload subscription (NEW)                     │
├─────────────────────────────────────────────────────────────┤
│                      Dialog Layer (NEW)                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ Settings     │  │ Backend      │  │ Role         │       │
│  │ Dialog       │  │ CRUD Dialog  │  │ CRUD Dialog  │       │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │
│         │                 │                 │                │
│         └─────────────────┴─────────────────┘                │
│                  Common Dialog Interface                     │
│  - IsOpen() bool                                             │
│  - HandleInput(msg) tea.Cmd                                  │
│  - View(w, h int) string                                     │
│  - Close()                                                   │
├─────────────────────────────────────────────────────────────┤
│                   Reusable List Component (NEW)              │
│  ┌─────────────────────────────────────────────────┐         │
│  │ CRUDList[T]                                     │         │
│  │  - bubbles/list wrapper                         │         │
│  │  - Item actions (Add/Edit/Delete)               │         │
│  │  - Form integration for create/edit             │         │
│  │  - Confirmation prompts                         │         │
│  └─────────────────────────────────────────────────┘         │
├─────────────────────────────────────────────────────────────┤
│                  Existing Panes (UNCHANGED)                 │
│  ┌──────────────┐  ┌──────────────┐                          │
│  │ AgentPane    │  │ DAGPane      │                          │
│  │              │  │              │                          │
│  └──────────────┘  └──────────────┘                          │
├─────────────────────────────────────────────────────────────┤
│              Updated Config Types (REFACTOR)                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ Backends     │  │ Roles        │  │ Workflows    │       │
│  │ (was         │  │ (was Agents) │  │ (unchanged)  │       │
│  │  Providers)  │  │              │  │              │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

## Recommended Project Structure

```
internal/tui/
├── model.go              # Root model (MODIFY: add dialog stack)
├── keys.go               # Key bindings (UNCHANGED)
├── styles.go             # Basic styles (REFACTOR → theme.go)
├── agent_pane.go         # Agent display (UNCHANGED)
├── dag_pane.go           # DAG display (UNCHANGED)
├── settings_pane.go      # Settings modal (REFACTOR → dialog)
│
├── theme.go              # NEW: Centralized lipgloss theme
├── dialog.go             # NEW: Dialog interface & stack helpers
│
├── dialogs/              # NEW: Dialog implementations
│   ├── settings.go       # Migrated from settings_pane.go
│   ├── backends.go       # Backend CRUD dialog
│   ├── roles.go          # Role CRUD dialog
│   ├── workflows.go      # Workflow CRUD dialog
│   ├── form.go           # Generic form dialog
│   └── confirm.go        # Yes/No confirmation
│
└── components/           # NEW: Reusable components
    └── crudlist.go       # Generic CRUD list wrapper

internal/config/
├── types.go              # MODIFY: Rename Provider→Backend, Agent→Role
├── loader.go             # MODIFY: Add backward compat migration
├── defaults.go           # MODIFY: Update default names
├── save.go               # UNCHANGED
└── *_test.go             # UPDATE: Test new names
```

### Structure Rationale

- **`dialogs/` package:** All dialogs in one place, implements `Dialog` interface consistently. Easy to add new dialogs without touching root model.

- **`components/` package:** Reusable UI components that aren't full dialogs. `CRUDList` is data structure wrapper, not Bubble Tea model.

- **`theme.go` at root:** Theme shared across all dialogs and panes. Single import point for consistent styling.

- **Config types stay in `config/`:** Domain logic separate from UI. UI imports config types, not vice versa.

## Architectural Patterns

### Pattern 1: Dialog Stack with Interface-Based Overlay

**What:** Manage multiple modal dialogs as a stack where topmost dialog has input priority. Based on Crush TUI pattern.

**When to use:** Multiple overlapping modal interactions (settings, CRUD forms, confirmations).

**Trade-offs:**
- **Pro:** Clean separation, predictable input routing, easy to add new dialogs
- **Pro:** Stack makes back navigation natural (Esc pops stack)
- **Con:** Requires consistent dialog interface implementation
- **Con:** Render order must be managed carefully for z-index

**Example:**
```go
// Dialog interface - all dialogs implement this
type Dialog interface {
    IsOpen() bool
    HandleInput(msg tea.Msg) tea.Cmd
    View(w, h int) string
    Close()
}

// Root model maintains stack
type Model struct {
    dialogStack []Dialog
    // ... existing fields
}

// Update routing - topmost dialog gets priority
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // If any dialog is open, route to topmost
    if len(m.dialogStack) > 0 {
        top := m.dialogStack[len(m.dialogStack)-1]
        if top.IsOpen() {
            cmd := top.HandleInput(msg)

            // After handling, check if closed and pop
            if !top.IsOpen() {
                m.dialogStack = m.dialogStack[:len(m.dialogStack)-1]
            }
            return m, cmd
        }
    }

    // Otherwise, normal pane routing
    // ... existing pane delegation
}

// View rendering - compose overlays
func (m Model) View() string {
    base := m.renderPanes()

    // Layer each dialog from bottom to top
    for _, dialog := range m.dialogStack {
        if dialog.IsOpen() {
            overlay := dialog.View(m.width, m.height)
            base = placeOverlay(base, overlay, m.width, m.height)
        }
    }

    return base
}
```

**Integration with existing:**
- Replace `showSettings bool` with dialog stack
- Migrate `SettingsPaneModel` to implement `Dialog` interface
- Keep existing pane structure unchanged
- Event bus continues unchanged

### Pattern 2: Generic CRUD List Component

**What:** Reusable list component wrapping bubbles/list with built-in add/edit/delete operations.

**When to use:** Managing collections (backends, roles, workflows).

**Trade-offs:**
- **Pro:** DRY - one implementation for all CRUD lists
- **Pro:** Consistent UX across all config editing
- **Con:** Generic constraints may feel clunky in Go 1.21
- **Con:** Each item type needs adapter to list.Item interface

**Example:**
```go
// Generic CRUD list wrapping bubbles/list
type CRUDList[T any] struct {
    list      list.Model
    items     []T
    toItem    func(T) list.Item        // Adapter to list.Item
    fromItem  func(list.Item) T        // Adapter from list.Item

    // Optional form for add/edit
    formDialog Dialog

    // Confirmation for delete
    confirmDialog Dialog
}

// Key methods
func (c *CRUDList[T]) HandleKey(key string) tea.Cmd {
    switch key {
    case "a":
        return c.openAddForm()
    case "e":
        return c.openEditForm()
    case "d":
        return c.openDeleteConfirm()
    default:
        // Delegate to bubbles/list
        var cmd tea.Cmd
        c.list, cmd = c.list.Update(msg)
        return cmd
    }
}

// CRUD operations use list methods
func (c *CRUDList[T]) Add(item T) tea.Cmd {
    listItem := c.toItem(item)
    return c.list.InsertItem(len(c.items), listItem)
}

func (c *CRUDList[T]) Update(index int, item T) tea.Cmd {
    listItem := c.toItem(item)
    return c.list.SetItem(index, listItem)
}

func (c *CRUDList[T]) Delete(index int) {
    c.list.RemoveItem(index)
}
```

**Usage for backends:**
```go
type BackendListItem struct {
    config.BackendConfig
    name string
}

func (b BackendListItem) FilterValue() string { return b.name }
func (b BackendListItem) Title() string       { return b.name }
func (b BackendListItem) Description() string { return b.Type }

// Create CRUD list
backendList := NewCRUDList(
    backends,
    func(bc config.BackendConfig) list.Item { return BackendListItem{bc, name} },
    func(item list.Item) config.BackendConfig { return item.(BackendListItem).BackendConfig },
)
```

**Integration points:**
- Each CRUD dialog contains a `CRUDList[ConfigType]`
- Form dialogs for add/edit pushed onto dialog stack
- Confirmation dialogs pushed onto dialog stack
- On save, config propagation triggered (see Pattern 4)

### Pattern 3: Centralized Theme System

**What:** Single source of truth for lipgloss styles, organized by component and semantic purpose.

**When to use:** Need consistent styling across dialogs, panes, and components.

**Trade-offs:**
- **Pro:** Single place to adjust colors/styles
- **Pro:** Supports future theme switching (light/dark)
- **Con:** Global state can make testing harder
- **Con:** Need discipline to use theme vs inline styles

**Example:**
```go
// internal/tui/theme.go
type Theme struct {
    // Border styles
    FocusedBorder   lipgloss.Style
    UnfocusedBorder lipgloss.Style
    DialogBorder    lipgloss.Style

    // Status colors
    Running   lipgloss.Style
    Complete  lipgloss.Style
    Failed    lipgloss.Style
    Pending   lipgloss.Style

    // UI element styles
    Title       lipgloss.Style
    Help        lipgloss.Style
    ListItem    lipgloss.Style
    ListSelected lipgloss.Style
    ButtonPrimary lipgloss.Style
    ButtonSecondary lipgloss.Style

    // Dialog-specific
    DialogTitle     lipgloss.Style
    DialogHelp      lipgloss.Style
    FormLabel       lipgloss.Style
    FormInput       lipgloss.Style
    FormError       lipgloss.Style
}

func DefaultTheme() *Theme {
    return &Theme{
        FocusedBorder: lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("62")),

        UnfocusedBorder: lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("240")),

        DialogBorder: lipgloss.NewStyle().
            Border(lipgloss.ThickBorder()).
            BorderForeground(lipgloss.Color("170")),

        // ... rest of styles
    }
}

// Pass theme to dialogs
type BackendCRUDDialog struct {
    theme *Theme
    // ...
}

func (d *BackendCRUDDialog) View(w, h int) string {
    title := d.theme.DialogTitle.Render("Manage Backends")
    // ... use theme throughout
}
```

**Integration:**
- Create `Theme` in root model initialization
- Pass to all panes and dialogs via constructor
- Replace direct `StyleFocusedBorder` usage with `theme.FocusedBorder`
- Existing styles.go becomes theme factory

### Pattern 4: Config Change Propagation

**What:** Notify running components when config changes, based on Bubble Tea message routing.

**When to use:** Config edits should affect runtime without restart.

**Trade-offs:**
- **Pro:** Immediate feedback on config changes
- **Pro:** Aligns with event-driven architecture
- **Con:** Not all changes can be hot-reloaded (backend type switch)
- **Con:** Need careful handling of in-flight tasks

**Example:**
```go
// Config change event
type ConfigChangedMsg struct {
    Section string // "backends", "roles", "workflows"
    Config  *config.OrchestratorConfig
}

// Root model sends after dialog saves
func (m Model) saveConfig(cfg *config.OrchestratorConfig) tea.Cmd {
    return func() tea.Msg {
        if err := config.Save(cfg, m.projectConfigPath); err != nil {
            return ConfigSaveErrorMsg{err}
        }
        return ConfigChangedMsg{
            Section: "backends",
            Config:  cfg,
        }
    }
}

// Components subscribe to config changes
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case ConfigChangedMsg:
        // Update config reference
        m.config = msg.Config

        // Propagate to components that need it
        var cmds []tea.Cmd

        // Backend factory needs to recreate backends
        if msg.Section == "backends" {
            cmds = append(cmds, m.restartBackends())
        }

        // Agent pane might update display
        if msg.Section == "roles" {
            var cmd tea.Cmd
            m.agentPane, cmd = m.agentPane.Update(msg)
            cmds = append(cmds, cmd)
        }

        return m, tea.Batch(cmds...)
    }
}
```

**Propagation strategy:**
- **Backends:** Requires backend restart (close old, create new)
- **Roles:** Update display only (affects new tasks)
- **Workflows:** Update registry (affects new workflow starts)

**Integration:**
- Dialog save triggers `ConfigChangedMsg`
- Root model broadcasts to relevant panes
- Components handle or ignore based on section
- Running tasks continue with old config (graceful)

### Pattern 5: Overlay Composition Using lipgloss.Place

**What:** Render dialog as overlay centered on base view using lipgloss.Place for positioning.

**When to use:** Modal dialogs that need precise centering over base content.

**Trade-offs:**
- **Pro:** Simple API, handles ANSI escape sequences correctly
- **Pro:** No manual line-by-line composition needed
- **Con:** Limited to rectangular overlays
- **Con:** Entire base view re-rendered even if unchanged

**Example:**
```go
import "github.com/charmbracelet/lipgloss"

func placeOverlay(base, overlay string, width, height int) string {
    // Place overlay centered on base
    return lipgloss.Place(
        width,
        height,
        lipgloss.Center,  // horizontal
        lipgloss.Center,  // vertical
        overlay,
        lipgloss.WithWhitespaceChars(" "),
        lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
    )
}

// Alternative: custom positioning
func placeOverlayCustom(base, overlay string, x, y, width, height int) string {
    // Split base into lines
    baseLines := strings.Split(base, "\n")
    overlayLines := strings.Split(overlay, "\n")

    // Composite overlay onto base at position
    for i, line := range overlayLines {
        row := y + i
        if row >= 0 && row < len(baseLines) {
            baseLines[row] = composeLine(baseLines[row], line, x)
        }
    }

    return strings.Join(baseLines, "\n")
}
```

**Integration:**
- Use in `Model.View()` after rendering base panes
- Each dialog in stack rendered as overlay on top of previous
- Dialogs don't need to know about base view structure

## Data Flow

### Dialog Lifecycle Flow

```
User presses 'b' (manage backends)
    ↓
Root Model.Update() receives KeyMsg
    ↓
Check: Is dialog stack empty?
    ↓ YES
Root pushes BackendCRUDDialog onto stack
    ↓
BackendCRUDDialog.HandleInput() processes keys
    ↓
User edits, adds, deletes items
    ↓
User presses 's' (save)
    ↓
Dialog updates config, returns ConfigChangedMsg cmd
    ↓
Dialog.Close() marks closed
    ↓
Root.Update() receives ConfigChangedMsg
    ↓
Root pops closed dialog from stack
    ↓
Root propagates config change to components
    ↓
Components update based on changed section
```

### Config Type Migration Flow

```
Current: ProviderConfig → AgentConfig → Backend factory
                ↓
Proposed: BackendConfig → RoleConfig → Backend factory
```

**Migration strategy:**
- `ProviderConfig` → `BackendConfig` (rename Type field semantics same)
- `AgentConfig` → `RoleConfig` (Provider becomes Backend reference)
- `Providers map[string]` → `Backends map[string]`
- `Agents map[string]` → `Roles map[string]`
- Workflows unchanged (steps reference roles not agents)
- Backend factory unchanged (Type field logic identical)

**Backward compatibility:**
- Config loader checks for old keys, migrates on load
- Save always uses new schema
- One-time migration per config file

### Key Data Flows

1. **Dialog Open Flow:** User key → Root checks stack → Push dialog → Dialog renders overlay
2. **Dialog Input Flow:** User key → Root routes to top dialog → Dialog processes → Return cmd
3. **Dialog Close Flow:** Dialog marks closed → Root pops on next update → Back to pane routing
4. **CRUD Add Flow:** Dialog opens form → User fills → Form returns item → List.InsertItem() → Config save
5. **CRUD Edit Flow:** List selection → Dialog opens form → User edits → Form returns item → List.SetItem() → Config save
6. **CRUD Delete Flow:** List selection → Dialog opens confirm → User confirms → List.RemoveItem() → Config save
7. **Config Propagation Flow:** Save complete → ConfigChangedMsg → Root broadcasts → Components update

## Integration Points

### New Components

| Component | Type | Integration Point |
|-----------|------|-------------------|
| **Dialog Interface** | New | Root model maintains stack, routes input |
| **BackendCRUDDialog** | New | Implements Dialog, uses CRUDList, pushes forms |
| **RoleCRUDDialog** | New | Implements Dialog, uses CRUDList, pushes forms |
| **WorkflowCRUDDialog** | New | Implements Dialog, uses CRUDList, pushes forms |
| **CRUDList[T]** | New | Wraps bubbles/list, embedded in CRUD dialogs |
| **FormDialog** | New | Generic form wrapper for add/edit, implements Dialog |
| **ConfirmDialog** | New | Yes/No confirmation, implements Dialog |
| **Theme** | New | Created in root, passed to all components |

### Modified Components

| Component | Changes | Reason |
|-----------|---------|--------|
| **Root Model** | Add `dialogStack []Dialog`, remove `showSettings bool`, add `theme *Theme` | Dialog stack architecture |
| **Root Model.Update()** | Add dialog routing logic before pane routing | Input priority for dialogs |
| **Root Model.View()** | Add dialog overlay composition | Render dialog stack |
| **SettingsPaneModel** | Convert to SettingsDialog, implement Dialog interface | Consistency with other dialogs |
| **config.OrchestratorConfig** | Rename Providers→Backends, Agents→Roles | Terminology alignment |
| **config.Loader** | Add migration from old schema | Backward compatibility |
| **backend.New()** | Reference updated config types | Config rename propagation |

### Unchanged Components

- **AgentPaneModel** (displays agents, no config editing)
- **DAGPaneModel** (displays DAG, no config editing)
- **Event Bus** (orthogonal to UI changes)
- **Backend Factory Logic** (Type switching unchanged)
- **Task Scheduler** (not affected by UI changes)

## Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| **Root ↔ Dialogs** | Message delegation, cmd returns | Dialogs are dumb components, root controls lifecycle |
| **Root ↔ Panes** | Message delegation when no dialog open | Existing pattern unchanged |
| **Dialog ↔ CRUDList** | Direct method calls | CRUDList is data structure, not Bubble Tea component |
| **Dialog ↔ Form Dialog** | Push/pop on dialog stack | Forms are dialogs too |
| **Config ↔ All Components** | Read config pointer, react to ConfigChangedMsg | Unidirectional data flow |
| **Theme ↔ All Components** | Read theme pointer | Shared immutable reference |

## Anti-Patterns

### Anti-Pattern 1: Dialog Directly Modifying Config

**What people do:** Dialog saves config file directly in Update().

**Why it's wrong:** Violates single responsibility, breaks config propagation, doesn't notify other components.

**Do this instead:** Dialog returns `tea.Cmd` that sends `ConfigChangedMsg`. Root model orchestrates save and propagation.

```go
// WRONG
func (d *BackendDialog) Update(msg tea.Msg) tea.Cmd {
    if saveKey {
        config.Save(d.config, "/path/to/config") // Direct save
        d.Close()
    }
}

// RIGHT
func (d *BackendDialog) HandleInput(msg tea.Msg) tea.Cmd {
    if saveKey {
        d.Close()
        return func() tea.Msg {
            return ConfigChangedMsg{Section: "backends", Config: d.config}
        }
    }
}
```

### Anti-Pattern 2: Multiple Overlay State Flags

**What people do:** Add `showBackendDialog bool`, `showRoleDialog bool`, etc.

**Why it's wrong:** Doesn't scale, can't handle stacked dialogs, complex state management.

**Do this instead:** Use dialog stack, push/pop as needed.

```go
// WRONG
type Model struct {
    showSettings bool
    showBackends bool
    showRoles bool
    showWorkflows bool
}

// RIGHT
type Model struct {
    dialogStack []Dialog
}
```

### Anti-Pattern 3: Inline Lipgloss Styles

**What people do:** Create styles directly in View() methods.

**Why it's wrong:** Inconsistent styling, can't switch themes, duplicated style definitions.

**Do this instead:** Use centralized theme.

```go
// WRONG
func (d Dialog) View() string {
    title := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("62")).
        Render("Title")
}

// RIGHT
func (d Dialog) View() string {
    title := d.theme.DialogTitle.Render("Title")
}
```

### Anti-Pattern 4: Tight Coupling to bubbles/list Internals

**What people do:** Directly manipulate list model fields, bypass methods.

**Why it's wrong:** Breaks encapsulation, fragile to bubbles updates, commands ignored.

**Do this instead:** Use list methods, handle returned commands.

```go
// WRONG
func (c *CRUDList) Add(item T) {
    c.list.items = append(c.list.items, item) // Don't access internals
}

// RIGHT
func (c *CRUDList) Add(item T) tea.Cmd {
    listItem := c.toItem(item)
    return c.list.InsertItem(len(c.items), listItem) // Use API, return cmd
}
```

## Recommended Build Order

Build order accounts for dependencies and incremental testing:

### Phase 1: Foundation (No UI changes visible)
1. **Theme system** - Create `theme.go`, migrate existing styles, test with current UI
2. **Config type rename** - Rename types, add backward compatibility, test load/save
3. **Dialog interface** - Define interface, stub implementations, no integration yet

**Why first:** These are foundational changes that don't affect existing UI flow. Can be tested independently.

### Phase 2: Dialog Stack (Modal behavior changes)
4. **Dialog stack in root model** - Add stack field, update routing logic
5. **Migrate SettingsPane to Dialog** - Refactor existing modal to new interface
6. **Test dialog open/close/stack** - Verify Esc pops, input routing works

**Why second:** Proves dialog stack architecture with existing modal before building new features.

### Phase 3: Reusable Components (Building blocks)
7. **CRUDList component** - Generic list wrapper, test with mock data
8. **FormDialog** - Generic form wrapper using huh.Form
9. **ConfirmDialog** - Simple yes/no confirmation dialog

**Why third:** Build reusable components separately, test in isolation before integration.

### Phase 4: CRUD Dialogs (New features)
10. **BackendCRUDDialog** - Full add/edit/delete for backends
11. **RoleCRUDDialog** - Full add/edit/delete for roles
12. **WorkflowCRUDDialog** - Full add/edit/delete for workflows

**Why fourth:** Each dialog uses components from Phase 3, tests full CRUD flow.

### Phase 5: Config Propagation (Runtime updates)
13. **ConfigChangedMsg** - Add event type, wire save → message
14. **Component subscription** - AgentPane, DAGPane react to changes
15. **Backend restart logic** - Handle backend config changes gracefully

**Why last:** Most complex feature, depends on all dialogs working. Tests end-to-end flow.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| **1-5 dialogs** | Current stack-based pattern is ideal |
| **5-20 dialogs** | Consider dialog registry with ID-based lookup, lazy initialization |
| **20+ dialogs** | Probably over-engineered for TUI, but could add dialog manager with lifecycle hooks |

### Scaling Priorities

1. **First bottleneck:** Dialog rendering performance with complex overlays
   - **Fix:** Cache rendered dialogs, only re-render on input or size change
   - **Fix:** Use lipgloss.Place() for efficient overlay composition

2. **Second bottleneck:** Config file size with many backends/roles
   - **Fix:** Lazy load config sections, pagination in CRUD lists
   - **Fix:** Consider splitting into multiple files (backends.json, roles.json)

3. **Third bottleneck:** Config propagation causing visible lag
   - **Fix:** Debounce config saves, batch multiple edits
   - **Fix:** Make propagation async with loading indicators

## Sources

### Bubble Tea Architecture
- [Bubble Tea GitHub Repository](https://github.com/charmbracelet/bubbletea)
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/)
- [Bubble Tea v2 Discussion](https://github.com/charmbracelet/bubbletea/discussions/1374)

### Dialog Overlay Patterns
- [Overlay Composition Using Bubble Tea - Leon Mika](https://lmika.org/2022/09/24/overlay-composition-using.html)
- [Bubble Tea Overlay Package](https://pkg.go.dev/github.com/quickphosphat/bubbletea-overlay)
- [Crush TUI Architecture](https://deepwiki.com/charmbracelet/crush/5.1-tui-architecture)

### List Component & CRUD
- [Bubbles List Component](https://pkg.go.dev/github.com/charmbracelet/bubbles/list)
- [Bubbles List README](https://github.com/charmbracelet/bubbles/blob/master/list/README.md)

### Lipgloss Theme System
- [Lipgloss GitHub Repository](https://github.com/charmbracelet/lipgloss)
- [Building Terminal UI with Go, Bubble Tea, and Lipgloss](https://www.grootan.com/blogs/building-an-awesome-terminal-user-interface-using-go-bubble-tea-and-lip-gloss/)

### Config and State Management
- [Synchronized Output Mode 2026](https://github.com/charmbracelet/bubbletea/discussions/1320)
- [Bubble Tea State Machine Pattern](https://zackproser.com/blog/bubbletea-state-machine)

---
*Architecture research for: Dialog overlay system and dynamic config integration*
*Researched: 2026-02-11*
