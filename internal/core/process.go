// Package core provides process management for SSH tunnels.
package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ProcessManager handles SSH process lifecycle operations
type ProcessManager struct {
	// Debug mode flag for verbose logging
	debug bool

	// Logger for debug output
	logger *log.Logger

	// Process tracking
	mu        sync.RWMutex
	processes map[string]*ProcessInfo
}

// ProcessInfo contains information about a running SSH process
type ProcessInfo struct {
	// Command that was executed
	Cmd *exec.Cmd

	// Process ID
	PID int

	// Tunnel configuration
	Tunnel *Tunnel

	// Start time
	StartedAt time.Time

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Output handlers for debug mode
	stdoutReader io.ReadCloser
	stderrReader io.ReadCloser
}

// ProcessManagerOption is a functional option for ProcessManager
type ProcessManagerOption func(*ProcessManager)

// WithDebug enables debug mode for the process manager
func WithDebug(debug bool) ProcessManagerOption {
	return func(pm *ProcessManager) {
		pm.debug = debug
	}
}

// WithLogger sets a custom logger for the process manager
func WithLogger(logger *log.Logger) ProcessManagerOption {
	return func(pm *ProcessManager) {
		pm.logger = logger
	}
}

// NewProcessManager creates a new process manager instance
func NewProcessManager(opts ...ProcessManagerOption) *ProcessManager {
	pm := &ProcessManager{
		processes: make(map[string]*ProcessInfo),
		logger:    log.New(os.Stderr, "[ProcessManager] ", log.LstdFlags),
	}

	// Apply options
	for _, opt := range opts {
		opt(pm)
	}

	return pm
}

// Connect establishes an SSH tunnel connection
func (pm *ProcessManager) Connect(tunnel *Tunnel) (*PidEntry, error) {
	if tunnel == nil {
		return nil, fmt.Errorf("tunnel cannot be nil")
	}

	// Validate tunnel configuration
	if err := tunnel.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tunnel configuration: %w", err)
	}

	// Build SSH command arguments
	args := pm.buildSSHArgs(tunnel)

	if pm.debug {
		LogSSHCommand(tunnel.Name, append([]string{"ssh"}, args...))
	}

	// Create command
	cmd := exec.Command("ssh", args...)

	// Set process group for clean termination
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Setup output handling for debug mode
	if pm.debug {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		// Start output monitoring goroutines
		go pm.monitorOutput("stdout", tunnel.ID, stdout)
		go pm.monitorOutput("stderr", tunnel.ID, stderr)
	}

	// Start the SSH process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start SSH process: %w", err)
	}

	// Create process context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	// Store process information
	processInfo := &ProcessInfo{
		Cmd:       cmd,
		PID:       cmd.Process.Pid,
		Tunnel:    tunnel,
		StartedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	pm.mu.Lock()
	pm.processes[tunnel.ID] = processInfo
	pm.mu.Unlock()

	if pm.debug {
		pm.logger.Printf("SSH process started for tunnel %s (PID: %d)", tunnel.ID, cmd.Process.Pid)
	}

	// Create PID entry for storage
	pidEntry := NewPidEntry(cmd.Process.Pid, tunnel.ID)

	// Monitor process lifecycle in background
	go pm.monitorProcess(tunnel.ID, processInfo)

	return pidEntry, nil
}

// Disconnect terminates an SSH tunnel connection
func (pm *ProcessManager) Disconnect(id string, pid int) error {
	pm.mu.Lock()
	processInfo, exists := pm.processes[id]
	if !exists {
		pm.mu.Unlock()
		// Try to kill by PID if process info not found
		return pm.killProcessByPID(pid)
	}
	pm.mu.Unlock()

	if pm.debug {
		pm.logger.Printf("Disconnecting tunnel %s (PID: %d)", id, processInfo.PID)
	}

	// Cancel context first
	if processInfo.cancel != nil {
		processInfo.cancel()
	}

	// Graceful termination with SIGTERM
	if err := pm.terminateProcess(processInfo.Cmd.Process); err != nil {
		if pm.debug {
			pm.logger.Printf("SIGTERM failed for PID %d: %v, attempting SIGKILL", processInfo.PID, err)
		}

		// Force kill if SIGTERM fails
		if err := pm.killProcess(processInfo.Cmd.Process); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", processInfo.PID, err)
		}
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- processInfo.Cmd.Wait()
	}()

	select {
	case <-done:
		if pm.debug {
			pm.logger.Printf("Process %d terminated successfully", processInfo.PID)
		}
	case <-time.After(5 * time.Second):
		// Force kill if still running
		processInfo.Cmd.Process.Kill()
		if pm.debug {
			pm.logger.Printf("Process %d force killed after timeout", processInfo.PID)
		}
	}

	// Clean up process info
	pm.mu.Lock()
	delete(pm.processes, id)
	pm.mu.Unlock()

	return nil
}

// GetProcessInfo returns information about a running process
func (pm *ProcessManager) GetProcessInfo(id string) (*ProcessInfo, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	info, exists := pm.processes[id]
	return info, exists
}

// GetAllProcesses returns all running processes
func (pm *ProcessManager) GetAllProcesses() map[string]*ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Create a copy to avoid race conditions
	processes := make(map[string]*ProcessInfo)
	for k, v := range pm.processes {
		processes[k] = v
	}
	return processes
}

// buildSSHArgs constructs SSH command arguments based on tunnel configuration
func (pm *ProcessManager) buildSSHArgs(tunnel *Tunnel) []string {
	var args []string

	// Add tunnel type specific options
	switch tunnel.Type {
	case LocalForward:
		// -L [bind_address:]port:host:hostport
		forward := fmt.Sprintf("%s:%d:%s:%d",
			tunnel.LocalHost, tunnel.LocalPort,
			tunnel.RemoteHost, tunnel.RemotePort)
		args = append(args, "-L", forward)

	case RemoteForward:
		// -R [bind_address:]port:host:hostport
		// RemotePort on remote side forwards to LocalHost:LocalPort
		// Omitting bind address to use server's default (usually 127.0.0.1)
		// For external access, server must have GatewayPorts enabled
		localHost := tunnel.LocalHost
		if localHost == "" || localHost == "0.0.0.0" {
			// For RemoteForward, we need a valid destination address
			localHost = "127.0.0.1"
		}
		forward := fmt.Sprintf("%d:%s:%d",
			tunnel.RemotePort, localHost, tunnel.LocalPort)
		args = append(args, "-R", forward)

	case DynamicForward:
		// -D [bind_address:]port
		args = append(args, "-D", fmt.Sprintf("%s:%d", tunnel.LocalHost, tunnel.LocalPort))
	}

	// Common SSH options for tunnel stability
	args = append(args,
		"-N",                             // No command execution (port forwarding only)
		"-T",                             // Disable pseudo-terminal allocation
		"-o", "ServerAliveInterval=60",  // Keep connection alive
		"-o", "ServerAliveCountMax=3",   // Max keepalive attempts
		"-o", "ExitOnForwardFailure=yes", // Exit if port forwarding fails
		"-o", "StrictHostKeyChecking=accept-new", // Auto-accept new host keys
		"-o", "ControlMaster=no",         // Don't use connection sharing
		"-o", "ControlPath=none",         // No control socket
	)

	// Add any extra arguments
	if len(tunnel.ExtraArgs) > 0 {
		args = append(args, tunnel.ExtraArgs...)
	}

	// Add verbose flag in debug mode
	if pm.debug {
		args = append(args, "-v")
	}

	// Add destination (SSH will use system default user or SSH config)
	args = append(args, tunnel.SSHHost)

	return args
}

// terminateProcess sends SIGTERM to a process and its group
func (pm *ProcessManager) terminateProcess(process *os.Process) error {
	// Send SIGTERM to the process group
	return syscall.Kill(-process.Pid, syscall.SIGTERM)
}

// killProcess sends SIGKILL to a process and its group
func (pm *ProcessManager) killProcess(process *os.Process) error {
	// Send SIGKILL to the process group
	return syscall.Kill(-process.Pid, syscall.SIGKILL)
}

// killProcessByPID attempts to kill a process by PID only
func (pm *ProcessManager) killProcessByPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}

	// Find process by PID
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// Try SIGTERM first
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Try SIGKILL if SIGTERM fails
		if err := process.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", pid, err)
		}
	}

	return nil
}

// monitorProcess monitors a running SSH process
func (pm *ProcessManager) monitorProcess(tunnelID string, info *ProcessInfo) {
	// Wait for process to exit
	err := info.Cmd.Wait()

	if pm.debug {
		if err != nil {
			pm.logger.Printf("Process for tunnel %s exited with error: %v", tunnelID, err)
		} else {
			pm.logger.Printf("Process for tunnel %s exited normally", tunnelID)
		}
	}

	// Clean up process info
	pm.mu.Lock()
	delete(pm.processes, tunnelID)
	pm.mu.Unlock()
}

// monitorOutput monitors and logs process output in debug mode
func (pm *ProcessManager) monitorOutput(streamName string, tunnelID string, reader io.ReadCloser) {
	defer reader.Close()

	var output strings.Builder
	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			output.Write(buffer[:n])
			// Log the output using the logger
			if streamName == "stdout" {
				LogSSHOutput(tunnelID, string(buffer[:n]), "")
			} else {
				LogSSHOutput(tunnelID, "", string(buffer[:n]))
			}
		}
		if err != nil {
			if err != io.EOF && pm.debug {
				Error("[%s][%s] Read error: %v", tunnelID, streamName, err)
			}
			break
		}
	}
}

// IsProcessRunning checks if a process is still running
func (pm *ProcessManager) IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Cleanup performs cleanup of all managed processes
func (pm *ProcessManager) Cleanup(ctx context.Context) error {
	pm.mu.Lock()
	tunnelIDs := make([]string, 0, len(pm.processes))
	for id := range pm.processes {
		tunnelIDs = append(tunnelIDs, id)
	}
	pm.mu.Unlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(tunnelIDs))

	for _, id := range tunnelIDs {
		wg.Add(1)
		go func(tunnelID string) {
			defer wg.Done()

			pm.mu.RLock()
			info, exists := pm.processes[tunnelID]
			pm.mu.RUnlock()

			if exists {
				if err := pm.Disconnect(tunnelID, info.PID); err != nil {
					errChan <- fmt.Errorf("failed to disconnect tunnel %s: %w", tunnelID, err)
				}
			}
		}(id)
	}

	// Wait for all disconnections or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All processes terminated
	case <-ctx.Done():
		return ctx.Err()
	}

	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup completed with %d errors", len(errors))
	}

	return nil
}