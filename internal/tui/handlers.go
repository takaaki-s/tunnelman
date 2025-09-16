// Package tui provides keyboard and interaction handlers
package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/takaaki-s/tunnelman/internal/core"
	"github.com/takaaki-s/tunnelman/internal/store"
)

// handleGlobalKeys handles global keyboard shortcuts
func (a *App) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
	// Check if any modal dialog is active
	// Modal pages that should block global shortcuts
	modalPages := []string{"add-tunnel", "edit-tunnel", "delete-confirm", "error", "filter-menu", "profile", "confirm", "ssh-import", "profile-mgmt"}
	for _, page := range modalPages {
		if a.pages.HasPage(page) {
			// Let the modal handle the input
			return event
		}
	}

	// Check if search mode is active
	if a.searchMode != nil && a.searchMode.active {
		return event
	}

	switch event.Key() {
	case tcell.KeyCtrlC:
		// Graceful shutdown
		a.shutdown()
		return nil

	case tcell.KeyRune:
		switch event.Rune() {
		case 'q', 'Q':
			a.confirmQuit()
			return nil

		case '?':
			a.showHelp()
			return nil

		case 'c', 'C':
			a.showAddTunnelForm()
			return nil

		case 'A':
			a.startAllTunnels()
			return nil

		case 'X':
			a.stopAllTunnels()
			return nil

		case '/':
			a.startSearch()
			return nil

		case 'f', 'F':
			a.toggleTunnelMode()
			return nil

		case 'g':
			// Switch profile
			a.showProfileMenu()
			return nil

		case 'p', 'P':
			// Profile management
			a.showProfileManagement()
			return nil

		case 'i':
			// Import from SSH config
			a.showSSHConfigImport()
			return nil
		}
	}

	return event
}

// handleListKeys handles keyboard input for the tunnel list
func (a *App) handleListKeys(event *tcell.EventKey) *tcell.EventKey {
	// Check if any modal dialog is active - if so, don't process list keys
	modalPages := []string{"add-tunnel", "edit-tunnel", "delete-confirm", "error", "filter-menu", "profile", "confirm", "ssh-import", "profile-mgmt"}
	for _, page := range modalPages {
		if a.pages.HasPage(page) {
			return event
		}
	}

	switch event.Key() {
	case tcell.KeyEnter:
		if a.selectedTunnel != nil {
			a.toggleTunnel()
		}
		return nil

	case tcell.KeyUp:
		row, col := a.tunnelList.GetSelection()
		if row > 1 {
			a.tunnelList.Select(row-1, col)
		}
		return nil

	case tcell.KeyDown:
		row, col := a.tunnelList.GetSelection()
		if row < a.tunnelList.GetRowCount()-1 {
			a.tunnelList.Select(row+1, col)
		}
		return nil

	case tcell.KeyRune:
		if a.selectedTunnel == nil && event.Rune() != 'c' && event.Rune() != 'C' {
			return event
		}

		switch event.Rune() {
		case 'u', 'U':
			// Start tunnel
			if a.selectedTunnel != nil && a.selectedTunnel.Status != core.StatusRunning {
				a.startTunnel()
			}
			return nil

		case 'd', 'D':
			// Stop tunnel
			if a.selectedTunnel != nil && a.selectedTunnel.Status == core.StatusRunning {
				a.stopTunnel()
			}
			return nil

		case 'r', 'R':
			// Delete tunnel with confirmation
			if a.selectedTunnel != nil {
				a.showDeleteConfirmation(a.selectedTunnel)
			}
			return nil

		case 'e', 'E':
			// Edit tunnel
			if a.selectedTunnel != nil {
				a.showEditTunnelDialog()
			}
			return nil

		case 'a':
			// Toggle auto-connect
			if a.selectedTunnel != nil {
				a.toggleAutoConnect()
			}
			return nil

		case 'j':
			// Move down (vim-style)
			row, col := a.tunnelList.GetSelection()
			if row < a.tunnelList.GetRowCount()-1 {
				a.tunnelList.Select(row+1, col)
			}
			return nil

		case 'k':
			// Move up (vim-style)
			row, col := a.tunnelList.GetSelection()
			if row > 1 {
				a.tunnelList.Select(row-1, col)
			}
			return nil
		}
	}

	return event
}

// toggleTunnel starts or stops the selected tunnel
func (a *App) toggleTunnel() {
	if a.selectedTunnel == nil {
		return
	}

	if a.selectedTunnel.Status == core.StatusRunning {
		a.stopTunnel()
	} else {
		a.startTunnel()
	}
}

// startTunnel starts the selected tunnel
func (a *App) startTunnel() {
	if a.selectedTunnel == nil {
		return
	}

	a.updateStatusBar("Starting tunnel...")
	err := a.tunnelManager.StartTunnel(a.selectedTunnel.ID)
	if err != nil {
		a.showErrorModal("Start Failed", err.Error())
	} else {
		a.updateStatusBar("✓ Tunnel started")
	}

	// Update UI
	a.updateTunnelList()
	a.updateHeaderBar()
	if tunnel, err := a.tunnelManager.GetTunnel(a.selectedTunnel.ID); err == nil {
		a.selectedTunnel = tunnel
		a.updateDetailView(tunnel)
	}
}

// stopTunnel stops the selected tunnel
func (a *App) stopTunnel() {
	if a.selectedTunnel == nil {
		return
	}

	a.updateStatusBar("Stopping tunnel...")
	err := a.tunnelManager.StopTunnel(a.selectedTunnel.ID)
	if err != nil {
		a.showErrorModal("Stop Failed", err.Error())
	} else {
		a.updateStatusBar("✓ Tunnel stopped")
	}

	// Update UI
	a.updateTunnelList()
	a.updateHeaderBar()
	if tunnel, err := a.tunnelManager.GetTunnel(a.selectedTunnel.ID); err == nil {
		a.selectedTunnel = tunnel
		a.updateDetailView(tunnel)
	}
}

// startAllTunnels starts all tunnels in the current profile
func (a *App) startAllTunnels() {
	a.updateStatusBar(fmt.Sprintf("Starting all tunnels in profile '%s'...", a.currentProfile))
	err := a.tunnelManager.StartProfileTunnels(a.currentProfile)
	if err != nil {
		a.updateStatusBar(fmt.Sprintf("Some tunnels failed to start: %v", err))
	} else {
		a.updateStatusBar(fmt.Sprintf("✓ Started all tunnels in profile '%s'", a.currentProfile))
	}

	a.updateTunnelList()
	a.updateHeaderBar()
}

// stopAllTunnels stops all running tunnels in the current profile
func (a *App) stopAllTunnels() {
	a.updateStatusBar(fmt.Sprintf("Stopping all tunnels in profile '%s'...", a.currentProfile))
	err := a.tunnelManager.StopProfileTunnels(a.currentProfile)
	if err != nil {
		a.updateStatusBar(fmt.Sprintf("Some tunnels failed to stop: %v", err))
	} else {
		a.updateStatusBar(fmt.Sprintf("✓ Stopped all tunnels in profile '%s'", a.currentProfile))
	}

	a.updateTunnelList()
	a.updateHeaderBar()
}

// restartTunnel restarts the selected tunnel
func (a *App) restartTunnel() {
	if a.selectedTunnel == nil {
		return
	}

	a.updateStatusBar("Restarting tunnel...")

	if err := a.tunnelManager.RestartTunnel(a.selectedTunnel.ID); err != nil {
		a.showErrorModal("Restart Failed", err.Error())
		return
	}

	a.updateStatusBar("Tunnel restarted")
	a.updateTunnelList()

	if tunnel, err := a.tunnelManager.GetTunnel(a.selectedTunnel.ID); err == nil {
		a.selectedTunnel = tunnel
		a.updateDetailView(tunnel)
	}
}

// toggleAutoConnect toggles the auto-connect setting for the selected tunnel
func (a *App) toggleAutoConnect() {
	if a.selectedTunnel == nil {
		return
	}

	tunnel := a.selectedTunnel.Clone()
	tunnel.AutoConnect = !tunnel.AutoConnect

	if err := a.tunnelManager.UpdateTunnel(tunnel); err != nil {
		a.showErrorModal("Update Failed", err.Error())
		return
	}

	status := "disabled"
	if tunnel.AutoConnect {
		status = "enabled"
	}
	a.updateStatusBar(fmt.Sprintf("✓ Auto-connect %s", status))

	a.selectedTunnel = tunnel
	a.updateTunnelList()
	a.updateDetailView(tunnel)
}

// toggleTunnelMode toggles the selected tunnel between forward and reverse mode
func (a *App) toggleTunnelMode() {
	if a.selectedTunnel == nil {
		a.updateStatusBar("⚠ No tunnel selected")
		return
	}

	// Check if tunnel is running
	if a.selectedTunnel.Status == core.StatusRunning {
		a.updateStatusBar("⚠ Stop the tunnel before changing mode")
		return
	}

	// Remember current selection position
	currentRow, _ := a.tunnelList.GetSelection()

	// Toggle between forward and reverse (skip dynamic)
	switch a.selectedTunnel.Type {
	case core.LocalForward:
		a.selectedTunnel.Type = core.RemoteForward
	case core.RemoteForward:
		a.selectedTunnel.Type = core.LocalForward
	case core.DynamicForward:
		// Dynamic forward stays as is
		a.updateStatusBar("⚠ Dynamic forward mode cannot be toggled")
		return
	}

	// Save the change
	if err := a.tunnelManager.UpdateTunnel(a.selectedTunnel); err != nil {
		a.showErrorModal("Update Failed", err.Error())
		return
	}

	// Update UI while maintaining selection position
	a.updateTunnelList()

	// Restore selection to the same row if possible
	if currentRow > 0 && currentRow < a.tunnelList.GetRowCount() {
		a.tunnelList.Select(currentRow, 0)
	}

	a.updateDetailView(a.selectedTunnel)

	modeStr := "forward"
	if a.selectedTunnel.Type == core.RemoteForward {
		modeStr = "reverse"
	}
	a.updateStatusBar(fmt.Sprintf("✓ Mode changed to %s", modeStr))
}

// showFilterMenu shows the filter menu
func (a *App) showFilterMenu() {
	filterOptions := []string{
		"All Tunnels",
		"Running",
		"Stopped",
		"Error",
		"Auto-connect",
		"Local Forward",
		"Remote Forward",
		"Dynamic/SOCKS",
	}

	modal := tview.NewModal().
		SetText("Select filter:").
		AddButtons(filterOptions).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonIndex {
			case 0:
				a.updateTunnelList() // Show all
			case 1:
				a.FilterTunnels("running")
			case 2:
				a.FilterTunnels("stopped")
			case 3:
				a.FilterTunnels("error")
			case 4:
				a.FilterTunnels("auto")
			case 5:
				a.FilterTunnels("local")
			case 6:
				a.FilterTunnels("remote")
			case 7:
				a.FilterTunnels("dynamic")
			}
			a.pages.RemovePage("filter")
			a.app.SetFocus(a.tunnelList)
		})

	a.pages.AddPage("filter", modal, true, true)
	a.app.SetFocus(modal)
}

// showHelp displays the help modal
func (a *App) showHelp() {
	a.pages.ShowPage("help")
	a.app.SetFocus(a.helpView)
}

// Removed - now using showAddTunnelForm from modals.go

// showEditTunnelDialog shows the dialog for editing a tunnel
func (a *App) showEditTunnelDialog() {
	if a.selectedTunnel == nil {
		return
	}

	if a.selectedTunnel.Status == core.StatusRunning {
		a.showErrorModal("Cannot Edit", "Stop the tunnel before editing")
		return
	}

	form := a.createAdvancedTunnelForm(a.selectedTunnel)

	// Set InputCapture to prevent global key handlers from interfering
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Allow ESC to close the form
		if event.Key() == tcell.KeyEscape {
			a.pages.RemovePage("edit-tunnel")
			a.app.SetFocus(a.tunnelList)
			return nil
		}
		// Let the form handle all other input
		return event
	})

	modal := a.createModalOverlay(form, 70, 25)
	a.pages.AddPage("edit-tunnel", modal, true, true)
	a.app.SetFocus(form)
}

// Removed - using forms from modals.go

// Removed - now using showDeleteConfirmation from modals.go

// confirmQuit shows confirmation dialog for application exit
func (a *App) confirmQuit() {
	// Check if any tunnels are running
	tunnels := a.tunnelManager.GetTunnels()
	runningCount := 0
	for _, t := range tunnels {
		if t.Status == core.StatusRunning {
			runningCount++
		}
	}

	message := "Are you sure you want to quit?"
	if runningCount > 0 {
		message = fmt.Sprintf("%d tunnel(s) are still running.\n%s", runningCount, message)
	}

	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Quit", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Quit" {
				a.shutdown()
			} else {
				a.pages.RemovePage("confirm")
				a.app.SetFocus(a.tunnelList)
			}
		})

	a.pages.AddPage("confirm", modal, true, true)
	a.app.SetFocus(modal)
}

// Removed - now using showErrorModal from modals.go

// shutdown performs graceful application shutdown
func (a *App) shutdown() {
	// Keep tunnels running on shutdown (as per spec)
	a.app.Stop()
}

// showProfileMenu shows the profile switching menu
func (a *App) showProfileMenu() {
	config, err := a.configStore.LoadConfig()
	if err != nil {
		a.showErrorModal("Error", "Failed to load profiles")
		return
	}

	profileOptions := []string{"default"}
	for _, profile := range config.Profiles {
		if profile.Name != "default" {
			profileOptions = append(profileOptions, profile.Name)
		}
	}
	profileOptions = append(profileOptions, "Cancel")

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Current profile: %s\n\nSelect profile:", a.currentProfile)).
		AddButtons(profileOptions).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel != "Cancel" && buttonIndex < len(profileOptions)-1 {
				a.currentProfile = buttonLabel
				a.updateStatusBar(fmt.Sprintf("Switched to profile: %s", a.currentProfile))
				a.updateTunnelList()
				a.updateHeaderBar()
			}
			a.pages.RemovePage("profile")
			a.app.SetFocus(a.tunnelList)
		})

	// Add InputCapture to handle ESC key properly
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.pages.RemovePage("profile")
			a.app.SetFocus(a.tunnelList)
			return nil
		}
		// Let the modal handle all other input
		return event
	})

	a.pages.AddPage("profile", modal, true, true)
	a.app.SetFocus(modal)
}

// showProfileManagement shows the profile management dialog
func (a *App) showProfileManagement() {
	form := tview.NewForm()
	form.SetBorder(true).
		SetTitle(" Profile Management ").
		SetTitleAlign(tview.AlignCenter)

	// Add dropdown for action selection
	actions := []string{"Create New Profile", "Delete Profile", "Cancel"}
	form.AddDropDown("Action", actions, 0, nil)

	// Add input field for profile name
	form.AddInputField("Profile Name", "", 30, nil, nil)

	// Set InputCapture to prevent global key handlers from interfering
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Allow ESC to close the form
		if event.Key() == tcell.KeyEscape {
			a.pages.RemovePage("profile-mgmt")
			a.app.SetFocus(a.tunnelList)
			return nil
		}
		// Let the form handle all other input
		return event
	})

	form.AddButton("Execute", func() {
		_, action := form.GetFormItemByLabel("Action").(*tview.DropDown).GetCurrentOption()
		profileName := form.GetFormItemByLabel("Profile Name").(*tview.InputField).GetText()

		if profileName == "" && action != "Cancel" {
			a.showErrorModal("Error", "Profile name is required")
			return
		}

		switch action {
		case "Create New Profile":
			// Create a new profile
			config, err := a.configStore.LoadConfig()
			if err != nil {
				a.pages.RemovePage("profile-mgmt")
				a.showErrorModal("Error", "Failed to load config")
				return
			}

			// Check if profile already exists
			for _, p := range config.Profiles {
				if p.Name == profileName {
					a.pages.RemovePage("profile-mgmt")
					a.showErrorModal("Error", "Profile already exists")
					return
				}
			}

			// Add new profile
			newProfile := store.Profile{
				Name:        profileName,
				Description: fmt.Sprintf("%s profile", profileName),
			}
			config.Profiles = append(config.Profiles, newProfile)

			// Save config
			if err := a.configStore.SaveConfig(config); err != nil {
				a.pages.RemovePage("profile-mgmt")
				a.showErrorModal("Error", "Failed to save profile")
				return
			}

			a.updateStatusBar(fmt.Sprintf("✓ Created profile: %s", profileName))

		case "Delete Profile":
			if profileName == "default" {
				a.pages.RemovePage("profile-mgmt")
				a.showErrorModal("Error", "Cannot delete default profile")
				return
			}

			// Load config and remove profile
			config, err := a.configStore.LoadConfig()
			if err != nil {
				a.pages.RemovePage("profile-mgmt")
				a.showErrorModal("Error", "Failed to load config")
				return
			}

			// Remove profile
			var newProfiles []store.Profile
			found := false
			for _, p := range config.Profiles {
				if p.Name != profileName {
					newProfiles = append(newProfiles, p)
				} else {
					found = true
				}
			}

			if !found {
				a.pages.RemovePage("profile-mgmt")
				a.showErrorModal("Error", "Profile not found")
				return
			}

			config.Profiles = newProfiles

			// Save config
			if err := a.configStore.SaveConfig(config); err != nil {
				a.pages.RemovePage("profile-mgmt")
				a.showErrorModal("Error", "Failed to delete profile")
				return
			}

			// If current profile was deleted, switch to default
			if a.currentProfile == profileName {
				a.currentProfile = "default"
				a.updateTunnelList()
				a.updateHeaderBar()
			}

			a.updateStatusBar(fmt.Sprintf("✓ Deleted profile: %s", profileName))
		}

		a.pages.RemovePage("profile-mgmt")
		a.app.SetFocus(a.tunnelList)
	})

	form.AddButton("Cancel", func() {
		a.pages.RemovePage("profile-mgmt")
		a.app.SetFocus(a.tunnelList)
	})

	// Set form styles
	form.SetButtonBackgroundColor(tcell.ColorBlue)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(tcell.ColorYellow)

	modal := a.createModalOverlay(form, 50, 12)
	a.pages.AddPage("profile-mgmt", modal, true, true)
	a.app.SetFocus(form)
}

// showSSHConfigImport shows the SSH config import dialog
func (a *App) showSSHConfigImport() {
	// Load available SSH hosts
	hosts, err := a.tunnelManager.LoadSSHConfigHosts()
	if err != nil {
		a.showErrorModal("Error", fmt.Sprintf("Failed to load SSH config: %v", err))
		return
	}

	if len(hosts) == 0 {
		a.showErrorModal("No Hosts", "No hosts found in SSH config")
		return
	}

	// Create form for host selection
	form := tview.NewForm()
	form.SetBorder(true).
		SetTitle(" Import from SSH Config ").
		SetTitleAlign(tview.AlignCenter)

	// Add dropdown for host selection
	form.AddDropDown("Select Host", hosts, 0, nil)

	// Load existing profiles for selection
	config, _ := a.configStore.LoadConfig()
	profileOptions := []string{"default", "ssh-config"}
	for _, p := range config.Profiles {
		if p.Name != "default" && p.Name != "ssh-config" {
			profileOptions = append(profileOptions, p.Name)
		}
	}

	// Find the index for "ssh-config" profile
	defaultProfileIndex := 1 // "ssh-config"
	for i, p := range profileOptions {
		if p == "ssh-config" {
			defaultProfileIndex = i
			break
		}
	}

	// Add profile selection dropdown
	form.AddDropDown("Import to Profile", profileOptions, defaultProfileIndex, nil)

	// Add input field for new profile name
	form.AddInputField("Or Create New Profile", "", 30, nil, nil)

	// Set InputCapture to prevent global key handlers from interfering
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Allow ESC to close the form
		if event.Key() == tcell.KeyEscape {
			a.pages.RemovePage("ssh-import")
			a.app.SetFocus(a.tunnelList)
			return nil
		}
		// Let the form handle all other input
		return event
	})

	form.AddButton("Import", func() {
		_, selectedHost := form.GetFormItemByLabel("Select Host").(*tview.DropDown).GetCurrentOption()

		// Get selected or new profile
		newProfileName := form.GetFormItemByLabel("Or Create New Profile").(*tview.InputField).GetText()
		var targetProfile string

		if newProfileName != "" {
			targetProfile = newProfileName
		} else {
			_, targetProfile = form.GetFormItemByLabel("Import to Profile").(*tview.DropDown).GetCurrentOption()
		}

		// Import tunnels from selected host
		imported, err := a.tunnelManager.ImportFromSSHConfig(selectedHost)
		if err != nil {
			a.showErrorModal("Import Failed", err.Error())
		} else {
			// Update profile for imported tunnels
			for _, tunnel := range imported {
				tunnel.Profile = targetProfile
				a.tunnelManager.UpdateTunnel(tunnel)
			}

			a.updateTunnelList()
			a.updateStatusBar(fmt.Sprintf("✓ Imported %d tunnel(s) from %s to profile '%s'", len(imported), selectedHost, targetProfile))
		}

		a.pages.RemovePage("ssh-import")
		a.app.SetFocus(a.tunnelList)
	})

	form.AddButton("Cancel", func() {
		a.pages.RemovePage("ssh-import")
		a.app.SetFocus(a.tunnelList)
	})

	// Set form styles
	form.SetButtonBackgroundColor(tcell.ColorBlue)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(tcell.ColorYellow)

	modal := a.createModalOverlay(form, 60, 12)
	a.pages.AddPage("ssh-import", modal, true, true)
	a.app.SetFocus(form)
}

// Removed - helper functions no longer needed