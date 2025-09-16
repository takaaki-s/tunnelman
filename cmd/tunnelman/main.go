// Package main provides the entry point for the tunnelman SSH tunnel manager application.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/takaaki-s/tunnelman/internal/core"
	"github.com/takaaki-s/tunnelman/internal/store"
	"github.com/takaaki-s/tunnelman/internal/tui"
)

// version information, set at build time
var (
	version = "1.0.0"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Parse command-line flags
	var (
		showVersion  = flag.Bool("version", false, "Show version information")
		configPath   = flag.String("config", "", "Path to config file (default: ~/.config/tunnelman/config.json)")
		debug        = flag.Bool("debug", false, "Enable debug mode (verbose logging)")
		autoProfile  = flag.String("auto", "", "Auto-connect tunnels in specified profile")
		listProfiles = flag.Bool("list-profiles", false, "List available profiles")
		profile      = flag.String("profile", "default", "Initial profile to load")
	)
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("tunnelman %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Initialize logger with debug mode
	core.InitLogger(*debug)

	// Initialize configuration store
	configStore, err := store.NewConfigStore(*configPath)
	if err != nil {
		core.Error("Failed to initialize config store: %v", err)
		os.Exit(1)
	}

	// Handle list-profiles flag
	if *listProfiles {
		config, err := configStore.LoadConfig()
		if err != nil {
			core.Error("Failed to load config: %v", err)
			os.Exit(1)
		}
		if len(config.Profiles) == 0 {
			fmt.Println("No profiles configured")
		} else {
			fmt.Println("Available profiles:")
			for _, p := range config.Profiles {
				fmt.Printf("  - %s\n", p.Name)
			}
		}
		os.Exit(0)
	}

	// Initialize PID store for tracking running tunnels
	pidStore, err := store.NewPIDStore()
	if err != nil {
		core.Error("Failed to initialize PID store: %v", err)
		os.Exit(1)
	}

	// Initialize tunnel manager with debug mode if specified
	var tunnelManagerOpts []core.TunnelManagerOption
	if *debug {
		tunnelManagerOpts = append(tunnelManagerOpts, core.WithDebugMode(true))
	}
	tunnelManager := core.NewTunnelManager(configStore, pidStore, tunnelManagerOpts...)

	// Handle auto-connect profile
	if *autoProfile != "" {
		core.Info("Starting all tunnels in profile: %s", *autoProfile)
		if err := tunnelManager.StartProfileTunnels(*autoProfile); err != nil {
			core.Error("Failed to start tunnels: %v", err)
			os.Exit(1)
		}
		core.Info("Successfully started tunnels in profile: %s", *autoProfile)
		// Exit after auto-connecting, don't start TUI
		os.Exit(0)
	}

	// Setup signal handlers for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create and run TUI application in a goroutine
	app := tui.NewApp(tunnelManager, configStore)
	app.SetInitialProfile(*profile)

	appErr := make(chan error, 1)
	go func() {
		if err := app.Run(); err != nil {
			appErr <- err
		}
		close(appErr)
	}()

	// Wait for either app to finish or signal
	select {
	case err := <-appErr:
		if err != nil {
			core.Error("Application error: %v", err)
			os.Exit(1)
		}
	case sig := <-sigChan:
		core.Info("Received signal: %v", sig)
		// Stop the TUI but keep tunnels running
		app.Stop()
		// Give TUI time to clean up
		time.Sleep(100 * time.Millisecond)
	}

	// Clean shutdown - tunnels keep running unless explicitly stopped
	core.Info("Tunnelman exiting. SSH tunnels remain running.")
	core.Info("To stop all tunnels, run: tunnelman --stop-all")
}

// handleStopAll stops all running tunnels
func handleStopAll(tunnelManager *core.TunnelManager) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	core.Info("Stopping all running tunnels...")
	if err := tunnelManager.StopAllTunnels(ctx); err != nil {
		core.Error("Failed to stop all tunnels: %v", err)
		os.Exit(1)
	}
	core.Info("All tunnels stopped")
}