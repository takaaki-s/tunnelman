package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/takaaki-s/tunnelman/internal/tunnel"
)

// Commander abstracts exec.Command for testability.
type Commander interface {
	Command(name string, args ...string) *exec.Cmd
}

type execCommander struct{}

func (execCommander) Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// ProcessInfo tracks a running SSH process.
type ProcessInfo struct {
	TunnelID  string
	PID       int
	Cmd       *exec.Cmd
	StartedAt time.Time
}

// ProcessManager manages SSH tunnel processes.
type ProcessManager struct {
	commander Commander
	processes map[string]*ProcessInfo
	mu        sync.RWMutex
	onExit    func(tunnelID string)
}

// NewProcessManager creates a ProcessManager with the given Commander.
func NewProcessManager(cmdr Commander) *ProcessManager {
	if cmdr == nil {
		cmdr = execCommander{}
	}
	return &ProcessManager{
		commander: cmdr,
		processes: make(map[string]*ProcessInfo),
	}
}

// SetOnExit sets a callback invoked when a tunnel process exits.
func (pm *ProcessManager) SetOnExit(fn func(tunnelID string)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.onExit = fn
}

// Connect starts an SSH tunnel process.
func (pm *ProcessManager) Connect(t *tunnel.Tunnel) (*ProcessInfo, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.processes[t.ID]; exists {
		return nil, fmt.Errorf("tunnel %s is already running", t.ID)
	}

	args := t.BuildSSHCommand()
	// args[0] is "ssh", rest are arguments
	cmd := pm.commander.Command(args[0], args[1:]...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start tunnel %s: %w", t.ID, err)
	}

	info := &ProcessInfo{
		TunnelID:  t.ID,
		PID:       cmd.Process.Pid,
		Cmd:       cmd,
		StartedAt: time.Now(),
	}
	pm.processes[t.ID] = info

	// Monitor process exit
	go pm.waitProcess(t.ID, cmd)

	return info, nil
}

// Disconnect stops a tunnel process.
func (pm *ProcessManager) Disconnect(tunnelID string) error {
	pm.mu.Lock()
	info, exists := pm.processes[tunnelID]
	if !exists {
		pm.mu.Unlock()
		return fmt.Errorf("tunnel %s is not running", tunnelID)
	}
	// Remove from map immediately to prevent double-disconnect
	delete(pm.processes, tunnelID)
	pm.mu.Unlock()

	if info.Cmd.Process != nil {
		// Signal the process; waitProcess goroutine will call Wait()
		_ = info.Cmd.Process.Signal(os.Interrupt)
		time.Sleep(100 * time.Millisecond)
		// Force kill if still alive
		_ = info.Cmd.Process.Kill()
	}

	return nil
}

// IsRunning checks if a tunnel process is active.
func (pm *ProcessManager) IsRunning(tunnelID string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	_, exists := pm.processes[tunnelID]
	return exists
}

// GetProcessInfo returns info for a running tunnel.
func (pm *ProcessManager) GetProcessInfo(tunnelID string) *ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.processes[tunnelID]
}

// StopAll stops all running processes.
func (pm *ProcessManager) StopAll() {
	pm.mu.RLock()
	ids := make([]string, 0, len(pm.processes))
	for id := range pm.processes {
		ids = append(ids, id)
	}
	pm.mu.RUnlock()

	for _, id := range ids {
		_ = pm.Disconnect(id)
	}
}

func (pm *ProcessManager) waitProcess(tunnelID string, cmd *exec.Cmd) {
	_ = cmd.Wait()

	pm.mu.Lock()
	delete(pm.processes, tunnelID)
	onExit := pm.onExit
	pm.mu.Unlock()

	if onExit != nil {
		onExit(tunnelID)
	}
}
