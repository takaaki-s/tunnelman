package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove a tunnel",
	Args:  cobra.ExactArgs(1),
	RunE:  runRm,
}

func init() {
	rootCmd.AddCommand(rmCmd)
}

func runRm(cmd *cobra.Command, args []string) error {
	client := newClient()
	if err := client.Remove(args[0]); err != nil {
		outputError(1, err.Error())
		return nil
	}
	outputResult(fmt.Sprintf("Removed tunnel %s", args[0]))
	return nil
}
