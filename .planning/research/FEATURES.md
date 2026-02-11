# Feature Research

**Domain:** TUI Config Management Dialogs (Backends/Roles/Workflows CRUD)
**Researched:** 2026-02-11
**Confidence:** MEDIUM

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| List view with keyboard navigation | Standard TUI pattern - users expect j/k or arrow keys | LOW | Bubbles list component provides this OOTB |
| Visual selection indicator | Users need to see what's focused/selected | LOW | List component includes cursor/highlight |
| ESC to close/cancel | Universal TUI escape hatch | LOW | Already implemented in current settings pane |
| Enter to confirm/select | Standard keyboard interaction | LOW | Huh forms use this; list selection needs explicit handling |
| Help overlay (?) | k9s, lazygit, and all modern TUIs show help | MEDIUM | Need to track keybindings and render help modal |
| Inline validation feedback | Prevent invalid configs from being created | MEDIUM | Huh supports validation; need error messaging strategy |
| Empty state messaging | List views need "No items yet" guidance | LOW | Simple conditional rendering |
| Tab/Shift-Tab field navigation | Standard form navigation pattern | LOW | Huh handles this in forms |
| Confirmation on delete | Destructive actions need confirmation | LOW | Modal dialog or inline confirmation |
| Save target selection (global/project) | Users expect to choose config scope | LOW | Already exists in current settings pane |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Multi-step wizard for role/workflow creation | Simplifies complex config creation vs single giant form | MEDIUM | Huh supports multi-group forms for wizard pattern |
| Context-aware defaults | Pre-fill provider based on selected backend type | LOW | Lookup table + form field dependencies |
| Live preview of workflow steps | See pipeline before saving (ASCII flow diagram) | MEDIUM | Requires DAG visualization library or custom rendering |
| Duplicate/clone existing items | Faster than creating from scratch | LOW | Copy existing config + edit name |
| Import/export role templates | Share role configs across projects | MEDIUM | JSON serialization + file picker |
| Keyboard shortcut customization | Power users want to remap keys | HIGH | Requires keybinding config system + conflict detection |
| Inline documentation tooltips | Show field help without leaving form | MEDIUM | Overlay or status bar for field descriptions |
| Search/filter in lists | Quick navigation in large lists | MEDIUM | Bubbles list has filtering OOTB |
| Undo/redo on config changes | Safety net for mistakes | HIGH | Requires command pattern + history tracking |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Mouse support for clicking | "GUIs have it" | Breaks keyboard flow, accessibility nightmare, terminal inconsistency | Strong keyboard UX with visual feedback |
| Real-time validation during typing | "Immediate feedback is good" | Interrupts flow, premature error messages, annoying for partial input | Validate on blur or submit only |
| Drag-and-drop list reordering | "Visual and intuitive" | Mouse dependency, complex state management, terminal rendering issues | j/k + Ctrl+arrow to move items |
| Auto-save on every change | "Never lose work" | Race conditions with file watchers, unclear when config is applied, no rollback | Explicit save with confirmation |
| WYSIWYG config editor | "Like a GUI config tool" | TUI is not GUI, breaks terminal paradigm, complex state sync | Form-based editing with preview |
| Nested dialog modals | "Step-by-step guidance" | Modal stacking is confusing, ESC behavior ambiguity, focus management hell | Single modal with multi-step form |

## Feature Dependencies

```
[List View]
    └──requires──> [Item Selection]
                       └──requires──> [Keyboard Navigation]

[Edit Item] ──requires──> [Form Rendering]
[Create Item] ──requires──> [Form Rendering]
                               └──requires──> [Validation]

[Delete Item] ──requires──> [Confirmation Dialog]
                               └──requires──> [Modal Overlay]

[Workflow Creation] ──requires──> [Role List] (select from existing roles)
                                     └──requires──> [Role CRUD] (create roles first)

[Help Overlay] ──enhances──> [All Features] (shows available keybindings)

[Search/Filter] ──enhances──> [List View] (optional for small lists)

[Multi-step Wizard] ──conflicts──> [Single-page Form] (choose one approach per item type)
```

### Dependency Notes

- **Edit/Create require Form Rendering:** Both operations use the same form component (Huh), populated differently
- **Workflow Creation requires Role List:** Can't create workflow without existing roles to reference
- **Delete requires Confirmation:** Destructive action needs safety check via modal
- **Modal Overlay is foundation:** Settings pane, confirmation dialogs, help all use overlay pattern
- **Multi-step Wizard vs Single-page Form:** Roles might need wizard (backend -> model -> prompt -> tools), but simple items like backend CLI config can use single form

## MVP Definition

### Launch With (v1)

Minimum viable product - what's needed to validate dynamic config management.

- [ ] **List view for Backends/Roles/Workflows** - Users need to see what exists (Bubbles list component)
- [ ] **Create new items via form** - Core CRUD operation (Huh form in modal overlay)
- [ ] **Edit existing items** - Modify configs without manual JSON editing (Pre-populated Huh form)
- [ ] **Delete items with confirmation** - Remove unused configs safely (Modal confirmation dialog)
- [ ] **Modal overlay system** - Foundation for all dialogs (bubbletea-overlay or custom composite)
- [ ] **Keyboard navigation (j/k, Enter, ESC)** - Standard TUI controls (Bubbles list + Huh built-ins)
- [ ] **Empty state messaging** - Guide users when lists are empty ("Press 'a' to add first role")
- [ ] **Inline form validation** - Prevent invalid configs (Huh validation on required fields)
- [ ] **Global vs project save target** - Respect existing config scoping (Already in settings pane)

### Add After Validation (v1.x)

Features to add once core CRUD is working and validated.

- [ ] **Search/filter in lists** - Add when lists grow beyond ~10 items (Bubbles filtering)
- [ ] **Help overlay (?)** - Add when keybindings exceed what users remember (Modal with keybinding list)
- [ ] **Multi-step wizard for roles** - Add if single-form UX is overwhelming (Huh multi-group forms)
- [ ] **Duplicate/clone items** - Add when users request it (Copy + rename pattern)
- [ ] **Context-aware defaults** - Add when backend types expand (Provider lookup by backend type)
- [ ] **Live workflow preview** - Add if users struggle with workflow structure (ASCII pipeline visualization)

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] **Import/export templates** - Wait for community to emerge with shareable configs
- [ ] **Inline documentation tooltips** - Wait for user confusion patterns to emerge
- [ ] **Undo/redo** - Complex feature, only if users frequently make mistakes
- [ ] **Keyboard shortcut customization** - Power user feature, defer until core UX is solid
- [ ] **Role/workflow versioning** - Defer until users need to track config history

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| List view (Backends/Roles/Workflows) | HIGH | LOW | P1 |
| Create new item | HIGH | MEDIUM | P1 |
| Edit existing item | HIGH | MEDIUM | P1 |
| Delete with confirmation | HIGH | LOW | P1 |
| Modal overlay system | HIGH | MEDIUM | P1 |
| Keyboard navigation | HIGH | LOW | P1 |
| Empty state messaging | MEDIUM | LOW | P1 |
| Form validation | HIGH | LOW | P1 |
| Help overlay (?) | HIGH | MEDIUM | P2 |
| Search/filter | MEDIUM | LOW | P2 |
| Multi-step wizard | MEDIUM | MEDIUM | P2 |
| Duplicate/clone | MEDIUM | LOW | P2 |
| Context-aware defaults | MEDIUM | LOW | P2 |
| Live workflow preview | MEDIUM | MEDIUM | P2 |
| Import/export | LOW | MEDIUM | P3 |
| Inline tooltips | LOW | MEDIUM | P3 |
| Undo/redo | MEDIUM | HIGH | P3 |
| Shortcut customization | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch (core CRUD + modal foundation)
- P2: Should have, add when possible (UX enhancements, discoverability)
- P3: Nice to have, future consideration (power user features, scaling features)

## TUI Pattern Reference

### Keyboard Navigation Standards (from k9s, lazygit, vim)

| Key | Expected Behavior | Implementation |
|-----|------------------|----------------|
| j / Down | Move down in list | Bubbles list built-in |
| k / Up | Move up in list | Bubbles list built-in |
| g / Home | Jump to top | Bubbles list built-in |
| G / End | Jump to bottom | Bubbles list built-in |
| Enter | Select/confirm | Custom handler on list item |
| ESC | Cancel/close | Modal visibility toggle |
| ? | Show help | Toggle help modal overlay |
| / | Search/filter | Bubbles list filter mode |
| a | Add new item | Trigger create form modal |
| e | Edit selected | Trigger edit form modal |
| d | Delete selected | Trigger confirmation dialog |
| Tab / Shift-Tab | Navigate form fields | Huh form built-in |
| Space | Toggle selection (multi-select) | Custom handler if needed |
| y | Duplicate/yank (vim pattern) | Copy selected item |

### Form Flow Patterns

**Single-page form (simple items like Backends):**
1. User presses 'a' in list view
2. Modal overlay with Huh form appears
3. User fills fields (Tab to navigate)
4. Enter submits, ESC cancels
5. Modal closes, list refreshes

**Multi-step wizard (complex items like Roles):**
1. User presses 'a' in list view
2. Modal overlay with multi-group Huh form appears
3. Step 1: Select backend type (huh.NewSelect)
4. Step 2: Configure model and provider (huh.NewInput with defaults from step 1)
5. Step 3: Set system prompt (huh.NewText multi-line)
6. Step 4: Select tools (huh.NewMultiSelect)
7. Enter on final step submits, ESC on any step cancels
8. Modal closes, list refreshes

**Edit flow (any item):**
1. User selects item in list, presses 'e'
2. Modal overlay with pre-populated Huh form appears
3. User modifies fields
4. Enter submits changes, ESC cancels
5. Modal closes, list refreshes with updated item

**Delete flow (any item):**
1. User selects item in list, presses 'd'
2. Small confirmation modal appears: "Delete '[name]'? [y/N]"
3. y confirms deletion, n/ESC cancels
4. Modal closes, list refreshes without deleted item

## Component Dependencies on Existing TUI

| New Feature | Existing Component | Dependency Type |
|-------------|-------------------|-----------------|
| List view | Bubbles list component | Use directly |
| Form rendering | Huh library | Already integrated |
| Modal overlay | bubbletea-overlay or custom composite | New dependency (choose one) |
| Settings pane pattern | internal/tui/settings_pane.go | Extend pattern for CRUD modals |
| Config types | internal/config/types.go | Read/write Backend/Role/Workflow |
| Config save | internal/config/save.go | Reuse for new config types |
| Split-pane layout | internal/tui root model | Add modal overlay layer on top |

### Integration Points

1. **Modal overlay layer:** Add above split-pane view, render when modal active
2. **Keybinding routing:** Main model intercepts 's' for settings, add 'b'/'r'/'w' for Backend/Role/Workflow lists
3. **Config mutation:** Use existing config.Save with modified config.OrchestratorConfig
4. **Form validation:** Extend Huh validation to check uniqueness (no duplicate names) and references (workflows reference valid roles)

## Sources

### General TUI Patterns
- [TUI Terminal Tools - Terminal Trove](https://terminaltrove.com/categories/tui/)
- [Navigation Deep Dive | Terminal.Gui v2](https://gui-cs.github.io/Terminal.GuiV2Docs/docs/navigation.html)
- [Keyboard Navigation Patterns for Complex Widgets | UXPin](https://www.uxpin.com/studio/blog/keyboard-navigation-patterns-complex-widgets/)
- [Developing a Keyboard Interface | APG | WAI | W3C](https://www.w3.org/WAI/ARIA/apg/practices/keyboard-interface/)

### Bubble Tea & Charm Libraries
- [bubbletea-overlay package](https://pkg.go.dev/github.com/quickphosphat/bubbletea-overlay)
- [GitHub - charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)
- [Building an Awesome Terminal User Interface Using Go, Bubble Tea, and Lip Gloss | Grootan Technologies](https://www.grootan.com/blogs/building-an-awesome-terminal-user-interface-using-go-bubble-tea-and-lip-gloss/)
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/)
- [huh package - charmbracelet/huh](https://pkg.go.dev/github.com/charmbracelet/huh)
- [GitHub - charmbracelet/huh](https://github.com/charmbracelet/huh)
- [list package - charmbracelet/bubbles/list](https://pkg.go.dev/github.com/charmbracelet/bubbles/list)
- [bubbles/list/README.md at master](https://github.com/charmbracelet/bubbles/blob/master/list/README.md)

### Reference Implementations
- [k9s Hotkeys](https://k9scli.io/topics/hotkeys/)
- [The Complete K9s Cheatsheet](https://ahmedjama.com/blog/2025/09/the-complete-k9s-cheatsheet/)
- [Supercharge Your Git Workflow with Lazygit](https://masri.blog/Blog/Coding/Git/Lazygit-A-TUI-Approach)
- [GitHub - derailed/k9s](https://github.com/derailed/k9s)
- [Vim Cheat Sheet](https://vim.rtorr.com/)
- [GitHub - erikw/vim-keybindings-everywhere-the-ultimate-list](https://github.com/erikw/vim-keybindings-everywhere-the-ultimate-list)

### Form & Validation Patterns
- [10 Design Guidelines for Reporting Errors in Forms - NN/G](https://www.nngroup.com/articles/errors-forms-design-guidelines/)
- [Form Validation Best Practices](https://ivyforms.com/blog/form-validation-best-practices/)
- [Wizard UI Pattern: When to Use It and How to Get It Right](https://www.eleken.co/blog-posts/wizard-ui-pattern-explained)
- [Wizards: Definition and Design Recommendations - NN/G](https://www.nngroup.com/articles/wizards/)
- [How to Design a Form Wizard | Andrew Coyle](https://coyleandrew.medium.com/how-to-design-a-form-wizard-b85fe1cc665a)

### Anti-patterns & Accessibility
- [The text mode lie: why modern TUIs are a nightmare for accessibility](https://xogium.me/the-text-mode-lie-why-modern-tuis-are-a-nightmare-for-accessibility)
- [Terminal.Gui - Cross Platform Terminal UI Toolkit](https://gui-cs.github.io/Terminal.Gui/index.html)

---
*Feature research for: TUI Config Management Dialogs (Backends/Roles/Workflows CRUD)*
*Researched: 2026-02-11*
