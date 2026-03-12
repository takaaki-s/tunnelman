package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// State represents the runtime state persisted to disk.
type State struct {
	Tunnels map[string]*TunnelState `json:"tunnels"`
	Daemon  *DaemonState            `json:"daemon,omitempty"`
}

// TunnelState represents the runtime state of a single tunnel.
type TunnelState struct {
	PID            int       `json:"pid"`
	StartedAt      time.Time `json:"started_at"`
	Status         string    `json:"status"`
	ReconnectCount int       `json:"reconnect_count"`
}

// DaemonState represents the runtime state of the daemon process.
type DaemonState struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// StateManager manages the runtime state file.
type StateManager struct {
	mu       sync.RWMutex
	filePath string
	state    *State
}

// NewStateManager creates a new StateManager for the given directory.
func NewStateManager(dir string) (*StateManager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}
	return &StateManager{
		filePath: filepath.Join(dir, "state.json"),
		state:    &State{Tunnels: make(map[string]*TunnelState)},
	}, nil
}

// Load reads the state from disk.
func (sm *StateManager) Load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			sm.state = &State{Tunnels: make(map[string]*TunnelState)}
			return nil
		}
		return fmt.Errorf("failed to read state: %w", err)
	}

	if len(data) == 0 {
		sm.state = &State{Tunnels: make(map[string]*TunnelState)}
		return nil
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("failed to parse state: %w", err)
	}
	if s.Tunnels == nil {
		s.Tunnels = make(map[string]*TunnelState)
	}
	sm.state = &s
	return nil
}

// Save writes the state to disk using atomic write.
func (sm *StateManager) Save() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tmp := sm.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	if err := os.Rename(tmp, sm.filePath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}

// SetTunnel adds or updates a tunnel state.
func (sm *StateManager) SetTunnel(id string, ts *TunnelState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.Tunnels[id] = ts
}

// RemoveTunnel removes a tunnel state.
func (sm *StateManager) RemoveTunnel(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.state.Tunnels, id)
}

// GetTunnel returns the state for a specific tunnel.
func (sm *StateManager) GetTunnel(id string) *TunnelState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state.Tunnels[id]
}

// GetAllTunnels returns a copy of all tunnel states.
func (sm *StateManager) GetAllTunnels() map[string]*TunnelState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[string]*TunnelState, len(sm.state.Tunnels))
	for k, v := range sm.state.Tunnels {
		result[k] = v
	}
	return result
}

// SetDaemon sets the daemon state.
func (sm *StateManager) SetDaemon(ds *DaemonState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.Daemon = ds
}

// GetDaemon returns the daemon state.
func (sm *StateManager) GetDaemon() *DaemonState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state.Daemon
}

// CleanStalePIDs removes tunnel states whose processes are no longer running.
func (sm *StateManager) CleanStalePIDs() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cleaned := 0
	for id, ts := range sm.state.Tunnels {
		if !isProcessRunning(ts.PID) {
			delete(sm.state.Tunnels, id)
			cleaned++
		}
	}
	return cleaned
}

// isProcessRunning checks if a process with the given PID exists.
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
