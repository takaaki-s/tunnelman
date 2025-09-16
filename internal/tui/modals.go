// Package tui provides modal dialogs for the tunnelman TUI
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/takaaki-s/tunnelman/internal/core"
)

// Modal represents a modal dialog
type Modal struct {
	app   *App
	frame *tview.Frame
	form  *tview.Form
}

// showDeleteConfirmation shows a confirmation modal for deletion
func (a *App) showDeleteConfirmation(tunnel *core.Tunnel) {
	if tunnel == nil {
		return
	}

	// Create confirmation text
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf(
			"[yellow]⚠ Delete Confirmation[::-]\n\n"+
				"Are you sure you want to delete tunnel:\n\n"+
				"[white]%s[::-]\n"+
				"[dim](%s)[::-]\n\n"+
				"This action cannot be undone.",
			tunnel.Name,
			tunnel.SSHHost,
		))

	// Create buttons
	deleteBtn := tview.NewButton("Delete (D)").
		SetSelectedFunc(func() {
			if err := a.tunnelManager.DeleteTunnel(tunnel.ID); err != nil {
				a.showErrorModal("Delete Failed", err.Error())
			} else {
				a.selectedTunnel = nil
				a.updateTunnelList()
				a.updateDetailView(nil)
				a.updateStatusBar("✓ Tunnel deleted successfully")
			}
			a.pages.RemovePage("delete-confirm")
			a.app.SetFocus(a.tunnelList)
		})
	deleteBtn.SetBackgroundColor(tcell.ColorRed)

	cancelBtn := tview.NewButton("Cancel (C)").
		SetSelectedFunc(func() {
			a.pages.RemovePage("delete-confirm")
			a.app.SetFocus(a.tunnelList)
		})
	cancelBtn.SetBackgroundColor(tcell.ColorBlue)

	// Create button container with tab navigation support
	buttons := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(deleteBtn, 14, 0, true).
		AddItem(nil, 2, 0, false).
		AddItem(cancelBtn, 14, 0, false).
		AddItem(nil, 0, 1, false)

	// Track which button is focused
	currentFocus := 0 // 0 = delete, 1 = cancel

	// Create container
	container := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(text, 0, 1, false).
		AddItem(buttons, 3, 0, true)

	container.SetBorder(true).
		SetTitle(" Delete Tunnel ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorRed)

	// Set InputCapture to handle keyboard navigation
	container.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			// Close on ESC
			a.pages.RemovePage("delete-confirm")
			a.app.SetFocus(a.tunnelList)
			return nil
		case tcell.KeyTab:
			// Tab to next button
			currentFocus = (currentFocus + 1) % 2
			if currentFocus == 0 {
				a.app.SetFocus(deleteBtn)
			} else {
				a.app.SetFocus(cancelBtn)
			}
			return nil
		case tcell.KeyBacktab:
			// Shift+Tab to previous button
			currentFocus = (currentFocus - 1 + 2) % 2
			if currentFocus == 0 {
				a.app.SetFocus(deleteBtn)
			} else {
				a.app.SetFocus(cancelBtn)
			}
			return nil
		case tcell.KeyLeft:
			// Left arrow - focus delete button
			currentFocus = 0
			a.app.SetFocus(deleteBtn)
			return nil
		case tcell.KeyRight:
			// Right arrow - focus cancel button
			currentFocus = 1
			a.app.SetFocus(cancelBtn)
			return nil
		}

		// Handle character keys for button shortcuts
		switch event.Rune() {
		case 'd', 'D':
			// Delete shortcut
			if err := a.tunnelManager.DeleteTunnel(tunnel.ID); err != nil {
				a.showErrorModal("Delete Failed", err.Error())
			} else {
				a.selectedTunnel = nil
				a.updateTunnelList()
				a.updateDetailView(nil)
				a.updateStatusBar("✓ Tunnel deleted successfully")
			}
			a.pages.RemovePage("delete-confirm")
			a.app.SetFocus(a.tunnelList)
			return nil
		case 'c', 'C':
			// Cancel shortcut
			a.pages.RemovePage("delete-confirm")
			a.app.SetFocus(a.tunnelList)
			return nil
		}

		return event
	})

	// Create modal overlay
	modal := a.createModalOverlay(container, 50, 15)
	a.pages.AddPage("delete-confirm", modal, true, true)
	a.app.SetFocus(cancelBtn)  // Start with Cancel button focused for safety
	currentFocus = 1
}

// showAddTunnelForm shows the form for adding a new tunnel
func (a *App) showAddTunnelForm() {
	form := a.createAdvancedTunnelForm(nil)

	// Set InputCapture to prevent global key handlers from interfering
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Allow ESC to close the form
		if event.Key() == tcell.KeyEscape {
			a.pages.RemovePage("add-tunnel")
			a.app.SetFocus(a.tunnelList)
			return nil
		}
		// Let the form handle all other input
		return event
	})

	modal := a.createModalOverlay(form, 70, 25)
	a.pages.AddPage("add-tunnel", modal, true, true)
	a.app.SetFocus(form)
}

// createAdvancedTunnelForm creates an advanced tunnel configuration form
func (a *App) createAdvancedTunnelForm(tunnel *core.Tunnel) *tview.Form {
	isNew := tunnel == nil
	if isNew {
		tunnel = &core.Tunnel{
			ID:        core.NewTunnel("", core.LocalForward).ID,
			Type:      core.LocalForward,
			LocalHost: "0.0.0.0",
			LocalPort: 8080,
			RemoteHost: "localhost",
			RemotePort: 80,
		}
	}

	form := tview.NewForm()

	// Set form title and style
	title := " ✚ New Tunnel "
	if !isNew {
		title = " ✎ Edit Tunnel "
	}
	form.SetBorder(true).
		SetTitle(title).
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorGreen)

	// Track current tunnel type for dynamic field updates
	currentType := tunnel.Type

	// Basic Information Section
	form.AddTextView("Basic Information", "[yellow]Basic Information[::-]", 0, 1, true, false)

	form.AddInputField("Name", tunnel.Name, 40, nil, nil).
		SetFieldBackgroundColor(tcell.ColorBlack)

	typeOptions := []string{"Local Forward (-L)", "Remote Forward (-R)", "Dynamic/SOCKS (-D)"}
	typeIndex := 0
	switch tunnel.Type {
	case core.RemoteForward:
		typeIndex = 1
	case core.DynamicForward:
		typeIndex = 2
	}

	typeDropdown := form.AddDropDown("Type", typeOptions, typeIndex, func(option string, index int) {
		// Update currentType based on selection
		switch index {
		case 0:
			currentType = core.LocalForward
		case 1:
			currentType = core.RemoteForward
		case 2:
			currentType = core.DynamicForward
		}
		// Dynamically update form fields based on type
		a.updateFormFieldsForType(form, currentType)
	})
	typeDropdown.SetFieldBackgroundColor(tcell.ColorBlack)

	// SSH Connection Section
	form.AddTextView("", "", 0, 0, false, false) // Spacer
	form.AddTextView("SSH Connection", "[yellow]SSH Connection[::-]", 0, 1, true, false)

	form.AddInputField("SSH Host", tunnel.SSHHost, 40, nil, nil).
		SetFieldBackgroundColor(tcell.ColorBlack)

	// Port Forwarding Section
	form.AddTextView("", "", 0, 0, false, false) // Spacer
	form.AddTextView("Port Forwarding", "[yellow]Port Forwarding[::-]", 0, 1, true, false)

	form.AddInputField("Local Port", fmt.Sprintf("%d", tunnel.LocalPort), 10, func(textToCheck string, lastChar rune) bool {
		if textToCheck == "" {
			return true
		}
		_, err := strconv.Atoi(textToCheck)
		return err == nil
	}, nil).SetFieldBackgroundColor(tcell.ColorBlack)

	// Add remote fields only for non-dynamic tunnels
	if currentType != core.DynamicForward {
		form.AddInputField("Remote Host", tunnel.RemoteHost, 40, nil, nil).
			SetFieldBackgroundColor(tcell.ColorBlack)

		form.AddInputField("Remote Port", fmt.Sprintf("%d", tunnel.RemotePort), 10, func(textToCheck string, lastChar rune) bool {
			if textToCheck == "" {
				return true
			}
			_, err := strconv.Atoi(textToCheck)
			return err == nil
		}, nil).SetFieldBackgroundColor(tcell.ColorBlack)
	}

	// Options Section
	form.AddTextView("", "", 0, 0, false, false) // Spacer
	form.AddTextView("Options", "[yellow]Options[::-]", 0, 1, true, false)

	// Profile selection
	config, _ := a.configStore.LoadConfig()
	profileOptions := []string{"default"}
	for _, p := range config.Profiles {
		if p.Name != "default" {
			profileOptions = append(profileOptions, p.Name)
		}
	}

	// Find current profile index
	profileIndex := 0
	currentProfile := tunnel.Profile
	if currentProfile == "" {
		currentProfile = "default"
	}
	for i, p := range profileOptions {
		if p == currentProfile {
			profileIndex = i
			break
		}
	}

	form.AddDropDown("Profile", profileOptions, profileIndex, nil)

	form.AddCheckbox("Auto-connect on startup", tunnel.AutoConnect, nil)

	extraArgs := strings.Join(tunnel.ExtraArgs, " ")
	form.AddInputField("Extra SSH Arguments", extraArgs, 50, nil, nil).
		SetFieldBackgroundColor(tcell.ColorBlack)

	// Buttons
	form.AddButton("Save", func() {
		if err := a.saveTunnelFromAdvancedForm(form, isNew, tunnel.ID, currentType); err != nil {
			a.showErrorModal("Validation Error", err.Error())
			return
		}
		if isNew {
			a.pages.RemovePage("add-tunnel")
			a.updateStatusBar("✓ Tunnel created successfully")
		} else {
			a.pages.RemovePage("edit-tunnel")
			a.updateStatusBar("✓ Tunnel updated successfully")
		}
		a.app.SetFocus(a.tunnelList)
		a.updateTunnelList()
	})

	form.AddButton("Cancel", func() {
		if isNew {
			a.pages.RemovePage("add-tunnel")
		} else {
			a.pages.RemovePage("edit-tunnel")
		}
		a.app.SetFocus(a.tunnelList)
	})

	// Set button colors
	form.SetButtonBackgroundColor(tcell.ColorBlue)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(tcell.ColorYellow)

	return form
}

// updateFormFieldsForType updates form fields based on tunnel type
func (a *App) updateFormFieldsForType(form *tview.Form, tunnelType core.TunnelType) {
	// This is a simplified version - in a real implementation,
	// you would need to dynamically add/remove form fields
	// For now, we'll just update the help text
	switch tunnelType {
	case core.LocalForward:
		form.SetTitle(" ✚ New Tunnel - Local Forward (-L) ")
	case core.RemoteForward:
		form.SetTitle(" ✚ New Tunnel - Remote Forward (-R) ")
	case core.DynamicForward:
		form.SetTitle(" ✚ New Tunnel - Dynamic/SOCKS (-D) ")
	}
}

// saveTunnelFromAdvancedForm extracts and saves tunnel data from the advanced form
func (a *App) saveTunnelFromAdvancedForm(form *tview.Form, isNew bool, tunnelID string, tunnelType core.TunnelType) error {
	// Extract form values
	name := form.GetFormItemByLabel("Name").(*tview.InputField).GetText()
	sshHost := form.GetFormItemByLabel("SSH Host").(*tview.InputField).GetText()
	localPortStr := form.GetFormItemByLabel("Local Port").(*tview.InputField).GetText()
	_, profileName := form.GetFormItemByLabel("Profile").(*tview.DropDown).GetCurrentOption()
	autoConnect := form.GetFormItemByLabel("Auto-connect on startup").(*tview.Checkbox).IsChecked()
	extraArgsStr := form.GetFormItemByLabel("Extra SSH Arguments").(*tview.InputField).GetText()

	// Parse integers
	localPort, _ := strconv.Atoi(localPortStr)

	// Create tunnel object
	tunnel := &core.Tunnel{
		ID:          tunnelID,
		Name:        name,
		Type:        tunnelType,
		SSHHost:     sshHost,
		LocalHost:   "0.0.0.0",
		LocalPort:   localPort,
		Profile:     profileName,
		AutoConnect: autoConnect,
	}

	// Parse extra arguments
	if extraArgsStr != "" {
		tunnel.ExtraArgs = strings.Fields(extraArgsStr)
	}

	// Handle type-specific fields
	if tunnelType != core.DynamicForward {
		remoteHost := form.GetFormItemByLabel("Remote Host").(*tview.InputField).GetText()
		remotePortStr := form.GetFormItemByLabel("Remote Port").(*tview.InputField).GetText()
		remotePort, _ := strconv.Atoi(remotePortStr)

		tunnel.RemoteHost = remoteHost
		tunnel.RemotePort = remotePort
	}

	// Validate
	if err := tunnel.Validate(); err != nil {
		return err
	}

	// Save
	if isNew {
		return a.tunnelManager.AddTunnel(tunnel)
	}
	return a.tunnelManager.UpdateTunnel(tunnel)
}

// showErrorModal displays an error modal dialog
func (a *App) showErrorModal(title, message string) {
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf(
			"[red]✗ %s[::-]\n\n%s",
			title,
			message,
		))

	button := a.createButton("OK", func() {
		a.pages.RemovePage("error")
		a.app.SetFocus(a.tunnelList)
	})

	buttonContainer := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(button, 10, 0, true).
		AddItem(nil, 0, 1, false)

	container := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(text, 0, 1, false).
		AddItem(buttonContainer, 3, 0, true)

	container.SetBorder(true).
		SetTitle(" Error ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorRed)

	modal := a.createModalOverlay(container, 50, 12)
	a.pages.AddPage("error", modal, true, true)
	a.app.SetFocus(button)
}

// createButton creates a styled button
func (a *App) createButton(label string, handler func()) *tview.Button {
	button := tview.NewButton(label).
		SetSelectedFunc(handler)

	button.SetBackgroundColor(tcell.ColorBlue)

	return button
}

// createModalOverlay creates a modal overlay with dimmed background
func (a *App) createModalOverlay(content tview.Primitive, width, height int) *tview.Flex {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(content, width, 1, true).
			AddItem(nil, 0, 1, false), height, 1, true).
		AddItem(nil, 0, 1, false)
}