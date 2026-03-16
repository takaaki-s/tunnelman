package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/takaaki-s/tunnelman/internal/daemon"
)

const (
	colIDWidth      = 22
	colNameWidth    = 18
	colTypeWidth    = 8
	colLocalWidth   = 8
	colRemoteWidth  = 15
	colSSHHostWidth = 14
)

func truncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return fmt.Sprintf("%-*s", width, s)
	}
	return string(runes[:width-1]) + "…"
}

// RenderTableHeader renders the column header row.
func RenderTableHeader() string {
	return styleHeader.Render(
		truncate("ID", colIDWidth) + " " +
			truncate("NAME", colNameWidth) + " " +
			truncate("TYPE", colTypeWidth) + " " +
			truncate("LOCAL", colLocalWidth) + " " +
			truncate("REMOTE", colRemoteWidth) + " " +
			truncate("SSH HOST", colSSHHostWidth) + " " +
			"STATUS",
	)
}

// RenderTable renders the full tunnel table with scroll offset.
func RenderTable(tunnels []daemon.TunnelInfo, cursor, offset, height int) string {
	if len(tunnels) == 0 {
		return styleInfo.Render("  No tunnels found. Press 'a' to add one.")
	}

	var sb strings.Builder
	for i := offset; i < len(tunnels) && i < offset+height; i++ {
		if i > offset {
			sb.WriteString("\n")
		}
		sb.WriteString(renderTunnelRow(tunnels[i], i == cursor))
	}
	return sb.String()
}

func renderTunnelRow(t daemon.TunnelInfo, selected bool) string {
	local := strconv.Itoa(t.LocalPort)
	remote := strconv.Itoa(t.RemotePort)
	if t.Type == "dynamic" {
		remote = "-"
	}

	icon := statusIcon(t.Status)
	statusText := icon + " " + t.Status

	base := truncate(t.ID, colIDWidth) + " " +
		truncate(t.Name, colNameWidth) + " " +
		truncate(t.Type, colTypeWidth) + " " +
		truncate(local, colLocalWidth) + " " +
		truncate(remote, colRemoteWidth) + " " +
		truncate(t.SSHHost, colSSHHostWidth) + " "

	if selected {
		return styleSelected.Render(base + statusText)
	}
	return base + statusStyle(t.Status).Render(statusText)
}
