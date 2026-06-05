package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).
			Padding(0, 2)

	okStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	badStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 2)

	keyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	liveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	idleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)
