package ui

import "strings"

// RenderProfileTabs renders the profile tab bar.
func RenderProfileTabs(profiles []string, selected int) string {
	var sb strings.Builder
	for i, p := range profiles {
		if i > 0 {
			sb.WriteString(" ")
		}
		if i == selected {
			sb.WriteString(styleTabActive.Render(p))
		} else {
			sb.WriteString(styleTabInactive.Render(p))
		}
	}
	return sb.String()
}
