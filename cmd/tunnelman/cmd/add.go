package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/tunnelman/internal/daemon"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new tunnel",
	RunE:  runAdd,
}

func init() {
	addCmd.Flags().String("name", "", "Tunnel name (required)")
	addCmd.Flags().String("type", "local", "Tunnel type: local, remote, dynamic")
	addCmd.Flags().String("ssh-host", "", "SSH host (required)")
	addCmd.Flags().String("local-host", "127.0.0.1", "Local bind host")
	addCmd.Flags().Int("local-port", 0, "Local port (required)")
	addCmd.Flags().String("remote-host", "", "Remote host")
	addCmd.Flags().Int("remote-port", 0, "Remote port")
	addCmd.Flags().String("profile", "", "Profile name")
	addCmd.Flags().Bool("auto-connect", false, "Auto-connect on daemon start")
	addCmd.Flags().StringSlice("ssh-option", nil, "Additional SSH options")

	_ = addCmd.MarkFlagRequired("name")
	_ = addCmd.MarkFlagRequired("ssh-host")
	_ = addCmd.MarkFlagRequired("local-port")

	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	tunnelType, _ := cmd.Flags().GetString("type")
	sshHost, _ := cmd.Flags().GetString("ssh-host")
	localHost, _ := cmd.Flags().GetString("local-host")
	localPort, _ := cmd.Flags().GetInt("local-port")
	remoteHost, _ := cmd.Flags().GetString("remote-host")
	remotePort, _ := cmd.Flags().GetInt("remote-port")
	profile, _ := cmd.Flags().GetString("profile")
	autoConnect, _ := cmd.Flags().GetBool("auto-connect")
	sshOptions, _ := cmd.Flags().GetStringSlice("ssh-option")

	id := fmt.Sprintf("%s-%d", name, time.Now().UnixMilli())

	client := newClient()
	err := client.Add(daemon.AddRequest{
		ID:          id,
		Name:        name,
		Type:        tunnelType,
		SSHHost:     sshHost,
		LocalHost:   localHost,
		LocalPort:   localPort,
		RemoteHost:  remoteHost,
		RemotePort:  remotePort,
		Profile:     profile,
		AutoConnect: autoConnect,
		SSHOptions:  sshOptions,
	})
	if err != nil {
		outputError(1, err.Error())
		return nil
	}

	if jsonOutput {
		outputResult(map[string]string{"id": id, "name": name})
	} else {
		fmt.Printf("Added tunnel %q (ID: %s)\n", name, id)
	}
	return nil
}
