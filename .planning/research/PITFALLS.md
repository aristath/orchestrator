# Pitfalls Research

**Domain:** Adding dialog overlay system and config refactor to existing Bubble Tea v2 TUI
**Researched:** 2026-02-11
**Confidence:** HIGH

## Critical Pitfalls

### Pitfall 1: Keyboard Event Leakage from Modal to Background

**What goes wrong:**
When a modal dialog is shown, keyboard events intended for the modal can "leak through" to the underlying panes, causing the background UI to respond to keypresses while the modal is active. For example, pressing 'q' in a modal input field could quit the entire application, or 'tab' could change the background pane focus instead of moving between form fields.

**Why it happens:**
Bubble Tea's event routing is hierarchical but not automatically modal-aware. The current implementation at lines 74-93 in `model.go` routes all keys to `settingsPane` when `m.showSettings` is true, but relies on manual checking. If the modal state flag is checked in the wrong order, or if child components don't respect the modal state, events will propagate incorrectly.

**How to avoid:**
- Implement early-return guard at the top of `Update()` before any other key handling
- Never route keyboard events to non-modal components when any modal is active
- Use a modal stack pattern if multiple overlays are possible (even if only one exists today)
- Test every keyboard shortcut explicitly with modal open to verify isolation

**Warning signs:**
- Background pane scrolling while typing in modal input
- 'q' or 'esc' quit the app instead of closing modal
- Tab cycling between background panes instead of modal fields
- Focus indicators appearing on background while modal is visible

**Phase to address:**
Phase 1 (Dialog System Foundation) — must be correct from the start or keyboard UX will be fundamentally broken

---

### Pitfall 2: Form State Not Resetting Between Modal Opens

**What goes wrong:**
When the settings modal is closed and reopened, previous values remain in the form fields or the form shows in a "completed" state, preventing user interaction. This occurs because huh.Form maintains internal state that persists across visibility toggles.

**Why it happens:**
The existing code at `settings_pane.go:268-271` attempts to rebuild the form when `SetVisible(true)` is called, but huh forms have complex internal state (`State`, field values, validation state) that doesn't reset automatically. Per the huh GitHub issue #319, forms that reach `StateCompleted` cannot be simply reinitialized — they must be fully recreated.

**How to avoid:**
- Always create a fresh `huh.Form` instance when showing modal, never reuse
- Reset all bound field variables to current config values before building form
- Don't rely on `form.Init()` to reset state — rebuild from scratch
- Store a "form generation counter" to force complete rebuilds

**Warning signs:**
- Modal opens showing "Settings saved successfully!" from previous session
- Form fields are disabled/uneditable when modal reopens
- Cannot navigate between fields with arrow keys after first open
- Form appears empty but won't accept input

**Phase to address:**
Phase 2 (Replace Hardcoded Settings Form) — existing code has the bug, must fix when implementing dynamic dialogs

---

### Pitfall 3: Config Schema Migration Without Backwards Compatibility

**What goes wrong:**
When refactoring from Provider/Agent/Workflow to Backend/Role/Workflow, existing config files fail to load, causing the application to fall back to defaults and silently lose user settings. Users who have customized configs find their settings wiped after upgrade.

**Why it happens:**
Go's `json.Unmarshal` into `OrchestratorConfig` will succeed even if the JSON schema doesn't match — it simply zero-values missing fields. The current loader at `loader.go:64-67` unmarshals into `OrchestratorConfig` directly, so old config files with different field names will unmarshal to an empty config with no error, which then gets overwritten by defaults.

**How to avoid:**
- Implement custom `UnmarshalJSON` method on `OrchestratorConfig` to detect old format
- Attempt to unmarshal as old schema first, then convert to new schema
- If unmarshal as new schema has empty required fields, retry as old schema
- Log a warning when migration occurs so users know config was converted
- Write migrated config back to disk in new format after first successful load

**Warning signs:**
- Unit tests loading old config fixtures pass but return default values
- Integration tests show "no config found" despite config file existing
- All users report losing settings after upgrade
- Config file exists but all Provider/Agent maps are empty in loaded struct

**Phase to address:**
Phase 3 (Config Type Refactor) — critical to implement migration BEFORE changing config schema, not after

---

### Pitfall 4: Shared Config Type Refactor Causing Import Cycles

**What goes wrong:**
When refactoring config types (Provider → Backend, Agent → Role), you need to update every file that imports `config.AgentConfig` or `config.ProviderConfig`. Creating a separate `config/migration` package to handle old/new schemas causes import cycles because the migration package needs to import both old and new `config` types, but `config` needs migration logic.

**Why it happens:**
Go's strict import cycle detection prevents circular dependencies. The config types are referenced in 24 files across the codebase (backend, scheduler, orchestrator, TUI, persistence). Attempting to create a "common" or "migration" package that both old and new code depend on creates a cycle: `config` → `migration` → `config`.

**How to avoid:**
- Keep migration logic inside the main `config` package, not a separate package
- Use type aliases temporarily during migration: `type ProviderConfig = BackendConfig`
- Implement migration as methods on the new types: `func (c *Config) UnmarshalJSON`
- Update all consuming packages simultaneously in a single phase
- Don't create intermediate "shared" packages for common types

**Warning signs:**
- Compile errors: "import cycle not allowed"
- Need to create `config/types` and `config/loader` split packages
- Consider creating `common` or `shared` package for types
- Find yourself moving structs between packages to break cycles

**Phase to address:**
Phase 3 (Config Type Refactor) — plan the refactor to avoid the cycle, don't create one and try to fix it

---

### Pitfall 5: Lipgloss Style Recomputation on Every Render

**What goes wrong:**
Building lipgloss.Style objects inside View() methods causes styles to be recomputed on every frame, even when dimensions/colors haven't changed. This adds 5-10ms per render frame, causing visible lag when scrolling or typing in modals, especially over SSH or slow terminals.

**Why it happens:**
The existing `settings_pane.go:235-240` creates new styles on every View() call. Lipgloss styles are builders that compute ANSI codes, measure text, and calculate padding/borders. Doing this 60 times per second (during smooth scrolling) wastes CPU and causes stutter.

**How to avoid:**
- Compute styles once at initialization or when dimensions change
- Store computed styles in model struct: `cachedBorderStyle lipgloss.Style`
- Only recompute when dependencies change (width, height, focus state)
- Use helper methods that return cached styles: `func (m Model) borderStyle() lipgloss.Style`
- Profile with `BUBBLETEA_PROFILE=1` to measure render time before/after caching

**Warning signs:**
- Scrolling feels "janky" or stutters
- Input lag when typing in modal forms
- CPU usage spikes during idle TUI display
- Render times >16ms (visible in profiling)
- Creating `lipgloss.NewStyle()` in hot path

**Phase to address:**
Phase 4 (Consistent Theme System) — implement theme as pre-computed style cache, not dynamic builders

---

### Pitfall 6: Dialog Overlay Z-Index Conflicts with Pane Borders

**What goes wrong:**
When a modal dialog is displayed using `lipgloss.Place` to center it, the dialog's border gets "cut off" by the background pane borders, or the background content renders on top of the modal, making text unreadable. This happens because lipgloss doesn't have true z-index layering — the View() return order determines paint order.

**Why it happens:**
The current `model.go:185-189` renders settings as the full view when `m.showSettings` is true, which works. But when implementing a proper overlay (rendering background + foreground together), the paint order matters. If you `lipgloss.JoinVertical(background, overlay)`, the overlay gets clipped by background dimensions. If you render background fully then overlay, the overlay might not cover the full screen.

**How to avoid:**
- Render background panes at normal dimensions
- Render overlay as a completely separate layer on top
- Don't use `lipgloss.JoinVertical/Horizontal` for overlay composition
- Use `lipgloss.Place` to position modal, but render background first, then Place result
- Consider using bubbletea-overlay library which handles composition correctly
- Test overlay rendering at different terminal sizes to verify no clipping

**Warning signs:**
- Modal border only visible on left/right, not top/bottom
- Background text visible through modal content area
- Modal appears "behind" pane borders
- Changing terminal size causes modal to disappear

**Phase to address:**
Phase 1 (Dialog System Foundation) — core overlay rendering must be solid before adding multiple dialog types

---

### Pitfall 7: CRUD List Navigation State Lost on Modal Open/Close

**What goes wrong:**
When viewing a list of Backends/Roles/Workflows, selecting one to edit (opening modal), editing values, and closing modal, the list cursor position resets to index 0 and the user must scroll back to where they were. This creates frustrating UX for editing multiple items in sequence.

**Why it happens:**
List components (like bubbles/list) maintain cursor position as internal state. If the list model is recreated when transitioning from list view → edit modal → list view, the state is lost. Similarly, if the list's Update() method doesn't run while the modal is open, it may clear state when resuming.

**How to avoid:**
- Keep list model instance alive across modal open/close transitions
- Store list cursor position explicitly in parent model
- Restore list position after modal closes: `m.list.Select(m.previousCursorIndex)`
- Don't recreate list model, just toggle visibility flag
- Pass the index being edited to modal, return it on close

**Warning signs:**
- List always shows first item selected after modal closes
- Scroll position resets to top of list after edit
- Cannot edit consecutive items without re-scrolling
- List appears to "flash" or redraw completely on modal close

**Phase to address:**
Phase 5 (CRUD List Implementation) — implement navigation state preservation from the start, hard to add later

---

### Pitfall 8: Huh Form Validation Errors Not Clearing Between Modals

**What goes wrong:**
When a user enters invalid data in a modal form (e.g., empty Backend command), sees the validation error, cancels the modal, then opens a different modal (e.g., to add a Role), the previous validation error message appears in the new modal's form fields.

**Why it happens:**
Huh's validation state is tied to field instances, not form instances. If you reuse field variables across multiple form builds (which happens if form field bindings are model struct fields like `m.backendName string`), the field's validation state persists. When you rebuild a form using the same bound variables, huh may show stale validation errors.

**How to avoid:**
- Always create new field binding variables for each form rebuild
- Don't store form field strings as model-level fields, use a sub-struct per form type
- Call `WithShowErrors(false)` when building forms for clean modals
- Re-enable errors only after first user interaction
- Consider creating separate model types for each dialog: `BackendEditModel`, `RoleEditModel`

**Warning signs:**
- Error messages appear in pristine forms that haven't been submitted
- Validation errors from one entity appear when creating a different entity type
- Form fields show red/error styling before user types anything
- Error messages are from previous editing session

**Phase to address:**
Phase 2 (Replace Hardcoded Settings Form) — establish pattern for clean form state before adding multiple dialogs

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Using string literals for modal state instead of enum/constants | Faster to type `m.mode = "edit-backend"` | Typos cause bugs, no IDE autocomplete, hard to refactor | Never — use constants |
| Single giant Update() switch for all modals | Don't need separate models per modal | Becomes unmaintainable at 3+ dialog types | Never — use model composition from start |
| Copying config struct instead of pointer passing | Avoid nil checks | Config changes in modal don't persist, causes "save failed" bugs | Never — config must be shared pointer |
| Rebuilding entire TUI layout on modal open | Simpler View() logic | Flickers, loses scroll position in background | Never — overlay should not redraw background |
| No migration, force users to reconfigure | Saves 2 hours of coding | Users lose all settings on upgrade, generates support requests | Never acceptable for v2.0 |
| Hard-coding dialog sizes instead of computing from content | Works for current terminal size | Breaks on small terminals (80x24), content gets clipped | Only for MVP if documented as limitation |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| huh forms in Bubble Tea | Calling `form.Update()` only when modal is visible | Always route messages to form, even when hidden, or recreate form on show |
| bubbles/list with modals | Stopping list Update() when modal opens | Continue routing messages to list, or explicitly pause/resume state |
| lipgloss styles in nested components | Passing width/height at construction | Recalculate dimensions on WindowSizeMsg, store in model |
| Backend factory with new config types | Mapping Agent.Provider → ProviderConfig | Mapping Role.Backend → BackendConfig (field name change) |
| Persistence layer with config refactor | Storing old `agent_name` field | Update schema to `role_name`, write migration |
| Event bus with modal dialogs | Publishing events from modal actions | Events may fire while TUI is in wrong state — guard with state checks |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Recreating lipgloss styles in View() | Stutter during scrolling, high CPU | Cache styles, recompute only on size/focus change | Immediately noticeable on SSH/slow terminals |
| Deep-copying config on every keystroke | Modal input lag (50-100ms per key) | Use pointer to shared config, modify in place | When config has 10+ backends/roles |
| Rendering background panes while modal open | Wasted CPU rendering invisible content | Skip background View() when modal is open | Not critical but wasteful |
| Linear search through config maps | Slow to open edit dialog for 50th backend | Maintain index map, or use ordered keys | At 20+ backends |
| Revalidating entire form on every keystroke | Input lag in multi-field forms | Validate field on blur, full form on submit | At 8+ fields per form |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No visual indication which list item is selected | User edits wrong backend | Highlight selected item with bold + color before opening edit modal |
| Modal opens with focus on "Cancel" button | User hits Enter accidentally, loses work | Focus first input field, not buttons |
| No confirmation dialog for delete actions | Accidental deletions | Add "Are you sure?" confirmation modal for destructive actions |
| ESC and 's' both close settings modal | Inconsistent behavior (ESC should cancel, 's' should toggle) | ESC cancels (discards changes), 's' saves and closes |
| No indication that changes require restart | User expects config changes to apply immediately | Show "Restart required" message after save, or reload config dynamically |
| Form submit on Enter in text input | User hits Enter to add newline, accidentally submits | Only submit on explicit button press or Ctrl+Enter |

## "Looks Done But Isn't" Checklist

- [ ] **Modal keyboard isolation:** Verified 'q', 'tab', '1', '2', '3' don't affect background when modal open
- [ ] **Config migration:** Tested loading v1 config file with old schema produces correct new schema values
- [ ] **Form state reset:** Verified opening modal, entering data, canceling, reopening shows clean form
- [ ] **List cursor preservation:** After editing item N, list cursor returns to item N, not item 0
- [ ] **Validation error clearing:** Opening create modal after failed edit shows no error messages
- [ ] **Style caching:** Profiled View() render time <5ms even with 10+ modals in codebase
- [ ] **Overlay rendering:** Modal visible and complete at 80x24 terminal size, no clipped borders
- [ ] **Delete confirmation:** Deleting a backend/role/workflow requires explicit confirmation
- [ ] **Error handling in CRUD:** Creating duplicate name shows error, doesn't silently fail
- [ ] **Config persistence:** Edited values survive app restart (saved to correct global/project path)
- [ ] **No orphaned references:** Deleting a Backend that's used by a Role shows error or cascade deletes
- [ ] **Esc vs Save behavior:** Esc discards changes (config unchanged), Save persists changes

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Keyboard events leak to background | LOW | Add early-return guard in Update() before any key handling (10 minutes) |
| Form state not resetting | LOW | Change SetVisible() to rebuild form instead of calling Init() (30 minutes) |
| No config migration | HIGH | Implement UnmarshalJSON, write tests for old fixtures, verify conversion (4 hours) |
| Import cycle from refactor | MEDIUM | Move migration into config package, use type aliases temporarily (2 hours) |
| Style recomputation lag | LOW | Extract style creation to init/size change methods, cache in model (1 hour) |
| Overlay z-index issues | MEDIUM | Rewrite View() to use proper overlay composition pattern (2 hours) |
| Lost list cursor position | LOW | Store cursor index before modal open, restore after close (30 minutes) |
| Validation errors persist | LOW | Create fresh field bindings per form instance (1 hour) |
| Orphaned references after delete | MEDIUM | Add referential integrity checks before delete, show error (3 hours) |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Keyboard event leakage | Phase 1 (Dialog Foundation) | All shortcuts tested with modal open, none affect background |
| Form state not resetting | Phase 2 (Replace Settings Form) | Modal opened 3x in row, each shows clean state |
| Config migration breaks | Phase 3 (Config Refactor) | Old config fixture loads correctly, values migrated |
| Import cycles | Phase 3 (Config Refactor) | Project compiles, no `import cycle` errors |
| Style recomputation | Phase 4 (Theme System) | Profile shows <5ms View() time |
| Overlay rendering | Phase 1 (Dialog Foundation) | Modal renders correctly at 80x24 and 200x60 sizes |
| List cursor lost | Phase 5 (CRUD Lists) | Edit item 5, cursor returns to item 5 |
| Validation errors persist | Phase 2 (Replace Settings Form) | Open create modal after validation failure shows clean form |
| No delete confirmation | Phase 5 (CRUD Lists) | Delete action requires confirmation, cannot accidentally delete |
| Orphaned references | Phase 5 (CRUD Lists) | Deleting Backend used by Role shows error |

## Sources

**Bubble Tea Dialog/Modal Patterns:**
- [GitHub bubbletea issue #642](https://github.com/charmbracelet/bubbletea/issues/642) — overlay keyboard event handling
- [bubbletea-overlay package](https://pkg.go.dev/github.com/quickphosphat/bubbletea-overlay) — overlay composition
- [Building Bubble Tea Programs](https://leg100.github.io/en/posts/building-bubbletea-programs/) — architecture patterns
- [Managing nested models with Bubble Tea](https://donderom.com/posts/managing-nested-models-with-bubble-tea/) — state management
- [BubbleTea multi model tutorial](https://blog.sometimestech.com/posts/bubbletea-multimodel) — multi-model routing

**Bubble Tea State Management:**
- [Handling state duplication - Discussion #707](https://github.com/charmbracelet/bubbletea/discussions/707) — state sharing pitfalls
- [Bubbletea State Machine pattern](https://zackproser.com/blog/bubbletea-state-machine) — state transitions

**Huh Form Issues:**
- [reset/show again a form - Issue #319](https://github.com/charmbracelet/huh/issues/319) — form state doesn't reset, must recreate
- [huh package documentation](https://pkg.go.dev/github.com/charmbracelet/huh/v2) — validation and error handling

**Go Config Migration:**
- [Go config schema migration](https://github.com/adlio/schema) — migration patterns
- [Database migrations in Go](https://betterstack.com/community/guides/scaling-go/golang-migrate/) — backwards compatibility
- [JSON evolution in Go](https://antonz.org/go-json-v2/) — UnmarshalJSON patterns
- [encoding/json package](https://pkg.go.dev/encoding/json) — custom unmarshaling

**Go Refactoring Pitfalls:**
- [Codebase Refactoring with Go](https://go.dev/talks/2016/refactor.article) — type refactoring at scale
- [Go issue #18130](https://github.com/golang/go/issues/18130) — moving types between packages
- [Practical Go](https://dave.cheney.net/practical-go/presentations/qcon-china.html) — anti-patterns including shared packages

**Lipgloss Performance:**
- [lipgloss package](https://pkg.go.dev/github.com/charmbracelet/lipgloss) — style composition
- [Terminal Capabilities and Styling](https://deepwiki.com/charmbracelet/mods/4.1-terminal-capabilities-and-styling) — performance patterns

**Bubble Tea v2 Changes:**
- [Bubble Tea v2 Discussion #1374](https://github.com/charmbracelet/bubbletea/discussions/1374) — breaking changes and migration
- [Bubble Tea releases](https://github.com/charmbracelet/bubbletea/releases) — changelog

---
*Pitfalls research for: Adding dialog overlay system and config refactor to existing Bubble Tea v2 TUI*
*Researched: 2026-02-11*
