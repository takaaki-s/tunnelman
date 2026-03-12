package cmd

import (
	"fmt"
	"text/tabwriter"
	"os"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/tunnelman/internal/daemon"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tunnels",
	RunE:  runList,
}

func init() {
	listCmd.Flags().String("profile", "", "Filter by profile")
	listCmd.Flags().String("status", "", "Filter by status")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	profile, _ := cmd.Flags().GetString("profile")
	status, _ := cmd.Flags().GetString("status")

	client := newClient()
	lr, err := client.List(daemon.ListRequest{Profile: profile, Status: status})
	if err != nil {
		outputError(2, err.Error())
		return nil
	}

	if jsonOutput {
		outputResult(lr)
		return nil
	}

	if len(lr.Tunnels) == 0 {
		fmt.Println("No tunnels configured")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tLOCAL\tREMOTE\tSSH HOST\tSTATUS")
	for _, t := range lr.Tunnels {
		local := fmt.Sprintf("%s:%d", t.LocalHost, t.LocalPort)
		remote := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
		if t.Type == "dynamic" {
			remote = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			t.ID, t.Name, t.Type, local, remote, t.SSHHost, t.Status)
	}
	w.Flush()
	return nil
}
