package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aristath/orchestrator/internal/config"
	"github.com/aristath/orchestrator/internal/events"
)

// PaneID identifies which pane is focused.
type PaneID int

const (
	PaneAgentList PaneID = iota
	PaneAgentOutput
	PaneDAG
)

// Model is the root Bubble Tea model for the TUI.
type Model struct {
	agentPane        AgentPaneModel
	dagPane          DAGPaneModel
	settingsPane     SettingsPaneModel
	focusedPane      PaneID
	eventSub         <-chan events.Event
	width            int
	height           int
	quitting         bool
	showSettings     bool
	config           *config.OrchestratorConfig
	globalConfigPath string
	projectConfigPath string
}

// New creates a new TUI model.
// It subscribes to all events from the event bus using SubscribeAll.
func New(eventBus *events.EventBus, cfg *config.OrchestratorConfig, globalPath, projectPath string) Model {
	return Model{
		agentPane:         NewAgentPaneModel(),
		dagPane:           NewDAGPaneModel(),
		settingsPane:      NewSettingsPaneModel(cfg, globalPath, projectPath),
		focusedPane:       PaneAgentList,
		eventSub:          eventBus.SubscribeAll(256),
		showSettings:      false,
		config:            cfg,
		globalConfigPath:  globalPath,
		projectConfigPath: projectPath,
	}
}

// Init initializes the model and returns the initial command.
func (m Model) Init() tea.Cmd {
	return waitForEvent(m.eventSub)
}

// waitForEvent returns a command that waits for the next event from the event bus.
func waitForEvent(sub <-chan events.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-sub
		if !ok {
			return nil // bus closed
		}
		return event
	}
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If settings panel is open, route all keys to it (modal behavior)
		if m.showSettings {
			switch msg.String() {
			case "s", "esc":
				// Toggle settings off
				m.showSettings = false
				m.settingsPane.SetVisible(false)
			default:
				// Route to settings pane
				var cmd tea.Cmd
				m.settingsPane, cmd = m.settingsPane.Update(msg)
				cmds = append(cmds, cmd)

				// Check if settings pane closed itself (after save)
				if !m.settingsPane.IsVisible() {
					m.showSettings = false
				}
			}
			return m, tea.Batch(cmds...)
		}

		// Normal mode (settings not open)
		switch msg.String() {
		case KeyQuit, KeyCtrlC:
			m.quitting = true
			return m, tea.Quit

		case "s":
			// Toggle settings on
			m.showSettings = true
			m.settingsPane.SetVisible(true)
			var cmd tea.Cmd
			cmd = m.settingsPane.Init()
			cmds = append(cmds, cmd)

		case KeyTab:
			// Cycle forward
			m.focusedPane = (m.focusedPane + 1) % 3
			m.updateFocusStates()

		case KeyShiftTab:
			// Cycle backward
			m.focusedPane = (m.focusedPane + 2) % 3 // +2 is equivalent to -1 mod 3
			m.updateFocusStates()

		case KeyPane1:
			m.focusedPane = PaneAgentList
			m.updateFocusStates()

		case KeyPane2:
			m.focusedPane = PaneAgentOutput
			m.updateFocusStates()

		case KeyPane3:
			m.focusedPane = PaneDAG
			m.updateFocusStates()

		default:
			// Delegate to focused pane
			switch m.focusedPane {
			case PaneAgentList, PaneAgentOutput:
				var cmd tea.Cmd
				m.agentPane, cmd = m.agentPane.Update(msg)
				cmds = append(cmds, cmd)
			case PaneDAG:
				var cmd tea.Cmd
				m.dagPane, cmd = m.dagPane.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.computeLayout()
		m.settingsPane.SetSize(msg.Width, msg.Height)

	case events.TaskStartedEvent, events.TaskOutputEvent, events.TaskCompletedEvent, events.TaskFailedEvent:
		// Forward task events to agent pane
		var cmd tea.Cmd
		m.agentPane, cmd = m.agentPane.Update(msg)
		cmds = append(cmds, cmd)
		// Also wait for next event
		cmds = append(cmds, waitForEvent(m.eventSub))

	case events.DAGProgressEvent:
		// Forward DAG events to DAG pane
		var cmd tea.Cmd
		m.dagPane, cmd = m.dagPane.Update(msg)
		cmds = append(cmds, cmd)
		// Also wait for next event
		cmds = append(cmds, waitForEvent(m.eventSub))

	case events.TaskMergedEvent:
		// Currently not displayed, but consume and wait for next event
		cmds = append(cmds, waitForEvent(m.eventSub))
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// If settings panel is visible, render it as overlay
	if m.showSettings {
		// Render settings pane centered over the normal view
		// For simplicity, render full-screen settings view
		return m.settingsPane.View()
	}

	// Compute layout dimensions
	leftWidth := (m.width * 35) / 100       // 35% for agent pane
	rightWidth := m.width - leftWidth       // 65% for right side
	availableHeight := m.height - 1         // reserve 1 line for help bar
	rightTopHeight := (availableHeight * 70) / 100  // 70% for agent output
	_ = rightTopHeight // Currently unused, will be used in future for separate agent output pane

	// Render left pane (agent list + output combined)
	leftPane := m.agentPane.View()

	// Render right-top pane (currently empty, will show DAG visualization)
	// For now, just create an empty styled box as placeholder
	rightTopStyle := StyleUnfocusedBorder
	rightTop := rightTopStyle.
		Width(rightWidth - 2).
		Height((availableHeight*70)/100 - 2).
		Render("Agent Output (shown in left pane)")

	// Render right-bottom pane (DAG progress)
	rightBottom := m.dagPane.View()

	// Join right panes vertically
	rightPane := lipgloss.JoinVertical(lipgloss.Left, rightTop, rightBottom)

	// Join left and right horizontally
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// Add help bar at bottom
	helpBar := HelpView()

	// Join main content and help bar vertically
	return lipgloss.JoinVertical(lipgloss.Left, mainContent, helpBar)
}

// computeLayout calculates pane dimensions and updates all child models.
func (m *Model) computeLayout() {
	leftWidth := (m.width * 35) / 100
	rightWidth := m.width - leftWidth
	availableHeight := m.height - 1
	rightTopHeight := (availableHeight * 70) / 100
	rightBottomHeight := availableHeight - rightTopHeight

	// Agent pane takes full left side
	m.agentPane.SetSize(leftWidth, availableHeight)

	// DAG pane takes right-bottom
	m.dagPane.SetSize(rightWidth, rightBottomHeight)

	m.updateFocusStates()
}

// updateFocusStates updates the focus state of all panes.
func (m *Model) updateFocusStates() {
	// For now, agent pane is focused for both AgentList and AgentOutput
	m.agentPane.SetFocused(m.focusedPane == PaneAgentList || m.focusedPane == PaneAgentOutput)
	m.dagPane.SetFocused(m.focusedPane == PaneDAG)
}
