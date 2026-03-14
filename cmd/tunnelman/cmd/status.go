package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <id>",
	Short: "Show tunnel status",
	Args:  cobra.ExactArgs(1),
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	client := newClient()
	info, err := client.Status(args[0])
	if err != nil {
		outputError(3, err.Error())
		return nil
	}

	if jsonOutput {
		outputResult(info)
		return nil
	}

	fmt.Printf("ID:          %s\n", info.ID)
	fmt.Printf("Name:        %s\n", info.Name)
	fmt.Printf("Type:        %s\n", info.Type)
	fmt.Printf("SSH Host:    %s\n", info.SSHHost)
	fmt.Printf("Local:       %s:%d\n", info.LocalHost, info.LocalPort)
	if info.Type != "dynamic" {
		fmt.Printf("Remote:      %s:%d\n", info.RemoteHost, info.RemotePort)
	}
	fmt.Printf("Profile:     %s\n", info.Profile)
	fmt.Printf("Status:      %s\n", info.Status)
	if info.PID > 0 {
		fmt.Printf("PID:         %d\n", info.PID)
	}
	return nil
}
