// Package core provides the core business logic for SSH tunnel management.
package core

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TunnelType represents the type of SSH tunnel
type TunnelType string

const (
	// LocalForward represents a local port forwarding tunnel (-L)
	LocalForward TunnelType = "local"
	// RemoteForward represents a remote port forwarding tunnel (-R)
	RemoteForward TunnelType = "remote"
	// DynamicForward represents a dynamic port forwarding tunnel (-D)
	DynamicForward TunnelType = "dynamic"
)

// TunnelStatus represents the current state of a tunnel
type TunnelStatus string

const (
	// StatusStopped indicates the tunnel is not running
	StatusStopped TunnelStatus = "stopped"
	// StatusRunning indicates the tunnel is active
	StatusRunning TunnelStatus = "running"
	// StatusError indicates the tunnel encountered an error
	StatusError TunnelStatus = "error"
	// StatusConnecting indicates the tunnel is being established
	StatusConnecting TunnelStatus = "connecting"
)

// Tunnel represents an SSH tunnel configuration and state
type Tunnel struct {
	// Configuration fields
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        TunnelType `json:"type"`
	LocalHost   string     `json:"local_host,omitempty"`
	LocalPort   int        `json:"local_port"`
	RemoteHost  string     `json:"remote_host,omitempty"`
	RemotePort  int        `json:"remote_port,omitempty"`
	SSHHost     string     `json:"ssh_host"`
	ExtraArgs   []string   `json:"extra_args,omitempty"`
	AutoConnect bool       `json:"auto_connect"`
	Profile     string     `json:"profile,omitempty"`

	// Runtime state fields (not persisted)
	Status    TunnelStatus `json:"-"`
	PID       int          `json:"-"`
	StartedAt *time.Time   `json:"-"`
	LastError error        `json:"-"`

	// Internal fields
	mu      sync.RWMutex
	process *exec.Cmd
}

// NewTunnel creates a new tunnel configuration with sensible defaults
func NewTunnel(name string, tunnelType TunnelType) *Tunnel {
	localHost := "0.0.0.0" // Default for LocalForward (bind address)
	if tunnelType == RemoteForward {
		localHost = "127.0.0.1" // For RemoteForward, this is the destination
	}
	return &Tunnel{
		ID:        generateID(),
		Name:      name,
		Type:      tunnelType,
		LocalHost: localHost,
		Status:    StatusStopped,
	}
}

// Validate checks if the tunnel configuration is valid
func (t *Tunnel) Validate() error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}

	if t.SSHHost == "" {
		return fmt.Errorf("SSH host is required")
	}

	switch t.Type {
	case LocalForward:
		if t.LocalPort <= 0 || t.LocalPort > 65535 {
			return fmt.Errorf("invalid local port: %d", t.LocalPort)
		}
		if t.RemotePort <= 0 || t.RemotePort > 65535 {
			return fmt.Errorf("invalid remote port: %d", t.RemotePort)
		}
		if t.RemoteHost == "" {
			t.RemoteHost = "127.0.0.1"
		}

	case RemoteForward:
		if t.LocalPort <= 0 || t.LocalPort > 65535 {
			return fmt.Errorf("invalid local port: %d", t.LocalPort)
		}
		if t.RemotePort <= 0 || t.RemotePort > 65535 {
			return fmt.Errorf("invalid remote port: %d", t.RemotePort)
		}

	case DynamicForward:
		if t.LocalPort <= 0 || t.LocalPort > 65535 {
			return fmt.Errorf("invalid local port: %d", t.LocalPort)
		}

	default:
		return fmt.Errorf("invalid tunnel type: %s", t.Type)
	}

	return nil
}

// BuildSSHCommand constructs the SSH command for this tunnel
func (t *Tunnel) BuildSSHCommand() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	args := []string{"ssh"}

	// Add tunnel-specific flags
	switch t.Type {
	case LocalForward:
		forward := fmt.Sprintf("%s:%d:%s:%d",
			t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort)
		args = append(args, "-L", forward)

	case RemoteForward:
		// -R [bind_address:]port:host:hostport
		// RemotePort on remote side forwards to LocalHost:LocalPort
		// Omitting bind address to use server's default (usually 127.0.0.1)
		// For external access, server must have GatewayPorts enabled
		localHost := t.LocalHost
		if localHost == "" || localHost == "0.0.0.0" {
			// For RemoteForward, we need a valid destination address
			localHost = "127.0.0.1"
		}
		forward := fmt.Sprintf("%d:%s:%d",
			t.RemotePort, localHost, t.LocalPort)
		args = append(args, "-R", forward)

	case DynamicForward:
		args = append(args, "-D", fmt.Sprintf("%s:%d", t.LocalHost, t.LocalPort))
	}

	// Common SSH options for tunnel stability
	args = append(args,
		"-N",                    // No command execution
		"-T",                    // Disable pseudo-terminal allocation
		"-o", "ServerAliveInterval=60",  // Keep connection alive
		"-o", "ServerAliveCountMax=3",   // Max keepalive attempts
		"-o", "ExitOnForwardFailure=yes", // Exit if port forwarding fails
	)

	// Add any extra arguments
	args = append(args, t.ExtraArgs...)

	// Add destination (SSH will use system default user or SSH config)
	args = append(args, t.SSHHost)

	return args
}

// GetDisplayName returns a formatted display name for the tunnel
func (t *Tunnel) GetDisplayName() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var portInfo string
	switch t.Type {
	case LocalForward:
		portInfo = fmt.Sprintf("L:%d→%s:%d", t.LocalPort, t.RemoteHost, t.RemotePort)
	case RemoteForward:
		portInfo = fmt.Sprintf("R:%d→%d", t.RemotePort, t.LocalPort)
	case DynamicForward:
		portInfo = fmt.Sprintf("D:%d", t.LocalPort)
	}

	return fmt.Sprintf("%s (%s)", t.Name, portInfo)
}

// Clone creates a deep copy of the tunnel configuration
func (t *Tunnel) Clone() *Tunnel {
	t.mu.RLock()
	defer t.mu.RUnlock()

	clone := &Tunnel{
		ID:          t.ID,
		Name:        t.Name,
		Type:        t.Type,
		LocalHost:   t.LocalHost,
		LocalPort:   t.LocalPort,
		RemoteHost:  t.RemoteHost,
		RemotePort:  t.RemotePort,
		SSHHost:     t.SSHHost,
		AutoConnect: t.AutoConnect,
		Status:      t.Status,
		PID:         t.PID,
		LastError:   t.LastError,
	}

	if len(t.ExtraArgs) > 0 {
		clone.ExtraArgs = make([]string, len(t.ExtraArgs))
		copy(clone.ExtraArgs, t.ExtraArgs)
	}

	if t.StartedAt != nil {
		startedAt := *t.StartedAt
		clone.StartedAt = &startedAt
	}

	return clone
}

// generateID creates a unique identifier for a tunnel
func generateID() string {
	return fmt.Sprintf("tunnel_%d", time.Now().UnixNano())
}

// ParseForwardingSpec parses a forwarding specification string
// Format examples:
//   - "8080:localhost:80" for local forward
//   - "8080:80" for remote forward
//   - "1080" for dynamic forward
func ParseForwardingSpec(spec string, tunnelType TunnelType) (localHost string, localPort int, remoteHost string, remotePort int, err error) {
	parts := strings.Split(spec, ":")

	switch tunnelType {
	case LocalForward:
		if len(parts) != 3 {
			err = fmt.Errorf("local forward requires format: localPort:remoteHost:remotePort")
			return
		}
		localHost = "0.0.0.0"
		localPort, err = strconv.Atoi(parts[0])
		if err != nil {
			err = fmt.Errorf("invalid local port: %v", err)
			return
		}
		remoteHost = parts[1]
		remotePort, err = strconv.Atoi(parts[2])
		if err != nil {
			err = fmt.Errorf("invalid remote port: %v", err)
			return
		}

	case RemoteForward:
		if len(parts) != 2 {
			err = fmt.Errorf("remote forward requires format: remotePort:localPort")
			return
		}
		localHost = "0.0.0.0"
		remotePort, err = strconv.Atoi(parts[0])
		if err != nil {
			err = fmt.Errorf("invalid remote port: %v", err)
			return
		}
		localPort, err = strconv.Atoi(parts[1])
		if err != nil {
			err = fmt.Errorf("invalid local port: %v", err)
			return
		}

	case DynamicForward:
		if len(parts) != 1 {
			err = fmt.Errorf("dynamic forward requires format: localPort")
			return
		}
		localHost = "0.0.0.0"
		localPort, err = strconv.Atoi(parts[0])
		if err != nil {
			err = fmt.Errorf("invalid local port: %v", err)
			return
		}

	default:
		err = fmt.Errorf("unsupported tunnel type: %s", tunnelType)
	}

	return
}