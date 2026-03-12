package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/tunnelman/internal/daemon"
	"github.com/takaaki-s/tunnelman/internal/sshconfig"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import tunnels from SSH config",
	RunE:  runImport,
}

func init() {
	importCmd.Flags().String("ssh-config", "", "Path to SSH config file (default: ~/.ssh/config)")
	importCmd.Flags().String("host", "", "SSH host alias to import (required)")
	importCmd.Flags().String("profile", "", "Assign imported tunnels to a profile")

	_ = importCmd.MarkFlagRequired("host")

	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	sshConfigPath, _ := cmd.Flags().GetString("ssh-config")
	host, _ := cmd.Flags().GetString("host")
	profile, _ := cmd.Flags().GetString("profile")

	parser := sshconfig.NewParser(sshConfigPath)
	hostInfo, err := parser.ParseHost(host)
	if err != nil {
		outputError(1, fmt.Sprintf("failed to parse SSH config: %v", err))
		return nil
	}
	if hostInfo == nil {
		outputError(3, fmt.Sprintf("host %q not found in SSH config", host))
		return nil
	}

	client := newClient()
	imported := 0

	// Import local forwards
	for _, fwd := range hostInfo.LocalForwards {
		id := fmt.Sprintf("%s-L%d-%d", host, fwd.BindPort, time.Now().UnixMilli())
		name := fmt.Sprintf("%s-L%d", host, fwd.BindPort)
		err := client.Add(daemon.AddRequest{
			ID:         id,
			Name:       name,
			Type:       "local",
			SSHHost:    host,
			LocalHost:  fwd.BindAddress,
			LocalPort:  fwd.BindPort,
			RemoteHost: fwd.Host,
			RemotePort: fwd.HostPort,
			Profile:    profile,
		})
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to import local forward %d: %v\n", fwd.BindPort, err)
			continue
		}
		imported++
	}

	// Import remote forwards
	for _, fwd := range hostInfo.RemoteForwards {
		id := fmt.Sprintf("%s-R%d-%d", host, fwd.BindPort, time.Now().UnixMilli())
		name := fmt.Sprintf("%s-R%d", host, fwd.BindPort)
		err := client.Add(daemon.AddRequest{
			ID:         id,
			Name:       name,
			Type:       "remote",
			SSHHost:    host,
			LocalHost:  fwd.Host,
			LocalPort:  fwd.HostPort,
			RemoteHost: fwd.BindAddress,
			RemotePort: fwd.BindPort,
			Profile:    profile,
		})
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to import remote forward %d: %v\n", fwd.BindPort, err)
			continue
		}
		imported++
	}

	// Import dynamic forwards
	for _, dyn := range hostInfo.DynamicForwards {
		id := fmt.Sprintf("%s-D%d-%d", host, dyn.BindPort, time.Now().UnixMilli())
		name := fmt.Sprintf("%s-D%d", host, dyn.BindPort)
		err := client.Add(daemon.AddRequest{
			ID:        id,
			Name:      name,
			Type:      "dynamic",
			SSHHost:   host,
			LocalHost: dyn.BindAddress,
			LocalPort: dyn.BindPort,
			Profile:   profile,
		})
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to import dynamic forward %d: %v\n", dyn.BindPort, err)
			continue
		}
		imported++
	}

	if jsonOutput {
		outputResult(map[string]int{"imported": imported})
	} else {
		fmt.Printf("Imported %d tunnel(s) from host %q\n", imported, host)
	}
	return nil
}
