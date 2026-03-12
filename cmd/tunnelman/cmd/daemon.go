package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/tunnelman/internal/config"
	"github.com/takaaki-s/tunnelman/internal/daemon"
)

var foreground bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the tunnelman daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon",
	RunE:  runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE:  runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE:  runDaemonStatusCmd,
}

func init() {
	daemonStartCmd.Flags().BoolVar(&foreground, "foreground", false, "Run in foreground")
	_ = daemonStartCmd.Flags().MarkHidden("foreground")

	daemonCmd.AddCommand(daemonStartCmd, daemonStopCmd, daemonStatusCmd)
	rootCmd.AddCommand(daemonCmd)
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	if foreground {
		return runDaemonForeground()
	}
	return runDaemonBackground()
}

func runDaemonForeground() error {
	sp := socketPath
	if sp == "" {
		sp = config.DefaultSocketPath()
	}
	cp := resolveConfigPath()

	cfg, err := config.LoadConfig(cp)
	if err != nil {
		outputError(1, fmt.Sprintf("failed to load config: %v", err))
		return nil
	}

	stateDir := filepath.Dir(config.DefaultStatePath())
	sm, err := config.NewStateManager(stateDir)
	if err != nil {
		outputError(1, fmt.Sprintf("failed to init state: %v", err))
		return nil
	}

	srv := daemon.NewServer(sp, cfg, cp, sm)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "Received %s, shutting down...\n", sig)
		srv.Stop()
	case err := <-errCh:
		if err != nil {
			outputError(1, fmt.Sprintf("daemon error: %v", err))
		}
	}
	return nil
}

func runDaemonBackground() error {
	// Check if already running
	client := newClient()
	if client.IsRunning() {
		outputError(1, "daemon is already running")
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		outputError(1, fmt.Sprintf("failed to get executable path: %v", err))
		return nil
	}

	daemonArgs := []string{"daemon", "start", "--foreground"}
	if socketPath != "" {
		daemonArgs = append(daemonArgs, "--socket", socketPath)
	}
	if configPath != "" {
		daemonArgs = append(daemonArgs, "--config", configPath)
	}

	child := exec.Command(exePath, daemonArgs...)
	child.Stdout = nil
	child.Stderr = nil
	child.Stdin = nil
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := child.Start(); err != nil {
		outputError(1, fmt.Sprintf("failed to start daemon: %v", err))
		return nil
	}

	// Wait for daemon to become reachable
	for i := 0; i < 50; i++ {
		if client.IsRunning() {
			outputResult("Daemon started")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	outputError(1, "daemon started but not reachable")
	return nil
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	client := newClient()
	if !client.IsRunning() {
		outputError(2, "daemon is not running")
		return nil
	}
	if err := client.Shutdown(); err != nil {
		outputError(1, err.Error())
		return nil
	}
	outputResult("Daemon stopped")
	return nil
}

func runDaemonStatusCmd(cmd *cobra.Command, args []string) error {
	client := newClient()
	ds, err := client.DaemonStatus()
	if err != nil {
		outputError(2, err.Error())
		return nil
	}
	if jsonOutput {
		outputResult(ds)
	} else {
		fmt.Printf("PID:        %d\n", ds.PID)
		fmt.Printf("Uptime:     %s\n", ds.Uptime)
		fmt.Printf("Tunnels:    %d\n", ds.Tunnels)
		fmt.Printf("Running:    %d\n", ds.Running)
		fmt.Printf("Config:     %s\n", ds.ConfigPath)
	}
	return nil
}
