package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/aristath/orchestrator/internal/events"
)

// AgentState represents the state of a single agent/task.
type AgentState struct {
	TaskID    string
	Name      string
	AgentRole string
	Status    string // "running", "completed", "failed"
	Output    []string
	StartTime time.Time
	Duration  time.Duration
}

// AgentPaneModel represents the agent list and output viewport pane.
type AgentPaneModel struct {
	agents      map[string]*AgentState // taskID -> state
	agentOrder  []string               // insertion order for display
	selectedIdx int                    // which agent is selected in list
	viewport    viewport.Model         // scrollable output viewport
	width       int
	height      int
	focused     bool
	updateTag   int // for debouncing
}

// NewAgentPaneModel creates a new agent pane model.
func NewAgentPaneModel() AgentPaneModel {
	vp := viewport.New(0, 0)
	return AgentPaneModel{
		agents:   make(map[string]*AgentState),
		viewport: vp,
	}
}

// tickMsg is used for debouncing viewport updates.
type tickMsg struct {
	tag int
}

// Update handles messages for the agent pane.
func (m AgentPaneModel) Update(msg tea.Msg) (AgentPaneModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()

	case tea.KeyMsg:
		if !m.focused {
			break
		}

		switch msg.String() {
		case KeyJ, KeyDown:
			if m.selectedIdx < len(m.agentOrder)-1 {
				m.selectedIdx++
				m.updateViewportContent()
			}
		case KeyK, KeyUp:
			if m.selectedIdx > 0 {
				m.selectedIdx--
				m.updateViewportContent()
			}
		default:
			// Delegate other keys to viewport for scrolling
			m.viewport, cmd = m.viewport.Update(msg)
		}

	case events.TaskStartedEvent:
		// Add new agent
		if _, exists := m.agents[msg.ID]; !exists {
			m.agents[msg.ID] = &AgentState{
				TaskID:    msg.ID,
				Name:      msg.Name,
				AgentRole: msg.AgentRole,
				Status:    "running",
				Output:    make([]string, 0),
				StartTime: msg.Timestamp,
			}
			m.agentOrder = append(m.agentOrder, msg.ID)
			// Auto-select first agent
			if len(m.agentOrder) == 1 {
				m.selectedIdx = 0
				m.updateViewportContent()
			}
		}

	case events.TaskOutputEvent:
		// Append output to agent
		if agent, exists := m.agents[msg.ID]; exists {
			agent.Output = append(agent.Output, msg.Line)
			// If this is the selected agent, update viewport with debouncing
			if m.getSelectedTaskID() == msg.ID {
				m.updateTag++
				tag := m.updateTag
				return m, tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
					return tickMsg{tag: tag}
				})
			}
		}

	case events.TaskCompletedEvent:
		if agent, exists := m.agents[msg.ID]; exists {
			agent.Status = "completed"
			agent.Duration = msg.Duration
			agent.Output = append(agent.Output, fmt.Sprintf("\n[Completed in %v]", msg.Duration))
			if m.getSelectedTaskID() == msg.ID {
				m.updateViewportContent()
			}
		}

	case events.TaskFailedEvent:
		if agent, exists := m.agents[msg.ID]; exists {
			agent.Status = "failed"
			agent.Duration = msg.Duration
			agent.Output = append(agent.Output, fmt.Sprintf("\n[Failed: %v]", msg.Err))
			if m.getSelectedTaskID() == msg.ID {
				m.updateViewportContent()
			}
		}

	case tickMsg:
		// Only update if this tick matches the current tag (debouncing)
		if msg.tag == m.updateTag {
			m.updateViewportContent()
		}
	}

	return m, cmd
}

// View renders the agent pane.
func (m AgentPaneModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Split into two columns: agent list (left) and viewport (right)
	listWidth := 25
	viewportWidth := m.width - listWidth - 4 // account for borders and padding

	// Render agent list
	listContent := m.renderAgentList(listWidth)

	// Render viewport
	viewportContent := m.viewport.View()

	// Join horizontally
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		listContent,
		lipgloss.NewStyle().
			Width(viewportWidth).
			Height(m.height-2).
			Render(viewportContent),
	)

	// Apply border style
	style := StyleUnfocusedBorder
	if m.focused {
		style = StyleFocusedBorder
	}

	return style.
		Width(m.width - 2).
		Height(m.height - 2).
		Render(content)
}

// renderAgentList renders the agent list column.
func (m AgentPaneModel) renderAgentList(width int) string {
	var b strings.Builder

	title := StyleTitle.Render("Agents")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", min(width, lipgloss.Width(title))))
	b.WriteString("\n\n")

	if len(m.agentOrder) == 0 {
		b.WriteString(StyleStatusPending.Render("Waiting..."))
	} else {
		for i, taskID := range m.agentOrder {
			agent := m.agents[taskID]
			icon := m.StatusIcon(agent.Status)
			name := agent.Name
			if len(name) > width-6 {
				name = name[:width-9] + "..."
			}

			line := fmt.Sprintf("%s %s", icon, name)
			if i == m.selectedIdx {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("62")).
					Foreground(lipgloss.Color("0")).
					Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(m.height - 2).
		Render(b.String())
}

// StatusIcon returns a styled status indicator.
func (m AgentPaneModel) StatusIcon(status string) string {
	switch status {
	case "running":
		return StyleStatusRunning.Render("●")
	case "completed":
		return StyleStatusComplete.Render("✓")
	case "failed":
		return StyleStatusFailed.Render("✗")
	default:
		return StyleStatusPending.Render("○")
	}
}

// getSelectedTaskID returns the task ID of the currently selected agent.
func (m AgentPaneModel) getSelectedTaskID() string {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.agentOrder) {
		return m.agentOrder[m.selectedIdx]
	}
	return ""
}

// updateViewportContent updates the viewport with the selected agent's output.
func (m *AgentPaneModel) updateViewportContent() {
	taskID := m.getSelectedTaskID()
	if taskID == "" {
		m.viewport.SetContent("Waiting for tasks...")
		return
	}

	agent, exists := m.agents[taskID]
	if !exists {
		m.viewport.SetContent("Waiting for tasks...")
		return
	}

	content := strings.Join(agent.Output, "\n")
	m.viewport.SetContent(content)
	// Auto-scroll to bottom
	m.viewport.GotoBottom()
}

// resizeViewport resizes the viewport based on pane dimensions.
func (m *AgentPaneModel) resizeViewport() {
	listWidth := 25
	viewportWidth := m.width - listWidth - 4
	viewportHeight := m.height - 4 // account for borders

	if viewportWidth < 10 {
		viewportWidth = 10
	}
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	m.viewport.Width = viewportWidth
	m.viewport.Height = viewportHeight
}

// SetSize updates the pane dimensions.
func (m *AgentPaneModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.resizeViewport()
}

// SetFocused updates the focus state.
func (m *AgentPaneModel) SetFocused(focused bool) {
	m.focused = focused
}
