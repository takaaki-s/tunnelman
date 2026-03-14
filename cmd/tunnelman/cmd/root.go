package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/tunnelman/internal/config"
	"github.com/takaaki-s/tunnelman/internal/daemon"
)

var (
	jsonOutput bool
	socketPath string
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "tunnelman",
	Short: "SSH tunnel manager",
	Long:  "tunnelman manages SSH tunnels through a daemon process.",
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVar(&socketPath, "socket", "", "Path to daemon socket")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func outputResult(data any) {
	if jsonOutput {
		out := map[string]any{"success": true, "data": data}
		_ = json.NewEncoder(os.Stdout).Encode(out)
	} else {
		switch v := data.(type) {
		case string:
			fmt.Println(v)
		case fmt.Stringer:
			fmt.Println(v.String())
		default:
			fmt.Printf("%v\n", v)
		}
	}
}

func outputError(code int, msg string) {
	if jsonOutput {
		out := map[string]any{"success": false, "error": msg, "code": code}
		_ = json.NewEncoder(os.Stdout).Encode(out)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	}
	os.Exit(code)
}

// newClient creates a daemon client with resolved socket path.
func newClient() *daemon.Client {
	sp := socketPath
	if sp == "" {
		sp = config.DefaultSocketPath()
	}
	return daemon.NewClient(sp)
}

// resolveConfigPath returns the config path, defaulting to XDG location.
func resolveConfigPath() string {
	if configPath != "" {
		return configPath
	}
	return config.DefaultConfigPath()
}
