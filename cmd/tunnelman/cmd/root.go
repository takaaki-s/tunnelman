package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
		json.NewEncoder(os.Stdout).Encode(out)
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
		json.NewEncoder(os.Stdout).Encode(out)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	}
	os.Exit(code)
}
