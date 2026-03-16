package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorRunning  = lipgloss.Color("82")
	colorStopped  = lipgloss.Color("240")
	colorStarting = lipgloss.Color("214")
	colorError    = lipgloss.Color("196")
	colorSelected = lipgloss.Color("63")

	styleHeader   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	styleSelected = lipgloss.NewStyle().
			Background(colorSelected).
			Foreground(lipgloss.Color("15"))
	styleStatusBar = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("250"))
	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))
	styleTitle = lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Foreground(lipgloss.Color("214"))
	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(colorSelected).
			Padding(0, 1)
	styleTabInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Padding(0, 1)
	styleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleInfo      = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	styleLabelNormal  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleLabelFocused = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	styleFormHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "running":
		return lipgloss.NewStyle().Foreground(colorRunning)
	case "stopped":
		return lipgloss.NewStyle().Foreground(colorStopped)
	case "starting", "reconnecting":
		return lipgloss.NewStyle().Foreground(colorStarting)
	case "unhealthy", "error":
		return lipgloss.NewStyle().Foreground(colorError)
	default:
		return lipgloss.NewStyle()
	}
}

func statusIcon(status string) string {
	switch status {
	case "running":
		return "●"
	case "stopped":
		return "○"
	case "starting", "reconnecting":
		return "◎"
	case "unhealthy", "error":
		return "✕"
	default:
		return "○"
	}
}
