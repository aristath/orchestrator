package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Border styles
var (
	StyleFocusedBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))

	StyleUnfocusedBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))
)

// Status styles
var (
	StyleStatusRunning = lipgloss.NewStyle().
				Foreground(lipgloss.Color("yellow")).
				Bold(true)

	StyleStatusComplete = lipgloss.NewStyle().
				Foreground(lipgloss.Color("green")).
				Bold(true)

	StyleStatusFailed = lipgloss.NewStyle().
				Foreground(lipgloss.Color("red")).
				Bold(true)

	StyleStatusPending = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// UI element styles
var (
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	StyleHelp = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)
