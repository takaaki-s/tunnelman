package ui

import "github.com/charmbracelet/lipgloss"

var helpContent = `
  tunnelman UI - Keyboard Shortcuts

  Navigation
    ↑ / ↓        Move cursor
    Tab          Next profile
    Shift+Tab    Previous profile

  Tunnel Actions
    s            Start selected tunnel
    S            Stop selected tunnel
    Ctrl+S       Start all tunnels in current profile
    Ctrl+X       Stop all tunnels in current profile

  CRUD
    a            Add new tunnel
    e            Edit selected tunnel
    d            Delete selected tunnel (with confirmation)

  General
    r            Refresh
    ?            Toggle this help
    q / Ctrl+C   Quit
`

// RenderHelp renders the help overlay centered in the given dimensions.
func RenderHelp(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(1, 2).
		Bold(false)

	box := style.Render(helpContent)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
