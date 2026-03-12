package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var startAll bool

var startCmd = &cobra.Command{
	Use:   "start [id]",
	Short: "Start a tunnel or all tunnels",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStart,
}

func init() {
	startCmd.Flags().BoolVar(&startAll, "all", false, "Start all tunnels")
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	client := newClient()

	if startAll {
		if err := client.StartAll(); err != nil {
			outputError(1, err.Error())
			return nil
		}
		outputResult("Started all tunnels")
		return nil
	}

	if len(args) == 0 {
		outputError(4, "tunnel ID required (or use --all)")
		return nil
	}

	if err := client.Start(args[0]); err != nil {
		outputError(1, err.Error())
		return nil
	}
	outputResult(fmt.Sprintf("Started tunnel %s", args[0]))
	return nil
}
