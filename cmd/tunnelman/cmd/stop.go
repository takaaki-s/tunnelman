package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopAll bool

var stopCmd = &cobra.Command{
	Use:   "stop [id]",
	Short: "Stop a tunnel or all tunnels",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStop,
}

func init() {
	stopCmd.Flags().BoolVar(&stopAll, "all", false, "Stop all tunnels")
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	client := newClient()

	if stopAll {
		if err := client.StopAll(); err != nil {
			outputError(1, err.Error())
			return nil
		}
		outputResult("Stopped all tunnels")
		return nil
	}

	if len(args) == 0 {
		outputError(4, "tunnel ID required (or use --all)")
		return nil
	}

	if err := client.Stop(args[0]); err != nil {
		outputError(1, err.Error())
		return nil
	}
	outputResult(fmt.Sprintf("Stopped tunnel %s", args[0]))
	return nil
}
