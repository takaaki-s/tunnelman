package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/tunnelman/internal/daemon"
)

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit an existing tunnel",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdit,
}

func init() {
	editCmd.Flags().String("name", "", "New tunnel name")
	editCmd.Flags().String("type", "", "New tunnel type")
	editCmd.Flags().String("ssh-host", "", "New SSH host")
	editCmd.Flags().String("local-host", "", "New local host")
	editCmd.Flags().Int("local-port", 0, "New local port")
	editCmd.Flags().String("remote-host", "", "New remote host")
	editCmd.Flags().Int("remote-port", 0, "New remote port")
	editCmd.Flags().String("profile", "", "New profile")
	editCmd.Flags().Bool("auto-connect", false, "Auto-connect on daemon start")
	editCmd.Flags().StringSlice("ssh-option", nil, "SSH options (replaces all)")

	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	req := daemon.EditRequest{ID: args[0]}

	if cmd.Flags().Changed("name") {
		v, _ := cmd.Flags().GetString("name")
		req.Name = &v
	}
	if cmd.Flags().Changed("type") {
		v, _ := cmd.Flags().GetString("type")
		req.Type = &v
	}
	if cmd.Flags().Changed("ssh-host") {
		v, _ := cmd.Flags().GetString("ssh-host")
		req.SSHHost = &v
	}
	if cmd.Flags().Changed("local-host") {
		v, _ := cmd.Flags().GetString("local-host")
		req.LocalHost = &v
	}
	if cmd.Flags().Changed("local-port") {
		v, _ := cmd.Flags().GetInt("local-port")
		req.LocalPort = &v
	}
	if cmd.Flags().Changed("remote-host") {
		v, _ := cmd.Flags().GetString("remote-host")
		req.RemoteHost = &v
	}
	if cmd.Flags().Changed("remote-port") {
		v, _ := cmd.Flags().GetInt("remote-port")
		req.RemotePort = &v
	}
	if cmd.Flags().Changed("profile") {
		v, _ := cmd.Flags().GetString("profile")
		req.Profile = &v
	}
	if cmd.Flags().Changed("auto-connect") {
		v, _ := cmd.Flags().GetBool("auto-connect")
		req.AutoConnect = &v
	}
	if cmd.Flags().Changed("ssh-option") {
		v, _ := cmd.Flags().GetStringSlice("ssh-option")
		req.SSHOptions = v
	}

	client := newClient()
	if err := client.Edit(req); err != nil {
		outputError(1, err.Error())
		return nil
	}
	outputResult(fmt.Sprintf("Updated tunnel %s", args[0]))
	return nil
}
