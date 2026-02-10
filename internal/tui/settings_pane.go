package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/aristath/orchestrator/internal/config"
)

// SettingsPaneModel manages the settings form overlay.
type SettingsPaneModel struct {
	form        *huh.Form
	config      *config.OrchestratorConfig
	savePath    string // "global" or "project"
	globalPath  string
	projectPath string
	width       int
	height      int
	visible     bool
	saved       bool
	err         error

	// Form field bindings (strings for Huh)
	saveTarget     string
	coderProvider  string
	coderModel     string
	reviewProvider string
	reviewModel    string
	claudeCommand  string
	codexCommand   string
	gooseCommand   string
}

// NewSettingsPaneModel creates a new settings pane.
func NewSettingsPaneModel(cfg *config.OrchestratorConfig, globalPath, projectPath string) SettingsPaneModel {
	m := SettingsPaneModel{
		config:      cfg,
		globalPath:  globalPath,
		projectPath: projectPath,
		visible:     false,
		saved:       false,

		// Initialize form field values from config
		saveTarget:     "global",
		coderProvider:  cfg.Agents["coder"].Provider,
		coderModel:     cfg.Agents["coder"].Model,
		reviewProvider: cfg.Agents["reviewer"].Provider,
		reviewModel:    cfg.Agents["reviewer"].Model,
		claudeCommand:  cfg.Providers["claude"].Command,
		codexCommand:   cfg.Providers["codex"].Command,
		gooseCommand:   cfg.Providers["goose"].Command,
	}

	m.buildForm()
	return m
}

// buildForm constructs the Huh form with all settings fields.
func (m *SettingsPaneModel) buildForm() {
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Key("saveTarget").
				Title("Save To").
				Options(
					huh.NewOption("Global (~/.orchestrator/config.json)", "global"),
					huh.NewOption("Project (.orchestrator/config.json)", "project"),
				).
				Value(&m.saveTarget),
		).Title("Save Target"),

		huh.NewGroup(
			huh.NewInput().
				Key("coderProvider").
				Title("Coder Provider").
				Value(&m.coderProvider).
				Placeholder("claude"),

			huh.NewInput().
				Key("coderModel").
				Title("Coder Model").
				Value(&m.coderModel).
				Placeholder("opus-4"),

			huh.NewInput().
				Key("reviewProvider").
				Title("Reviewer Provider").
				Value(&m.reviewProvider).
				Placeholder("claude"),

			huh.NewInput().
				Key("reviewModel").
				Title("Reviewer Model").
				Value(&m.reviewModel).
				Placeholder("sonnet-4"),
		).Title("Default Agent Settings"),

		huh.NewGroup(
			huh.NewInput().
				Key("claudeCommand").
				Title("Claude Command").
				Value(&m.claudeCommand).
				Placeholder("claude"),

			huh.NewInput().
				Key("codexCommand").
				Title("Codex Command").
				Value(&m.codexCommand).
				Placeholder("codex"),

			huh.NewInput().
				Key("gooseCommand").
				Title("Goose Command").
				Value(&m.gooseCommand).
				Placeholder("goose"),
		).Title("Provider Settings"),
	)
}

// Init initializes the settings pane.
func (m SettingsPaneModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles messages for the settings pane.
func (m SettingsPaneModel) Update(msg tea.Msg) (SettingsPaneModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancel without saving
			m.visible = false
			m.saved = false
			return m, nil
		}
	}

	// Delegate to form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	// Check if form is completed
	if m.form.State == huh.StateCompleted {
		// Copy form values back to config
		m.applyFormToConfig()

		// Determine save path
		targetPath := m.globalPath
		if m.saveTarget == "project" {
			targetPath = m.projectPath
		}

		// Save config
		if err := config.Save(m.config, targetPath); err != nil {
			m.err = err
			m.saved = false
		} else {
			m.saved = true
			m.err = nil
		}

		// Hide form after successful save
		if m.saved {
			m.visible = false
		}
	}

	return m, cmd
}

// applyFormToConfig copies form field values back to the config struct.
func (m *SettingsPaneModel) applyFormToConfig() {
	// Update agents
	if coder, ok := m.config.Agents["coder"]; ok {
		coder.Provider = m.coderProvider
		coder.Model = m.coderModel
		m.config.Agents["coder"] = coder
	}

	if reviewer, ok := m.config.Agents["reviewer"]; ok {
		reviewer.Provider = m.reviewProvider
		reviewer.Model = m.reviewModel
		m.config.Agents["reviewer"] = reviewer
	}

	// Update providers
	if claude, ok := m.config.Providers["claude"]; ok {
		claude.Command = m.claudeCommand
		m.config.Providers["claude"] = claude
	}

	if codex, ok := m.config.Providers["codex"]; ok {
		codex.Command = m.codexCommand
		m.config.Providers["codex"] = codex
	}

	if goose, ok := m.config.Providers["goose"]; ok {
		goose.Command = m.gooseCommand
		m.config.Providers["goose"] = goose
	}
}

// View renders the settings pane.
func (m SettingsPaneModel) View() string {
	if !m.visible {
		return ""
	}

	var content string

	// Show saved message if just saved
	if m.saved && m.form.State == huh.StateCompleted {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true).
			Render("✓ Settings saved successfully!")
	} else if m.err != nil {
		// Show error if save failed
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true).
			Render(fmt.Sprintf("✗ Error saving: %v", m.err))
	} else {
		// Render form
		content = m.form.View()
	}

	// Wrap in styled border
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(m.width - 4).
		Height(m.height - 4)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Render("⚙ Settings")

	body := style.Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, title, body)
}

// SetSize updates the dimensions of the settings pane.
func (m *SettingsPaneModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	if m.form != nil {
		m.form.WithWidth(w - 8).WithHeight(h - 8)
	}
}

// SetVisible shows or hides the settings pane.
func (m *SettingsPaneModel) SetVisible(v bool) {
	m.visible = v
	m.saved = false
	m.err = nil

	// Reset form state when showing
	if v && m.form != nil {
		// Rebuild form to reset state
		m.buildForm()
	}
}

// IsVisible returns whether the settings pane is currently visible.
func (m SettingsPaneModel) IsVisible() bool {
	return m.visible
}
