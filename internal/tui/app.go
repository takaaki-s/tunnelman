// Package tui provides the terminal user interface for tunnelman.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/takaaki-s/tunnelman/internal/core"
	"github.com/takaaki-s/tunnelman/internal/store"
)

// App represents the TUI application
type App struct {
	app           *tview.Application
	tunnelManager *core.TunnelManager
	configStore   *store.ConfigStore

	// UI components
	pages       *tview.Pages
	headerBar   *tview.TextView
	tunnelList  *tview.Table
	statusBar   *tview.TextView
	detailView  *tview.TextView
	helpView    *tview.TextView
	footerBar   *tview.TextView

	// State
	selectedTunnel *core.Tunnel
	lastUpdate     time.Time
	searchMode     *SearchMode
	currentProfile string
}

// NewApp creates a new TUI application
func NewApp(tunnelManager *core.TunnelManager, configStore *store.ConfigStore) *App {
	return &App{
		app:            tview.NewApplication(),
		tunnelManager:  tunnelManager,
		configStore:    configStore,
		lastUpdate:     time.Now(),
		currentProfile: "default",
	}
}

// Run starts the TUI application
func (a *App) Run() error {
	// Initialize UI components
	a.initUI()

	// Start status update goroutine
	go a.watchStatusChanges()

	// Start auto-connect tunnels
	a.tunnelManager.StartAutoConnectTunnels()

	// Run the application
	return a.app.Run()
}

// Stop stops the TUI application without stopping tunnels
func (a *App) Stop() {
	a.app.Stop()
}

// SetInitialProfile sets the initial profile to display
func (a *App) SetInitialProfile(profile string) {
	a.currentProfile = profile
}

// initUI initializes the user interface
func (a *App) initUI() {
	// Initialize search mode
	a.initSearchMode()

	// Create main layout components
	a.createHeaderBar()
	a.createTunnelList()
	a.createDetailView()
	a.createStatusBar()
	a.createFooterBar()
	a.createHelpView()

	// Create layout with flexbox
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.headerBar, 3, 0, false).
		AddItem(a.createMainContent(), 0, 1, true).
		AddItem(a.statusBar, 1, 0, false).
		AddItem(a.footerBar, 2, 0, false)

	// Create pages for modal dialogs
	a.pages = tview.NewPages().
		AddPage("main", mainFlex, true, true).
		AddPage("help", a.createHelpModal(), true, false)

	// Set up application
	a.app.SetRoot(a.pages, true).
		SetFocus(a.tunnelList).
		SetInputCapture(a.handleGlobalKeys)

	// Initial tunnel list update
	a.updateTunnelList()
}

// createMainContent creates the main content area
func (a *App) createMainContent() *tview.Flex {
	// Create horizontal split between list and details
	return tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(a.createListPanel(), 0, 2, true).
		AddItem(a.detailView, 0, 1, false)
}

// createListPanel creates the tunnel list panel
func (a *App) createListPanel() *tview.Flex {
	listPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.tunnelList, 0, 1, true)

	return listPanel
}

// createHeaderBar creates the application header bar
func (a *App) createHeaderBar() {
	a.headerBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	a.updateHeaderBar()
	a.headerBar.SetBorder(true).
		SetBorderColor(tcell.ColorBlue)
}

// createFooterBar creates the footer bar with shortcuts
func (a *App) createFooterBar() {
	a.footerBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	a.updateFooterBar()
	a.footerBar.SetBorder(false).
		SetBackgroundColor(tcell.ColorBlack)
}

// createTunnelList creates the tunnel list table
func (a *App) createTunnelList() {
	a.tunnelList = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetSeparator(' ')

	// Set up selection handler
	a.tunnelList.SetSelectionChangedFunc(a.onTunnelSelected)

	// Set up input handler
	a.tunnelList.SetInputCapture(a.handleListKeys)

	// Style the list
	a.tunnelList.SetBorder(true).
		SetTitle(" Tunnels ").
		SetTitleAlign(tview.AlignLeft)
}

// createDetailView creates the tunnel detail view
func (a *App) createDetailView() {
	a.detailView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)

	a.detailView.SetBorder(true).
		SetTitle(" Details ").
		SetTitleAlign(tview.AlignLeft)
}

// createStatusBar creates the status bar
func (a *App) createStatusBar() {
	a.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	a.updateStatusBar("")
}

// createHelpView creates the help view
func (a *App) createHelpView() {
	helpText := `[::b]Keyboard Shortcuts[::-]

[yellow]Navigation:[::-]
  ↑/k     Move up
  ↓/j     Move down
  Tab     Switch focus
  /       Search tunnels

[yellow]Tunnel Operations:[::-]
  Enter   Start/Stop tunnel
  u       Start tunnel
  d       Stop tunnel
  e       Edit tunnel
  c       Create new tunnel
  r       Remove (delete) tunnel
  a       Toggle auto-connect

[yellow]Batch Operations:[::-]
  A       Start all tunnels in profile
  X       Stop all tunnels in profile
  g       Switch profile
  p       Profile management (add/delete)
  f       Filter view

[yellow]Application:[::-]
  ?       Show this help
  q       Quit (tunnels keep running)
  Ctrl+C  Force quit

[yellow]Tunnel Types:[::-]
  Local (-L):   Forward local port to remote
  Remote (-R):  Forward remote port to local
  Dynamic (-D): SOCKS proxy

Press any key to close this help.`

	a.helpView = tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText).
		SetScrollable(true)
}

// createHelpModal creates the help modal dialog
func (a *App) createHelpModal() *tview.Flex {
	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(a.helpView, 60, 1, true).
			AddItem(nil, 0, 1, false), 20, 1, true).
		AddItem(nil, 0, 1, false)

	a.helpView.SetBorder(true).
		SetTitle(" Help ").
		SetTitleAlign(tview.AlignCenter)

	a.helpView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		a.pages.HidePage("help")
		a.app.SetFocus(a.tunnelList)
		return nil
	})

	return modal
}

// updateTunnelList updates the tunnel list display
func (a *App) updateTunnelList() {
	a.tunnelList.Clear()

	// Add header row with updated columns
	headers := []string{"St", "Name", "Host", "Local", "Remote", "Mode", "Started"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetAlign(tview.AlignCenter)
		a.tunnelList.SetCell(0, col, cell)
	}

	// Get tunnels filtered by current profile
	var tunnels []*core.Tunnel
	if a.currentProfile != "" {
		tunnels = a.tunnelManager.GetTunnelsByProfile(a.currentProfile)
	} else {
		tunnels = a.tunnelManager.GetTunnels()
	}
	for row, tunnel := range tunnels {
		rowNum := row + 1

		// Status indicator
		var statusIcon string
		var statusColor tcell.Color
		switch tunnel.Status {
		case core.StatusRunning:
			statusIcon = "●"
			statusColor = tcell.ColorGreen
		case core.StatusStopped:
			statusIcon = "○"
			statusColor = tcell.ColorGray
		case core.StatusError:
			statusIcon = "×"
			statusColor = tcell.ColorRed
		case core.StatusConnecting:
			statusIcon = "◐"
			statusColor = tcell.ColorYellow
		default:
			statusIcon = "○"
			statusColor = tcell.ColorGray
		}

		// Mode indicator
		var modeIcon string
		var modeColor tcell.Color
		switch tunnel.Type {
		case core.LocalForward:
			modeIcon = "→"
			modeColor = tcell.ColorBlue
		case core.RemoteForward:
			modeIcon = "←"
			modeColor = tcell.ColorOrange
		case core.DynamicForward:
			modeIcon = "⇄"
			modeColor = tcell.ColorPurple
		}

		// Started time
		var startedStr string
		if tunnel.StartedAt != nil {
			duration := time.Since(*tunnel.StartedAt)
			startedStr = formatDuration(duration)
		} else {
			startedStr = "-"
		}

		// Create cells
		cells := []struct {
			text  string
			color tcell.Color
			align int
		}{
			{statusIcon, statusColor, tview.AlignCenter},
			{tunnel.Name, tcell.ColorWhite, tview.AlignLeft},
			{tunnel.SSHHost, tcell.ColorAqua, tview.AlignLeft},
			{fmt.Sprintf("%d", tunnel.LocalPort), tcell.ColorWhite, tview.AlignRight},
			{fmt.Sprintf("%d", tunnel.RemotePort), tcell.ColorWhite, tview.AlignRight},
			{modeIcon, modeColor, tview.AlignCenter},
			{startedStr, tcell.ColorWhite, tview.AlignRight},
		}

		for col, cell := range cells {
			tableCell := tview.NewTableCell(cell.text).
				SetTextColor(cell.color).
				SetReference(tunnel).
				SetAlign(cell.align)

			a.tunnelList.SetCell(rowNum, col, tableCell)
		}
	}

	// Restore selection if possible
	if a.selectedTunnel != nil {
		for row := 1; row < a.tunnelList.GetRowCount(); row++ {
			if cell := a.tunnelList.GetCell(row, 1); cell != nil {
				if t, ok := cell.GetReference().(*core.Tunnel); ok && t.ID == a.selectedTunnel.ID {
					a.tunnelList.Select(row, 1)
					break
				}
			}
		}
	} else if a.tunnelList.GetRowCount() > 1 {
		a.tunnelList.Select(1, 1)
	}
}

// formatStatus formats tunnel status with appropriate color
func (a *App) formatStatus(status core.TunnelStatus) (string, tcell.Color) {
	switch status {
	case core.StatusRunning:
		return "● Running", tcell.ColorGreen
	case core.StatusStopped:
		return "○ Stopped", tcell.ColorSilver
	case core.StatusConnecting:
		return "◐ Connecting", tcell.ColorYellow
	case core.StatusError:
		return "✗ Error", tcell.ColorRed
	default:
		return string(status), tcell.ColorWhite
	}
}

// onTunnelSelected handles tunnel selection
func (a *App) onTunnelSelected(row, column int) {
	if row == 0 || row >= a.tunnelList.GetRowCount() {
		return
	}

	cell := a.tunnelList.GetCell(row, 1)
	if cell == nil {
		return
	}

	if tunnel, ok := cell.GetReference().(*core.Tunnel); ok {
		a.selectedTunnel = tunnel
		a.updateDetailView(tunnel)
	}
}

// updateDetailView updates the detail view for a tunnel
func (a *App) updateDetailView(tunnel *core.Tunnel) {
	if tunnel == nil {
		a.detailView.Clear()
		return
	}

	details := strings.Builder{}
	details.WriteString(fmt.Sprintf("[::b]%s[::-]\n\n", tunnel.Name))

	// Connection details
	details.WriteString("[yellow]Connection:[::-]\n")
	details.WriteString(fmt.Sprintf("  SSH: %s\n", tunnel.SSHHost))
	details.WriteString("\n")

	// Forwarding details
	details.WriteString("[yellow]Forwarding:[::-]\n")
	switch tunnel.Type {
	case core.LocalForward:
		details.WriteString(fmt.Sprintf("  Type: Local Forward (-L)\n"))
		details.WriteString(fmt.Sprintf("  Local: %s:%d\n", tunnel.LocalHost, tunnel.LocalPort))
		details.WriteString(fmt.Sprintf("  Remote: %s:%d\n", tunnel.RemoteHost, tunnel.RemotePort))
	case core.RemoteForward:
		details.WriteString(fmt.Sprintf("  Type: Remote Forward (-R)\n"))
		details.WriteString(fmt.Sprintf("  Remote Port: %d\n", tunnel.RemotePort))
		details.WriteString(fmt.Sprintf("  Local: %s:%d\n", tunnel.LocalHost, tunnel.LocalPort))
	case core.DynamicForward:
		details.WriteString(fmt.Sprintf("  Type: Dynamic (SOCKS)\n"))
		details.WriteString(fmt.Sprintf("  Local: %s:%d\n", tunnel.LocalHost, tunnel.LocalPort))
	}
	details.WriteString("\n")

	// Status details
	details.WriteString("[yellow]Status:[::-]\n")
	status, color := a.formatStatus(tunnel.Status)
	details.WriteString(fmt.Sprintf("  State: [%s]%s[::-]\n", getColorName(color), status))
	if tunnel.PID > 0 {
		details.WriteString(fmt.Sprintf("  PID: %d\n", tunnel.PID))
	}
	if tunnel.StartedAt != nil {
		duration := time.Since(*tunnel.StartedAt)
		details.WriteString(fmt.Sprintf("  Uptime: %s\n", formatDuration(duration)))
	}
	if tunnel.LastError != nil {
		details.WriteString(fmt.Sprintf("  [red]Error: %v[::-]\n", tunnel.LastError))
	}
	details.WriteString("\n")

	// Options
	details.WriteString("[yellow]Options:[::-]\n")
	details.WriteString(fmt.Sprintf("  Auto-connect: %v\n", tunnel.AutoConnect))
	if len(tunnel.ExtraArgs) > 0 {
		details.WriteString(fmt.Sprintf("  Extra args: %s\n", strings.Join(tunnel.ExtraArgs, " ")))
	}

	// SSH Command
	details.WriteString("\n[yellow]SSH Command:[::-]\n")
	cmd := strings.Join(tunnel.BuildSSHCommand(), " ")
	details.WriteString(fmt.Sprintf("  [dim]%s[::-]\n", cmd))

	a.detailView.SetText(details.String())
}

// updateHeaderBar updates the header bar
func (a *App) updateHeaderBar() {
	tunnels := a.tunnelManager.GetTunnels()
	running := 0
	for _, t := range tunnels {
		if t.Status == core.StatusRunning {
			running++
		}
	}

	headerText := fmt.Sprintf(
		"[::b]TUNNELMAN[::-] | Profile: [yellow]%s[::-] | Connections: [green]%d/%d[::-] | [dim]? Help | / Search | q Quit[::-]",
		a.currentProfile,
		running,
		len(tunnels),
	)
	a.headerBar.SetText(headerText)
}

// updateFooterBar updates the footer bar with current shortcuts
func (a *App) updateFooterBar() {
	shortcuts := []string{
		"[yellow]u/d[::-] Start/Stop",
		"[yellow]A[::-] All Start",
		"[yellow]X[::-] All Stop",
		"[yellow]c[::-] Create",
		"[yellow]r[::-] Remove",
		"[yellow]f[::-] Mode(→/←)",
		"[yellow]g[::-] Profile",
		"[yellow]/[::-] Search",
	}

	footerText := fmt.Sprintf(" %s", strings.Join(shortcuts, " | "))
	a.footerBar.SetText(footerText)
}

// updateStatusBar updates the status bar
func (a *App) updateStatusBar(message string) {
	if message != "" {
		a.statusBar.SetText(fmt.Sprintf(" %s", message))
		return
	}

	// Default status message
	tunnels := a.tunnelManager.GetTunnels()
	running := 0
	for _, t := range tunnels {
		if t.Status == core.StatusRunning {
			running++
		}
	}

	status := fmt.Sprintf(" Ready | %d tunnel(s), %d active", len(tunnels), running)
	a.statusBar.SetText(status)
}

// watchStatusChanges watches for tunnel status changes
func (a *App) watchStatusChanges() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	statusChanges := a.tunnelManager.GetStatusChanges()

	for {
		select {
		case change := <-statusChanges:
			a.app.QueueUpdateDraw(func() {
				a.updateTunnelList()
				if a.selectedTunnel != nil && a.selectedTunnel.ID == change.TunnelID {
					if tunnel, err := a.tunnelManager.GetTunnel(change.TunnelID); err == nil {
						a.updateDetailView(tunnel)
					}
				}
				if change.Error != nil {
					a.updateStatusBar(fmt.Sprintf("Error: %v", change.Error))
				} else {
					a.updateStatusBar("")
				}
			})

		case <-ticker.C:
			// Periodic UI update for uptime display
			if time.Since(a.lastUpdate) > 5*time.Second {
				a.app.QueueUpdateDraw(func() {
					if a.selectedTunnel != nil {
						if tunnel, err := a.tunnelManager.GetTunnel(a.selectedTunnel.ID); err == nil {
							a.updateDetailView(tunnel)
						}
					}
					a.lastUpdate = time.Now()
				})
			}
		}
	}
}

// Helper functions

func getColorName(color tcell.Color) string {
	switch color {
	case tcell.ColorGreen:
		return "green"
	case tcell.ColorRed:
		return "red"
	case tcell.ColorYellow:
		return "yellow"
	case tcell.ColorSilver:
		return "gray"
	default:
		return "white"
	}
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}