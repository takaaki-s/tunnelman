package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/takaaki-s/tunnelman/internal/daemon"
)

// RenderDetail renders the detail panel for the selected tunnel.
func RenderDetail(t *daemon.TunnelInfo, width int) string {
	if t == nil {
		return styleBorder.Width(width - 2).Render("  No tunnel selected.")
	}

	autoConnect := strconv.FormatBool(t.AutoConnect)

	pid := "-"
	if t.PID > 0 {
		pid = fmt.Sprintf("%d", t.PID)
	}

	profile := t.Profile
	if profile == "" {
		profile = "(none)"
	}

	lines := []string{
		fmt.Sprintf("  ID: %-20s  Name: %s", t.ID, t.Name),
		fmt.Sprintf("  Profile: %-14s  AutoConnect: %-6s  PID: %s", profile, autoConnect, pid),
		fmt.Sprintf("  Type: %-16s  Reconnect Count: %d", t.Type, t.ReconnectCount),
		fmt.Sprintf("  SSH Host: %s", t.SSHHost),
	}

	content := strings.Join(lines, "\n")
	title := fmt.Sprintf(" Detail: %s ", t.Name)
	return styleBorder.Width(width - 2).Render(
		styleHeader.Render(title) + "\n" + content,
	)
}
