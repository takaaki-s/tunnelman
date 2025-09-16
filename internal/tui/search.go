// Package tui provides search functionality for the tunnelman TUI
package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/takaaki-s/tunnelman/internal/core"
)

// SearchMode represents the search state
type SearchMode struct {
	active       bool
	query        string
	results      []*core.Tunnel
	currentIndex int
	inputField   *tview.InputField
}

// initSearchMode initializes the search mode
func (a *App) initSearchMode() {
	a.searchMode = &SearchMode{
		active:       false,
		query:        "",
		results:      []*core.Tunnel{},
		currentIndex: 0,
	}
}

// startSearch initiates the search mode
func (a *App) startSearch() {
	if a.searchMode == nil {
		a.initSearchMode()
	}

	a.searchMode.active = true
	a.searchMode.query = ""
	a.searchMode.results = []*core.Tunnel{}
	a.searchMode.currentIndex = 0

	// Create search input field
	searchInput := tview.NewInputField().
		SetLabel("/").
		SetFieldWidth(30).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetLabelColor(tcell.ColorYellow).
		SetFieldTextColor(tcell.ColorWhite).
		SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				a.selectSearchResult()
				a.exitSearch()
			case tcell.KeyEscape:
				a.exitSearch()
			case tcell.KeyTab:
				a.nextSearchResult()
			}
		}).
		SetChangedFunc(func(text string) {
			a.searchMode.query = text
			a.performSearch()
		})

	a.searchMode.inputField = searchInput

	// Create search overlay
	searchBar := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(searchInput, 35, 0, true).
		AddItem(tview.NewTextView().
			SetDynamicColors(true).
			SetText("[dim]ESC: cancel | TAB: next | Enter: select[::-]"), 0, 1, false)

	searchBar.SetBorder(true).
		SetTitle(" Search ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorYellow)

	// Position search bar at the bottom
	searchOverlay := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 2, 0, false).
			AddItem(searchBar, 0, 1, true).
			AddItem(nil, 2, 0, false), 3, 0, true)

	a.pages.AddPage("search", searchOverlay, true, true)
	a.app.SetFocus(searchInput)

	// Initial search with empty query (shows all)
	a.performSearch()
}

// performSearch executes the search and highlights results
func (a *App) performSearch() {
	query := strings.ToLower(a.searchMode.query)
	a.searchMode.results = []*core.Tunnel{}
	a.searchMode.currentIndex = 0

	tunnels := a.tunnelManager.GetTunnels()

	// Clear previous highlights
	a.updateTunnelList()

	if query == "" {
		// If no query, show all tunnels normally
		return
	}

	// Find matching tunnels
	for _, tunnel := range tunnels {
		if a.matchesTunnel(tunnel, query) {
			a.searchMode.results = append(a.searchMode.results, tunnel)
		}
	}

	// Highlight search results in the table
	a.highlightSearchResults()

	// Update status bar with search info
	if len(a.searchMode.results) > 0 {
		a.updateStatusBar(fmt.Sprintf("Search: %d result(s) for '%s'", len(a.searchMode.results), query))
		// Select first result
		a.selectTunnelByID(a.searchMode.results[0].ID)
	} else {
		a.updateStatusBar(fmt.Sprintf("Search: No results for '%s'", query))
	}
}

// matchesTunnel checks if a tunnel matches the search query
func (a *App) matchesTunnel(tunnel *core.Tunnel, query string) bool {
	// Search in multiple fields
	searchFields := []string{
		strings.ToLower(tunnel.Name),
		strings.ToLower(tunnel.SSHHost),
		strings.ToLower(string(tunnel.Type)),
		fmt.Sprintf("%d", tunnel.LocalPort),
		fmt.Sprintf("%d", tunnel.RemotePort),
		strings.ToLower(tunnel.RemoteHost),
		strings.ToLower(string(tunnel.Status)),
	}

	for _, field := range searchFields {
		if strings.Contains(field, query) {
			return true
		}
	}

	return false
}

// highlightSearchResults highlights matching tunnels in the list
func (a *App) highlightSearchResults() {
	if len(a.searchMode.results) == 0 {
		return
	}

	// Create a map of result IDs for quick lookup
	resultMap := make(map[string]bool)
	for _, tunnel := range a.searchMode.results {
		resultMap[tunnel.ID] = true
	}

	// Update table cells to highlight results
	for row := 1; row < a.tunnelList.GetRowCount(); row++ {
		cell := a.tunnelList.GetCell(row, 1) // Name column
		if cell == nil {
			continue
		}

		if tunnel, ok := cell.GetReference().(*core.Tunnel); ok {
			if resultMap[tunnel.ID] {
				// Highlight matching rows
				for col := 0; col < a.tunnelList.GetColumnCount(); col++ {
					if c := a.tunnelList.GetCell(row, col); c != nil {
						c.SetBackgroundColor(tcell.ColorDarkBlue)
					}
				}
			}
		}
	}
}

// nextSearchResult moves to the next search result
func (a *App) nextSearchResult() {
	if len(a.searchMode.results) == 0 {
		return
	}

	a.searchMode.currentIndex = (a.searchMode.currentIndex + 1) % len(a.searchMode.results)
	tunnel := a.searchMode.results[a.searchMode.currentIndex]
	a.selectTunnelByID(tunnel.ID)
	a.updateStatusBar(fmt.Sprintf("Search result %d of %d", a.searchMode.currentIndex+1, len(a.searchMode.results)))
}

// selectSearchResult selects the current search result
func (a *App) selectSearchResult() {
	if len(a.searchMode.results) == 0 {
		return
	}

	tunnel := a.searchMode.results[a.searchMode.currentIndex]
	a.selectTunnelByID(tunnel.ID)
}

// selectTunnelByID selects a tunnel in the list by its ID
func (a *App) selectTunnelByID(tunnelID string) {
	for row := 1; row < a.tunnelList.GetRowCount(); row++ {
		cell := a.tunnelList.GetCell(row, 1)
		if cell == nil {
			continue
		}

		if tunnel, ok := cell.GetReference().(*core.Tunnel); ok && tunnel.ID == tunnelID {
			a.tunnelList.Select(row, 1)
			a.selectedTunnel = tunnel
			a.updateDetailView(tunnel)
			break
		}
	}
}

// exitSearch exits the search mode
func (a *App) exitSearch() {
	a.searchMode.active = false
	a.searchMode.query = ""
	a.searchMode.results = []*core.Tunnel{}
	a.searchMode.currentIndex = 0

	a.pages.RemovePage("search")
	a.app.SetFocus(a.tunnelList)

	// Clear highlights
	a.updateTunnelList()
	a.updateStatusBar("")
}

// FilterTunnels filters tunnels based on various criteria
func (a *App) FilterTunnels(filterType string) {
	tunnels := a.tunnelManager.GetTunnels()
	var filtered []*core.Tunnel

	switch filterType {
	case "running":
		for _, t := range tunnels {
			if t.Status == core.StatusRunning {
				filtered = append(filtered, t)
			}
		}
	case "stopped":
		for _, t := range tunnels {
			if t.Status == core.StatusStopped {
				filtered = append(filtered, t)
			}
		}
	case "error":
		for _, t := range tunnels {
			if t.Status == core.StatusError {
				filtered = append(filtered, t)
			}
		}
	case "auto":
		for _, t := range tunnels {
			if t.AutoConnect {
				filtered = append(filtered, t)
			}
		}
	case "local":
		for _, t := range tunnels {
			if t.Type == core.LocalForward {
				filtered = append(filtered, t)
			}
		}
	case "remote":
		for _, t := range tunnels {
			if t.Type == core.RemoteForward {
				filtered = append(filtered, t)
			}
		}
	case "dynamic":
		for _, t := range tunnels {
			if t.Type == core.DynamicForward {
				filtered = append(filtered, t)
			}
		}
	default:
		// No filter, show all
		return
	}

	// Update display with filtered results
	a.searchMode.results = filtered
	a.highlightSearchResults()
	a.updateStatusBar(fmt.Sprintf("Filter: %s (%d tunnels)", filterType, len(filtered)))
}