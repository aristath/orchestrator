package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aristath/orchestrator/internal/events"
)

// DAGPaneModel represents the DAG progress display pane.
type DAGPaneModel struct {
	total     int
	completed int
	running   int
	failed    int
	pending   int
	width     int
	height    int
	focused   bool
}

// NewDAGPaneModel creates a new DAG pane model.
func NewDAGPaneModel() DAGPaneModel {
	return DAGPaneModel{}
}

// Update handles messages for the DAG pane.
func (m DAGPaneModel) Update(msg tea.Msg) (DAGPaneModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case events.DAGProgressEvent:
		m.total = msg.Total
		m.completed = msg.Completed
		m.running = msg.Running
		m.failed = msg.Failed
		m.pending = msg.Pending
	}

	return m, nil
}

// View renders the DAG pane.
func (m DAGPaneModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var b strings.Builder

	// Title
	title := StyleTitle.Render("DAG Progress")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", lipgloss.Width(title)))
	b.WriteString("\n\n")

	// Counts
	b.WriteString(fmt.Sprintf("Total:     %d\n", m.total))
	b.WriteString(fmt.Sprintf("Completed: %s\n", StyleStatusComplete.Render(fmt.Sprintf("%d", m.completed))))
	b.WriteString(fmt.Sprintf("Running:   %s\n", StyleStatusRunning.Render(fmt.Sprintf("%d", m.running))))
	b.WriteString(fmt.Sprintf("Failed:    %s\n", StyleStatusFailed.Render(fmt.Sprintf("%d", m.failed))))
	b.WriteString(fmt.Sprintf("Pending:   %s\n", StyleStatusPending.Render(fmt.Sprintf("%d", m.pending))))

	b.WriteString("\n")

	// Progress bar
	if m.total > 0 {
		barWidth := min(m.width-4, 40)
		completedWidth := (m.completed * barWidth) / m.total
		failedWidth := (m.failed * barWidth) / m.total
		runningWidth := (m.running * barWidth) / m.total
		pendingWidth := barWidth - completedWidth - failedWidth - runningWidth

		bar := StyleStatusComplete.Render(strings.Repeat("=", max(0, completedWidth)))
		bar += StyleStatusFailed.Render(strings.Repeat("!", max(0, failedWidth)))
		bar += StyleStatusRunning.Render(strings.Repeat("-", max(0, runningWidth)))
		bar += StyleStatusPending.Render(strings.Repeat(".", max(0, pendingWidth)))

		b.WriteString(fmt.Sprintf("[%s]  %d/%d\n", bar, m.completed, m.total))
	}

	content := b.String()

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

// SetSize updates the pane dimensions.
func (m *DAGPaneModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetFocused updates the focus state.
func (m *DAGPaneModel) SetFocused(focused bool) {
	m.focused = focused
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
