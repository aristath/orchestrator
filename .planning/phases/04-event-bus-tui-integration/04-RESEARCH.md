# Phase 4: Event Bus and TUI Integration - Research

**Researched:** 2026-02-10
**Domain:** Terminal UI framework (Bubble Tea), event-driven architecture (Go pubsub), real-time multi-pane display
**Confidence:** HIGH

## Summary

Phase 4 requires building a real-time TUI for monitoring parallel agent execution. The standard stack is **Bubble Tea v2** with **Lip Gloss** for layouts and **Huh** for forms. Multi-pane layouts are achievable through lipgloss layout utilities (`JoinHorizontal`/`JoinVertical`) combined with `viewport` components from the Bubbles library. For event propagation, the idiomatic Go pattern uses **channels with buffering** rather than external event bus libraries—this aligns with the existing codebase's stdlib-first approach.

The critical architectural challenge is **decoupling agent execution from TUI rendering**. The orchestrator's `ParallelRunner` currently executes tasks synchronously within errgroup goroutines. The TUI must observe this execution through an event stream without blocking task execution or overwhelming the render loop.

**Primary recommendation:** Implement a buffered channel-based event bus (`internal/events`) with topic-based subscriptions. Instrument `ParallelRunner` to publish events (TaskStarted, TaskOutput, TaskCompleted, TaskFailed) to the bus. Build the TUI (`internal/tui`) as a separate Bubble Tea program consuming these events through a subscription, using debouncing for high-frequency output streams.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/charmbracelet/bubbletea | v2 | TUI framework | The de facto Go TUI framework based on Elm architecture. Over 10,000 apps built with it. Clean model-update-view pattern. |
| github.com/charmbracelet/bubbles | Latest | Pre-built TUI components | Official component library: viewports, spinners, text inputs, tables, progress bars. |
| github.com/charmbracelet/lipgloss | v2 | Styling and layout | Official layout/styling companion. Provides `JoinHorizontal`/`JoinVertical` for split-pane layouts. |
| github.com/charmbracelet/huh | Latest | Terminal forms | Official form/settings UI library with first-class Bubble Tea integration. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/NimbleMarkets/ntcharts | Latest | Terminal charts | Optional for DAG visualization (bar charts, sparklines for progress). |
| github.com/john-marinelli/panes | Latest | Multi-pane layout component | Alternative to manual lipgloss layout—provides grid-based panes with focus management and vim keybindings. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Bubble Tea | Ratatui (Rust) | Ratatui is more mature for complex layouts but requires Rust bindings or separate process. Rejected—Go-native stack preferred. |
| Channel-based pubsub | github.com/asaskevich/EventBus | External library adds minimal value over stdlib channels. Channel pattern is more idiomatic and testable. |
| Manual layout (lipgloss) | john-marinelli/panes | Panes library provides grid-based structure with focus management. Use if complexity grows beyond 4-5 panes. For initial implementation, lipgloss offers more control. |

**Installation:**
```bash
go get github.com/charmbracelet/bubbletea/v2
go get github.com/charmbracelet/bubbles
go get github.com/charmbracelet/lipgloss/v2
go get github.com/charmbracelet/huh
# Optional
go get github.com/NimbleMarkets/ntcharts
go get github.com/john-marinelli/panes
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── events/           # Event bus (channel-based pubsub)
│   ├── bus.go       # Topic-based event distribution
│   └── types.go     # Event type definitions
├── tui/             # Bubble Tea application
│   ├── model.go     # Root TUI model
│   ├── panes.go     # Individual pane models (agent, DAG, settings)
│   ├── keybindings.go
│   └── styles.go    # Lip Gloss style definitions
└── orchestrator/
    └── runner.go    # Modified to publish events
```

### Pattern 1: Channel-Based Event Bus
**What:** In-memory pubsub using Go channels with topic-based subscriptions. Avoids external dependencies while providing decoupling.

**When to use:** Communicating state changes from orchestrator to TUI without tight coupling.

**Example:**
```go
// Source: https://eli.thegreenplace.net/2020/pubsub-using-channels-in-go/
type EventBus struct {
    mu     sync.RWMutex
    subs   map[string][]chan Event // topic -> subscriber channels
    closed bool
}

func (b *EventBus) Subscribe(topic string) <-chan Event {
    b.mu.Lock()
    defer b.mu.Unlock()

    // Buffered channel prevents slow subscribers from blocking publishers
    ch := make(chan Event, 100)
    b.subs[topic] = append(b.subs[topic], ch)
    return ch
}

func (b *EventBus) Publish(topic string, event Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        return
    }

    for _, ch := range b.subs[topic] {
        // Non-blocking send to prevent slow subscriber from blocking publisher
        select {
        case ch <- event:
        default:
            // Drop message if subscriber is overwhelmed
            // Alternative: use goroutine per send (memory tradeoff)
        }
    }
}

func (b *EventBus) Close() {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.closed {
        return
    }
    b.closed = true

    // Close all subscriber channels to signal shutdown
    for _, subs := range b.subs {
        for _, ch := range subs {
            close(ch)
        }
    }
}
```

### Pattern 2: Split-Pane Layout with Lipgloss
**What:** Use `lipgloss.JoinHorizontal` and `JoinVertical` to compose multi-pane layouts from styled component views.

**When to use:** Building complex TUI layouts with side-by-side and stacked regions.

**Example:**
```go
// Source: https://github.com/charmbracelet/lipgloss/blob/master/examples/layout/main.go
func (m model) View() string {
    // Left pane: Agent list
    agentListView := lipgloss.NewStyle().
        Width(30).
        Height(m.height).
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("63")).
        Render(m.agentList.View())

    // Right pane: Active agent viewport (top) + DAG status (bottom)
    agentOutputView := lipgloss.NewStyle().
        Width(m.width - 32).
        Height(m.height - 10).
        BorderStyle(lipgloss.NormalBorder()).
        Render(m.activeAgentViewport.View())

    dagView := lipgloss.NewStyle().
        Width(m.width - 32).
        Height(8).
        BorderStyle(lipgloss.NormalBorder()).
        Render(m.dagStatus.View())

    // Stack right pane components vertically
    rightPane := lipgloss.JoinVertical(
        lipgloss.Left,
        agentOutputView,
        dagView,
    )

    // Join left and right panes horizontally
    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        agentListView,
        rightPane,
    )
}
```

### Pattern 3: Parent-Child Model Message Routing
**What:** Root model maintains child models (one per pane) and routes messages based on focus state. Each child model handles its own Update/View logic.

**When to use:** Managing multiple interactive components within a single TUI application.

**Example:**
```go
// Source: https://donderom.com/posts/managing-nested-models-with-bubble-tea/
type RootModel struct {
    focusedPane  PaneID
    agentPane    AgentPaneModel
    dagPane      DAGPaneModel
    settingsPane SettingsPaneModel
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle global keys (quit, pane navigation)
        if msg.String() == "ctrl+c" {
            return m, tea.Quit
        }

        // Vim-style pane navigation
        if msg.String() == "ctrl+h" {
            m.focusedPane = PaneLeft
            return m, nil
        }
        // ... ctrl+j/k/l for other directions

        // Route input to focused pane
        switch m.focusedPane {
        case PaneAgent:
            updated, cmd := m.agentPane.Update(msg)
            m.agentPane = updated.(AgentPaneModel)
            return m, cmd
        case PaneDAG:
            updated, cmd := m.dagPane.Update(msg)
            m.dagPane = updated.(DAGPaneModel)
            return m, cmd
        // ... other panes
        }

    case tea.WindowSizeMsg:
        // Broadcast window resize to all panes
        m.agentPane, _ = m.agentPane.Update(msg)
        m.dagPane, _ = m.dagPane.Update(msg)
        m.settingsPane, _ = m.settingsPane.Update(msg)
    }

    return m, nil
}
```

### Pattern 4: Debouncing High-Frequency Output
**What:** Use tag-based message filtering with `tea.Tick()` to coalesce rapid updates into periodic renders.

**When to use:** Agent output streams produce messages faster than TUI can reasonably render (>60 msgs/sec).

**Example:**
```go
// Source: https://github.com/charmbracelet/bubbletea/blob/main/examples/debounce/main.go
type AgentOutputModel struct {
    buffer      []string
    updateTag   int
    lastRender  time.Time
}

type outputDebounceMsg int

func (m AgentOutputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case AgentOutputEvent:
        // Append to buffer
        m.buffer = append(m.buffer, msg.Line)

        // Increment tag to invalidate previous debounce timers
        m.updateTag++

        // Schedule render after 100ms
        tag := m.updateTag
        return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
            return outputDebounceMsg(tag)
        })

    case outputDebounceMsg:
        // Only render if tag matches (no newer updates queued)
        if int(msg) == m.updateTag {
            m.lastRender = time.Now()
            return m, nil
        }
        // Stale message, ignore
        return m, nil
    }

    return m, nil
}
```

### Pattern 5: Settings Panel with Huh Forms
**What:** Embed `huh.Form` in a settings pane model. Forms are Bubble Tea models themselves, so they integrate via standard Update/View delegation.

**When to use:** Building config editors within the TUI.

**Example:**
```go
// Source: https://github.com/charmbracelet/huh official docs
type SettingsPaneModel struct {
    form *huh.Form
}

func NewSettingsPane(cfg *config.Config) SettingsPaneModel {
    form := huh.NewForm(
        huh.NewGroup(
            huh.NewInput().
                Key("claude_model").
                Title("Claude Model").
                Value(&cfg.Agents["claude"].Model).
                Validate(func(s string) error {
                    if s == "" {
                        return errors.New("model cannot be empty")
                    }
                    return nil
                }),

            huh.NewSelect[string]().
                Key("merge_strategy").
                Title("Merge Strategy").
                Options(
                    huh.NewOption("Fast-forward", "ff"),
                    huh.NewOption("No fast-forward", "no-ff"),
                    huh.NewOption("Squash", "squash"),
                ).
                Value(&cfg.Project.MergeStrategy),
        ),
    )

    return SettingsPaneModel{form: form}
}

func (m SettingsPaneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    form, cmd := m.form.Update(msg)
    if f, ok := form.(*huh.Form); ok {
        m.form = f
    }

    // Check if form completed
    if m.form.State == huh.StateCompleted {
        // Save config values (already updated via pointers)
        // Signal parent model to persist config to disk
    }

    return m, cmd
}
```

### Anti-Patterns to Avoid

- **Synchronous event dispatch:** Do NOT block task execution goroutines to send events. Use buffered channels with non-blocking sends.
- **Unbuffered subscriber channels:** Slow TUI rendering can block the event bus. Always use buffered channels (recommended: 100-500 message buffer).
- **Direct backend coupling:** Do NOT pass TUI references into `ParallelRunner`. Use event bus as intermediary.
- **String-based state management:** Bubble Tea v2 uses declarative `tea.View` structs, not string returns. Avoid v1 patterns.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Multi-pane layout | Custom ANSI escape code renderer | lipgloss `JoinHorizontal`/`JoinVertical` or `panes` library | Terminal dimensions, borders, alignment, and dynamic resize are deceptively complex. Lipgloss handles terminal capabilities (color downsampling, width calculation). |
| Scrollable viewports | Custom scroll buffer with bounds checking | `bubbles/viewport` | Mouse wheel support, keyboard navigation, and content overflow are thoroughly tested in bubbles. Reimplementing introduces edge cases. |
| Form validation | String parsing with manual error display | `huh` forms with `.Validate()` callbacks | Field focus management, error rendering, and multi-step forms are non-trivial. Huh provides accessible, tested patterns. |
| Event distribution | Custom goroutine pool with channel fan-out | Channel-based pubsub pattern | Topic-based subscriptions, safe shutdown (channel closure), and subscriber management require careful synchronization. Eli Bendersky's pattern is battle-tested. |
| Keyboard input parsing | Raw terminal mode with termbox/tcell | Bubble Tea's key handling | Bubble Tea v2 detects modern terminal capabilities (shift+enter, ctrl+m, key releases). Parsing this manually misses edge cases across terminal emulators. |

**Key insight:** Terminal UIs involve platform-specific quirks (terminal emulator differences, SSH rendering constraints, color support detection). The Charm ecosystem (Bubble Tea, Lipgloss, Bubbles, Huh) abstracts these complexities. Reimplementing basic primitives leads to cross-platform bugs and maintenance burden.

## Common Pitfalls

### Pitfall 1: Unbuffered Event Channels Blocking Task Execution
**What goes wrong:** If the event bus uses unbuffered channels or synchronous sends, a slow TUI (e.g., rendering large output) blocks `ParallelRunner.executeTask()` goroutines, stalling task execution.

**Why it happens:** Go's channels block when full. Without buffering, `eventBus.Publish()` waits for a receiver to consume the message. If the TUI's Bubble Tea event loop is busy rendering, it doesn't read from the subscription channel quickly enough.

**How to avoid:**
1. Use buffered channels for subscriptions (recommended: 100-500 messages)
2. Use non-blocking sends in `Publish()` with a `select/default` fallback
3. Drop messages if the buffer is full (for high-frequency output) OR spawn goroutines per send (memory tradeoff)

**Warning signs:** Task execution pauses when TUI is rendering. Goroutine profiles show publisher goroutines blocked on channel sends.

### Pitfall 2: TUI State Desync from Orchestrator State
**What goes wrong:** TUI displays "Task Running" but orchestrator already marked it completed. Or DAG shows 3/5 tasks complete but TUI shows 2/5.

**Why it happens:** Race condition between event publication and TUI event processing. Or lost events due to channel buffer overflow.

**How to avoid:**
1. Make event bus the single source of truth—orchestrator publishes ALL state transitions
2. On TUI startup, query orchestrator for initial state snapshot, THEN subscribe to events
3. Use monotonic counters or sequence IDs in events to detect dropped messages
4. For critical state (task status), use authoritative events (TaskCompleted) instead of inferring from output logs

**Warning signs:** TUI shows stale data after resuming from pause. DAG progress bar doesn't match number of completed tasks.

### Pitfall 3: WindowSizeMsg Not Broadcast to All Child Models
**What goes wrong:** After terminal resize, some panes render with old dimensions, causing truncated text or broken borders.

**Why it happens:** Bubble Tea sends `tea.WindowSizeMsg` to root model. If root model doesn't forward it to ALL child models, they retain stale width/height values.

**How to avoid:**
1. In root model's `Update()`, explicitly pass `tea.WindowSizeMsg` to every child model (agent pane, DAG pane, settings pane)
2. Each child model must store `width` and `height` fields and update them on `tea.WindowSizeMsg`
3. Use `lipgloss.Width()`/`lipgloss.Height()` to calculate component dimensions dynamically, not hardcoded values

**Warning signs:** After resizing terminal, panes have misaligned borders or content overflows. Running `stty size` shows correct dimensions but TUI renders incorrectly.

### Pitfall 4: High-Frequency Agent Output Overwhelming Render Loop
**What goes wrong:** Agent produces 1000 log lines/second. TUI becomes unresponsive or stutters. Terminal output lags 30+ seconds behind real time.

**Why it happens:** Bubble Tea processes messages sequentially through `Update()`. If agent publishes one event per line, TUI spends all CPU time updating the model and rendering, with no time for user input.

**How to avoid:**
1. Batch agent output events (collect 10-50 lines, publish once per batch)
2. Debounce viewport updates using tag-based pattern (render at most 10-20 times/sec)
3. Implement backpressure: if event channel is 80% full, skip publishing low-priority events (verbose logs)
4. Use `viewport.SetContent()` with pre-formatted strings instead of appending per-line

**Warning signs:** TUI doesn't respond to keystrokes during heavy output. `top` shows 100% CPU on TUI process. Terminal becomes "laggy."

### Pitfall 5: Focus Management Desync in Multi-Pane Layout
**What goes wrong:** User presses Ctrl+L to switch focus right, but TUI input still goes to left pane. Or status indicators don't show which pane is focused.

**Why it happens:** Root model updates `focusedPane` state but doesn't notify child models to blur/focus. Or child models don't re-render to show focus change.

**How to avoid:**
1. When focus changes, send explicit `FocusMsg`/`BlurMsg` to old and new pane models
2. Each pane model renders differently when focused (e.g., brighter border, cursor visible)
3. Alternative: use `panes` library which handles focus transitions via `In()`/`Out()` methods
4. Ensure root model's `View()` renders all panes (not just focused one) so focus changes are visible

**Warning signs:** User navigates between panes but input doesn't route correctly. Visual focus indicator (border color) doesn't update.

## Code Examples

Verified patterns from official sources:

### Event Bus Subscription and Non-Blocking Publish
```go
// Source: https://eli.thegreenplace.net/2020/pubsub-using-channels-in-go/
// Production-ready pattern with proper shutdown

type EventBus struct {
    mu     sync.RWMutex
    subs   map[string][]chan Event
    closed bool
}

func NewEventBus() *EventBus {
    return &EventBus{
        subs: make(map[string][]chan Event),
    }
}

func (b *EventBus) Subscribe(topic string) <-chan Event {
    b.mu.Lock()
    defer b.mu.Unlock()

    // Buffer size prevents slow subscribers from blocking
    ch := make(chan Event, 100)
    b.subs[topic] = append(b.subs[topic], ch)
    return ch
}

func (b *EventBus) Publish(topic string, event Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        return
    }

    for _, ch := range b.subs[topic] {
        // Non-blocking send to prevent one slow subscriber from blocking others
        select {
        case ch <- event:
        default:
            // Channel full - drop message or spawn goroutine
            // For production: log dropped message and consider increasing buffer
        }
    }
}

func (b *EventBus) Close() {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.closed {
        return
    }
    b.closed = true

    // Close all channels to signal subscribers to stop
    for _, subs := range b.subs {
        for _, ch := range subs {
            close(ch)
        }
    }
}
```

### Bubble Tea Vim-Style Navigation with Focus Management
```go
// Source: Community pattern (https://github.com/john-marinelli/panes)
// Adapted for custom implementation

type PaneID int

const (
    PaneAgentList PaneID = iota
    PaneAgentOutput
    PaneDAGStatus
    PaneSettings
)

type RootModel struct {
    focusedPane PaneID
    panes       map[PaneID]tea.Model
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Vim-style pane navigation (Ctrl+hjkl)
        switch msg.String() {
        case "ctrl+h":
            m.focusedPane = PaneAgentList
            return m, nil
        case "ctrl+l":
            m.focusedPane = PaneAgentOutput
            return m, nil
        case "ctrl+j":
            m.focusedPane = PaneDAGStatus
            return m, nil
        case "ctrl+k":
            m.focusedPane = PaneSettings
            return m, nil
        }

        // Route to focused pane
        if pane, ok := m.panes[m.focusedPane]; ok {
            updated, cmd := pane.Update(msg)
            m.panes[m.focusedPane] = updated
            return m, cmd
        }
    }

    return m, nil
}
```

### Integrating Event Stream into Bubble Tea Model
```go
// Pattern for consuming event bus in TUI

type AgentPaneModel struct {
    agents       map[string]*AgentState
    eventSub     <-chan Event
}

func (m AgentPaneModel) Init() tea.Cmd {
    // Return command to listen for events
    return waitForEvent(m.eventSub)
}

func waitForEvent(sub <-chan Event) tea.Cmd {
    return func() tea.Msg {
        event, ok := <-sub
        if !ok {
            // Channel closed, event bus shut down
            return nil
        }
        return event
    }
}

func (m AgentPaneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case TaskStartedEvent:
        m.agents[msg.TaskID] = &AgentState{
            Status: "running",
            Output: []string{},
        }
        // Continue listening
        return m, waitForEvent(m.eventSub)

    case TaskOutputEvent:
        if agent, ok := m.agents[msg.TaskID]; ok {
            agent.Output = append(agent.Output, msg.Line)
        }
        return m, waitForEvent(m.eventSub)

    case TaskCompletedEvent:
        if agent, ok := m.agents[msg.TaskID]; ok {
            agent.Status = "completed"
        }
        return m, waitForEvent(m.eventSub)
    }

    return m, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Bubble Tea v1 (string-based View) | Bubble Tea v2 (declarative tea.View struct) | Late 2025 | View() now returns structured metadata (cursor position, window title, mouse settings) instead of strings. Prevents state desync between commands and rendering. |
| tmux/screen for multi-pane TUIs | Native Bubble Tea with lipgloss layouts | 2024-2025 | No external multiplexer dependency. Simpler deployment (single binary). |
| Manual ANSI escape codes | Lip Gloss styling with automatic color downsampling | 2023-present | Lipgloss detects terminal capabilities and downgrades colors (true color → 256 → 16) automatically. |
| Callbacks for async operations | tea.Cmd pattern (return functions, not mutate state) | Since Bubble Tea v0 | Enforces functional updates. Prevents race conditions. Easier testing. |
| Mode 2026 (synchronized updates) off by default | Mode 2026 enabled automatically in Bubble Tea v2 | Late 2025 | Reduces flicker and tearing during rendering. Better for SSH/high-latency connections. |

**Deprecated/outdated:**
- **Bubble Tea v1 import path** (`github.com/charmbracelet/bubbletea`): Use v2 vanity domain `charm.land/bubbletea/v2`
- **Key.Type**: Renamed to `Key.Code` in v2
- **Key.Runes**: Renamed to `Key.Text` in v2
- **Paste events in KeyMsg**: Now separate `tea.PasteMsg`/`tea.PasteStartMsg`/`tea.PasteEndMsg`
- **EventBus external libraries**: Idiomatic Go now uses channels directly. Libraries like `asaskevich/EventBus` add minimal value for in-process pubsub.

## Open Questions

1. **Should we batch agent output events or use per-line granularity?**
   - What we know: Per-line events provide real-time updates but may overwhelm TUI at >100 lines/sec
   - What's unclear: Acceptable latency for output display (100ms? 500ms?). User expectation for "real-time."
   - Recommendation: Start with per-line events + debouncing (100ms). Profile under load. Switch to batching if TUI becomes unresponsive.

2. **How to handle DAG visualization for 50+ task graphs?**
   - What we know: ASCII DAG rendering (like Beads Viewer) is complex. Requirements specify "overall DAG progress," not full graph visualization.
   - What's unclear: Is a progress summary sufficient (5/50 tasks complete), or do users need dependency tree visualization?
   - Recommendation: Phase 4 implements summary view (task counts, status distribution). Defer full DAG visualization to Phase 6 if user feedback demands it.

3. **Should settings changes apply immediately or require restart?**
   - What we know: Huh forms can update config struct pointers in-place. Some settings (model, provider) require backend recreation.
   - What's unclear: User expectation—save config to disk immediately, or stage changes until "Apply" button?
   - Recommendation: Immediate save to disk, but warn that some changes (backend configs) require orchestrator restart. Phase 4 focuses on editing; hot-reload is out of scope.

4. **Event bus lifecycle: start before or after TUI initialization?**
   - What we know: TUI needs event subscription before `ParallelRunner.Run()` starts publishing. But event bus lifetime exceeds TUI (orchestrator may run headless).
   - What's unclear: Who owns event bus lifecycle? Main function, orchestrator, or TUI?
   - Recommendation: Main function creates event bus and passes it to both orchestrator and TUI. Event bus lifetime spans entire program. TUI subscribes in `Init()`, unsubscribes on quit.

## Sources

### Primary (HIGH confidence)
- Bubble Tea v2 upgrade guide: https://github.com/charmbracelet/bubbletea/discussions/1374
- Lipgloss layout example: https://github.com/charmbracelet/lipgloss/blob/master/examples/layout/main.go
- Bubble Tea debounce pattern: https://github.com/charmbracelet/bubbletea/blob/main/examples/debounce/main.go
- Go channel-based pubsub (Eli Bendersky): https://eli.thegreenplace.net/2020/pubsub-using-channels-in-go/
- Bubbles component library: https://github.com/charmbracelet/bubbles
- Huh forms integration: https://github.com/charmbracelet/huh

### Secondary (MEDIUM confidence)
- Tips for building Bubble Tea programs: https://leg100.github.io/en/posts/building-bubbletea-programs/
- Managing nested models: https://donderom.com/posts/managing-nested-models-with-bubble-tea/
- Panes multi-pane component: https://github.com/john-marinelli/panes
- TmuxCC agent monitoring architecture: https://github.com/nyanko3141592/tmuxcc
- ntcharts terminal charts: https://github.com/NimbleMarkets/ntcharts
- Testing Bubble Tea interfaces: https://carlosbecker.com/posts/teatest/

### Tertiary (LOW confidence)
- Go event bus libraries (needs verification): https://github.com/simonfxr/pubsub, https://github.com/asaskevich/EventBus
- Beads Viewer DAG visualization (reference only): https://github.com/Dicklesworthstone/beads_viewer

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Bubble Tea is the established Go TUI framework. Official Charm libraries (lipgloss, bubbles, huh) are documented and widely used.
- Architecture (event bus): HIGH - Channel-based pubsub is verified pattern from authoritative source (Eli Bendersky). Existing codebase uses stdlib patterns, this aligns.
- Architecture (multi-pane layout): MEDIUM - Lipgloss layout utilities are official, but complex layouts require combining techniques. Panes library is LOW maturity (14 stars, 5 commits).
- Pitfalls: MEDIUM - Derived from community articles and WebSearch findings. Not all scenarios verified in production Bubble Tea apps.

**Research date:** 2026-02-10
**Valid until:** 2026-03-12 (30 days - Bubble Tea v2 is stable, but ecosystem evolves rapidly)
