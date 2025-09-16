// Package core provides tunnel management functionality
package core

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/takaaki-s/tunnelman/internal/store"
)

// TunnelManager manages the lifecycle of SSH tunnels
type TunnelManager struct {
	tunnels     map[string]*Tunnel
	configStore *store.ConfigStore
	pidStore    *store.PIDStore
	mu          sync.RWMutex

	// Process manager for SSH connections
	processManager *ProcessManager

	// Debug mode flag
	debug bool

	// Event channels for UI updates
	statusChanges chan TunnelStatusChange
}

// TunnelStatusChange represents a tunnel status change event
type TunnelStatusChange struct {
	TunnelID string
	OldStatus TunnelStatus
	NewStatus TunnelStatus
	Error     error
}

// TunnelManagerOption is a functional option for TunnelManager
type TunnelManagerOption func(*TunnelManager)

// WithDebugMode enables debug mode for the tunnel manager
func WithDebugMode(debug bool) TunnelManagerOption {
	return func(tm *TunnelManager) {
		tm.debug = debug
	}
}

// NewTunnelManager creates a new tunnel manager instance
func NewTunnelManager(configStore *store.ConfigStore, pidStore *store.PIDStore, opts ...TunnelManagerOption) *TunnelManager {
	tm := &TunnelManager{
		tunnels:       make(map[string]*Tunnel),
		configStore:   configStore,
		pidStore:      pidStore,
		statusChanges: make(chan TunnelStatusChange, 100),
	}

	// Apply options
	for _, opt := range opts {
		opt(tm)
	}

	// Initialize process manager with debug mode
	tm.processManager = NewProcessManager(WithDebug(tm.debug))

	// Load tunnels from config
	tm.loadTunnels()

	// Restore running tunnel states from PID store
	tm.restoreTunnelStates()

	return tm
}

// GetTunnels returns a list of all tunnels sorted by name
func (tm *TunnelManager) GetTunnels() []*Tunnel {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tunnels := make([]*Tunnel, 0, len(tm.tunnels))
	for _, t := range tm.tunnels {
		tunnels = append(tunnels, t.Clone())
	}

	// Sort tunnels by name for consistent ordering
	sort.Slice(tunnels, func(i, j int) bool {
		return tunnels[i].Name < tunnels[j].Name
	})

	return tunnels
}

// GetTunnel returns a specific tunnel by ID
func (tm *TunnelManager) GetTunnel(id string) (*Tunnel, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tunnel, exists := tm.tunnels[id]
	if !exists {
		return nil, fmt.Errorf("tunnel not found: %s", id)
	}
	return tunnel.Clone(), nil
}

// AddTunnel adds a new tunnel configuration
func (tm *TunnelManager) AddTunnel(tunnel *Tunnel) error {
	if err := tunnel.Validate(); err != nil {
		return fmt.Errorf("invalid tunnel configuration: %w", err)
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tunnels[tunnel.ID]; exists {
		return fmt.Errorf("tunnel with ID %s already exists", tunnel.ID)
	}

	tm.tunnels[tunnel.ID] = tunnel

	// Save to config store
	if err := tm.saveTunnels(); err != nil {
		delete(tm.tunnels, tunnel.ID)
		return fmt.Errorf("failed to save tunnel: %w", err)
	}

	return nil
}

// UpdateTunnel updates an existing tunnel configuration
func (tm *TunnelManager) UpdateTunnel(tunnel *Tunnel) error {
	if err := tunnel.Validate(); err != nil {
		return fmt.Errorf("invalid tunnel configuration: %w", err)
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	existing, exists := tm.tunnels[tunnel.ID]
	if !exists {
		return fmt.Errorf("tunnel not found: %s", tunnel.ID)
	}

	// Don't allow updating a running tunnel
	if existing.Status == StatusRunning {
		return fmt.Errorf("cannot update running tunnel")
	}

	tm.tunnels[tunnel.ID] = tunnel

	// Save to config store
	if err := tm.saveTunnels(); err != nil {
		tm.tunnels[tunnel.ID] = existing
		return fmt.Errorf("failed to save tunnel: %w", err)
	}

	return nil
}

// DeleteTunnel removes a tunnel configuration
func (tm *TunnelManager) DeleteTunnel(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tunnel, exists := tm.tunnels[id]
	if !exists {
		return fmt.Errorf("tunnel not found: %s", id)
	}

	// Don't allow deleting a running tunnel
	if tunnel.Status == StatusRunning {
		return fmt.Errorf("cannot delete running tunnel")
	}

	delete(tm.tunnels, id)

	// Save to config store
	if err := tm.saveTunnels(); err != nil {
		tm.tunnels[id] = tunnel
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// StartTunnel starts an SSH tunnel
func (tm *TunnelManager) StartTunnel(id string) error {
	tm.mu.Lock()
	tunnel, exists := tm.tunnels[id]
	if !exists {
		tm.mu.Unlock()
		return fmt.Errorf("tunnel not found: %s", id)
	}

	if tunnel.Status == StatusRunning {
		tm.mu.Unlock()
		return fmt.Errorf("tunnel is already running")
	}

	// Update status
	oldStatus := tunnel.Status
	tunnel.Status = StatusConnecting
	tm.mu.Unlock()

	// Notify status change
	tm.notifyStatusChange(id, oldStatus, StatusConnecting, nil)

	// Use process manager to connect
	pidEntry, err := tm.processManager.Connect(tunnel)
	if err != nil {
		tm.mu.Lock()
		tunnel.Status = StatusError
		tunnel.LastError = err
		tm.mu.Unlock()

		// Log the failure
		Error("FAILED to start tunnel '%s': %v", tunnel.Name, err)

		tm.notifyStatusChange(id, StatusConnecting, StatusError, err)
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	// Update tunnel state
	tm.mu.Lock()
	tunnel.PID = pidEntry.PID
	now := time.Now()
	tunnel.StartedAt = &now
	tunnel.Status = StatusRunning
	tunnel.LastError = nil

	// Get process info for monitoring
	if processInfo, exists := tm.processManager.GetProcessInfo(id); exists {
		tunnel.process = processInfo.Cmd
	}
	tm.mu.Unlock()

	// Save PID for recovery
	if err := tm.pidStore.AddPid(id, pidEntry.PID); err != nil {
		// Log error but don't fail the start
		if tm.debug {
			fmt.Printf("Warning: failed to save PID: %v\n", err)
		}
	}

	// Notify status change
	tm.notifyStatusChange(id, StatusConnecting, StatusRunning, nil)

	// Monitor the process in a goroutine
	go tm.monitorTunnel(id)

	return nil
}

// StopTunnel stops a running SSH tunnel
func (tm *TunnelManager) StopTunnel(id string) error {
	tm.mu.Lock()
	tunnel, exists := tm.tunnels[id]
	if !exists {
		tm.mu.Unlock()
		return fmt.Errorf("tunnel not found: %s", id)
	}

	if tunnel.Status != StatusRunning {
		tm.mu.Unlock()
		return fmt.Errorf("tunnel is not running")
	}

	pid := tunnel.PID
	oldStatus := tunnel.Status
	tm.mu.Unlock()

	// Use process manager to disconnect
	if err := tm.processManager.Disconnect(id, pid); err != nil {
		// Log error but continue with cleanup
		if tm.debug {
			fmt.Printf("Warning: error disconnecting tunnel %s: %v\n", id, err)
		}
	}

	// Update tunnel state
	tm.mu.Lock()
	tunnel.Status = StatusStopped
	tunnel.process = nil
	tunnel.PID = 0
	tunnel.StartedAt = nil
	tm.mu.Unlock()

	// Remove PID from store
	tm.pidStore.RemovePid(id)

	// Notify status change
	tm.notifyStatusChange(id, oldStatus, StatusStopped, nil)

	return nil
}

// RestartTunnel restarts a tunnel
func (tm *TunnelManager) RestartTunnel(id string) error {
	// Stop if running
	tunnel, err := tm.GetTunnel(id)
	if err != nil {
		return err
	}

	if tunnel.Status == StatusRunning {
		if err := tm.StopTunnel(id); err != nil {
			return fmt.Errorf("failed to stop tunnel: %w", err)
		}

		// Wait a moment for clean shutdown
		time.Sleep(500 * time.Millisecond)
	}

	// Start the tunnel
	return tm.StartTunnel(id)
}

// StartAutoConnectTunnels starts all tunnels marked for auto-connect
func (tm *TunnelManager) StartAutoConnectTunnels() {
	tm.mu.RLock()
	tunnels := make([]*Tunnel, 0)
	for _, t := range tm.tunnels {
		if t.AutoConnect && t.Status == StatusStopped {
			tunnels = append(tunnels, t)
		}
	}
	tm.mu.RUnlock()

	for _, tunnel := range tunnels {
		if err := tm.StartTunnel(tunnel.ID); err != nil {
			fmt.Printf("Failed to auto-start tunnel %s: %v\n", tunnel.Name, err)
		}
	}
}

// StopAllTunnels stops all running tunnels
func (tm *TunnelManager) StopAllTunnels(ctx context.Context) error {
	// Use process manager's cleanup for efficient bulk termination
	if err := tm.processManager.Cleanup(ctx); err != nil {
		// Log error but continue with tunnel state cleanup
		if tm.debug {
			fmt.Printf("Warning: process cleanup error: %v\n", err)
		}
	}

	// Update all tunnel states
	tm.mu.Lock()
	for id, tunnel := range tm.tunnels {
		if tunnel.Status == StatusRunning {
			oldStatus := tunnel.Status
			tunnel.Status = StatusStopped
			tunnel.process = nil
			tunnel.PID = 0
			tunnel.StartedAt = nil

			// Remove from PID store
			tm.pidStore.RemovePid(id)

			// Notify status change
			tm.notifyStatusChange(id, oldStatus, StatusStopped, nil)
		}
	}
	tm.mu.Unlock()

	return nil
}

// GetTunnelsByProfile returns tunnels belonging to a specific profile sorted by name
func (tm *TunnelManager) GetTunnelsByProfile(profileName string) []*Tunnel {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tunnels []*Tunnel
	for _, tunnel := range tm.tunnels {
		if tunnel.Profile == profileName || (profileName == "default" && tunnel.Profile == "") {
			tunnels = append(tunnels, tunnel.Clone())
		}
	}

	// Sort tunnels by name for consistent ordering
	sort.Slice(tunnels, func(i, j int) bool {
		return tunnels[i].Name < tunnels[j].Name
	})

	return tunnels
}

// StartProfileTunnels starts all tunnels in a profile
func (tm *TunnelManager) StartProfileTunnels(profileName string) error {
	tunnels := tm.GetTunnelsByProfile(profileName)
	var failedTunnels []string

	for i, tunnel := range tunnels {
		if tunnel.Status != StatusRunning {
			if err := tm.StartTunnel(tunnel.ID); err != nil {
				failedTunnels = append(failedTunnels, tunnel.Name)
				Error("Failed to start tunnel %s: %v", tunnel.Name, err)
			} else {
				// Add a small delay between tunnel starts to avoid SSH connection issues
				// But not after the last tunnel
				if i < len(tunnels)-1 {
					time.Sleep(200 * time.Millisecond)
				}
			}
		}
	}

	if len(failedTunnels) > 0 {
		return fmt.Errorf("failed to start %d tunnel(s): %v", len(failedTunnels), failedTunnels)
	}
	return nil
}

// StopProfileTunnels stops all tunnels in a profile
func (tm *TunnelManager) StopProfileTunnels(profileName string) error {
	tunnels := tm.GetTunnelsByProfile(profileName)
	var lastErr error

	for _, tunnel := range tunnels {
		if tunnel.Status == StatusRunning {
			if err := tm.StopTunnel(tunnel.ID); err != nil {
				lastErr = err
				Error("Failed to stop tunnel %s: %v", tunnel.Name, err)
			}
		}
	}

	return lastErr
}

// AutoConnectProfile auto-connects all tunnels marked for auto-connect in a profile
func (tm *TunnelManager) AutoConnectProfile(profileName string) {
	tunnels := tm.GetTunnelsByProfile(profileName)

	for _, tunnel := range tunnels {
		if tunnel.AutoConnect && tunnel.Status == StatusStopped {
			if err := tm.StartTunnel(tunnel.ID); err != nil {
				Error("Failed to auto-start tunnel %s: %v", tunnel.Name, err)
			} else {
				Info("Auto-started tunnel: %s", tunnel.Name)
			}
		}
	}
}

// GetStatusChanges returns the channel for status change notifications
func (tm *TunnelManager) GetStatusChanges() <-chan TunnelStatusChange {
	return tm.statusChanges
}

// monitorTunnel monitors a running tunnel process
func (tm *TunnelManager) monitorTunnel(id string) {
	// Wait for process to be removed from process manager
	for {
		time.Sleep(1 * time.Second)

		// Check if process still exists in process manager
		if _, exists := tm.processManager.GetProcessInfo(id); !exists {
			break
		}
	}

	tm.mu.Lock()
	tunnel, exists := tm.tunnels[id]
	if !exists {
		tm.mu.Unlock()
		return
	}

	oldStatus := tunnel.Status

	// Only update status if it's still running
	if tunnel.Status == StatusRunning {
		tunnel.Status = StatusStopped
		tunnel.process = nil
		tunnel.PID = 0
		tunnel.StartedAt = nil
	}

	newStatus := tunnel.Status
	lastError := tunnel.LastError
	tm.mu.Unlock()

	// Remove PID from store
	tm.pidStore.RemovePid(id)

	// Notify status change
	if oldStatus != newStatus {
		tm.notifyStatusChange(id, oldStatus, newStatus, lastError)
	}
}

// notifyStatusChange sends a status change notification
func (tm *TunnelManager) notifyStatusChange(tunnelID string, oldStatus, newStatus TunnelStatus, err error) {
	select {
	case tm.statusChanges <- TunnelStatusChange{
		TunnelID:  tunnelID,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		Error:     err,
	}:
	default:
		// Channel full, skip notification
	}
}

// loadTunnels loads tunnel configurations from the config store
func (tm *TunnelManager) loadTunnels() {
	config, err := tm.configStore.LoadConfig()
	if err != nil {
		// If config doesn't exist, start with empty tunnels
		return
	}

	// Convert TunnelConfig to Tunnel
	for _, tc := range config.Tunnels {
		// Map mode values for backward compatibility
		mode := tc.Mode
		if mode == "forward" {
			mode = "local"
		} else if mode == "reverse" {
			mode = "remote"
		}

		tunnel := &Tunnel{
			ID:          tc.ID,
			Name:        tc.Name,
			SSHHost:     tc.Host,
			LocalPort:   tc.LocalPort,
			RemotePort:  tc.RemotePort,
			Type:        TunnelType(mode),
			ExtraArgs:   tc.Options,
			Profile:     tc.Profile,
			AutoConnect: tc.AutoConnect,
			Status:      StatusStopped,
			LocalHost:   "0.0.0.0",
		}

		// Set default profile if not specified
		if tunnel.Profile == "" {
			tunnel.Profile = "default"
		}

		// Set default remote host for local forward
		if tunnel.Type == LocalForward && tunnel.RemoteHost == "" {
			tunnel.RemoteHost = "127.0.0.1"
		}

		tm.tunnels[tunnel.ID] = tunnel
	}
}

// saveTunnels saves tunnel configurations to the config store
func (tm *TunnelManager) saveTunnels() error {

	config := &store.AppConfig{
		Version: "1.0",
	}

	// Convert tunnels to TunnelConfig
	var tunnelConfigs []store.TunnelConfig
	for _, t := range tm.tunnels {
		tunnelConfigs = append(tunnelConfigs, store.TunnelConfig{
			ID:          t.ID,
			Name:        t.Name,
			Host:        t.SSHHost,
			LocalPort:   t.LocalPort,
			RemotePort:  t.RemotePort,
			Mode:        string(t.Type),
			Options:     t.ExtraArgs,
			Profile:     t.Profile,
			AutoConnect: t.AutoConnect,
		})
	}
	config.Tunnels = tunnelConfigs

	// Collect unique profiles from tunnels
	profileMap := make(map[string]bool)
	profileMap["default"] = true // Always include default profile
	for _, t := range tm.tunnels {
		if t.Profile != "" && t.Profile != "default" {
			profileMap[t.Profile] = true
		}
	}

	// Convert profile map to slice
	var profiles []store.Profile
	for name := range profileMap {
		profiles = append(profiles, store.Profile{
			Name:        name,
			Description: fmt.Sprintf("%s profile", name),
		})
	}
	config.Profiles = profiles

	return tm.configStore.SaveConfig(config)
}

// restoreTunnelStates attempts to restore running tunnel states from PID store
func (tm *TunnelManager) restoreTunnelStates() {
	pids, err := tm.pidStore.LoadPids()
	if err != nil {
		return
	}

	for tunnelID, pidInfo := range pids.Pids {
		tunnel, exists := tm.tunnels[tunnelID]
		if !exists {
			// Remove orphaned PID
			tm.pidStore.RemovePid(tunnelID)
			continue
		}

		// Check if process is still running
		process, err := os.FindProcess(pidInfo.PID)
		if err != nil {
			tm.pidStore.RemovePid(tunnelID)
			continue
		}

		// Send signal 0 to check if process exists
		if err := process.Signal(syscall.Signal(0)); err != nil {
			// Process doesn't exist
			tm.pidStore.RemovePid(tunnelID)
		} else {
			// Process is still running
			tunnel.Status = StatusRunning
			tunnel.PID = pidInfo.PID
			// Parse and set the started time
			if startTime, err := time.Parse(time.RFC3339, pidInfo.Started); err == nil {
				tunnel.StartedAt = &startTime
			} else {
				now := time.Now()
				tunnel.StartedAt = &now
			}
		}
	}
}

// ImportFromSSHConfig imports tunnel configurations from SSH config for a specific host
func (tm *TunnelManager) ImportFromSSHConfig(hostAlias string) ([]*Tunnel, error) {
	parser := NewSSHConfigParser()
	hostConfig, err := parser.ParseHost(hostAlias)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH config: %w", err)
	}

	if hostConfig == nil {
		return nil, fmt.Errorf("host %s not found in SSH config", hostAlias)
	}

	// Convert SSH config to tunnels
	tunnels := hostConfig.ConvertToTunnels()
	if len(tunnels) == 0 {
		return nil, fmt.Errorf("no tunnel configurations found for host %s", hostAlias)
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	var imported []*Tunnel
	for _, tunnel := range tunnels {
		// Check if tunnel with same ID already exists
		if _, exists := tm.tunnels[tunnel.ID]; !exists {
			tm.tunnels[tunnel.ID] = tunnel
			imported = append(imported, tunnel)
		}
	}

	// Save updated configuration
	if len(imported) > 0 {
		if err := tm.saveTunnels(); err != nil {
			// Rollback on save failure
			for _, tunnel := range imported {
				delete(tm.tunnels, tunnel.ID)
			}
			return nil, fmt.Errorf("failed to save configuration: %w", err)
		}
	}

	return imported, nil
}

// LoadSSHConfigHosts loads all available SSH hosts from SSH config
func (tm *TunnelManager) LoadSSHConfigHosts() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".ssh", "config")
	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to open SSH config: %w", err)
	}
	defer file.Close()

	var hosts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "host ") {
			hostLine := strings.TrimSpace(line[5:])
			for _, h := range strings.Fields(hostLine) {
				// Skip wildcards and patterns
				if !strings.Contains(h, "*") && !strings.Contains(h, "?") {
					hosts = append(hosts, h)
				}
			}
		}
	}

	return hosts, scanner.Err()
}