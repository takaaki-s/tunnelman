package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/takaaki-s/tunnelman/cmd/tunnelman/cmd/ui"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Interactive TUI for managing SSH tunnels",
	RunE:  runUI,
}

func init() {
	rootCmd.AddCommand(uiCmd)
}

func runUI(cmd *cobra.Command, args []string) error {
	client := newClient()
	if !client.IsRunning() {
		return fmt.Errorf("daemon is not running. Start with: tunnelman daemon start")
	}

	m := ui.NewModel(client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
