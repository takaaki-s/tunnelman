// Package tunnel provides SSH tunnel configuration types and validation.
package tunnel

import "fmt"

// TunnelType represents the type of SSH tunnel.
type TunnelType string

const (
	LocalForward   TunnelType = "local"
	RemoteForward  TunnelType = "remote"
	DynamicForward TunnelType = "dynamic"
)

// TunnelStatus represents the runtime state of a tunnel.
type TunnelStatus string

const (
	StatusStopped   TunnelStatus = "stopped"
	StatusStarting  TunnelStatus = "starting"
	StatusRunning   TunnelStatus = "running"
	StatusUnhealthy TunnelStatus = "unhealthy"
	StatusError     TunnelStatus = "error"
)

// Tunnel represents an SSH tunnel configuration.
type Tunnel struct {
	ID          string     `yaml:"id" json:"id"`
	Name        string     `yaml:"name" json:"name"`
	Type        TunnelType `yaml:"type" json:"type"`
	SSHHost     string     `yaml:"ssh_host" json:"ssh_host"`
	LocalHost   string     `yaml:"local_host" json:"local_host"`
	LocalPort   int        `yaml:"local_port" json:"local_port"`
	RemoteHost  string     `yaml:"remote_host" json:"remote_host"`
	RemotePort  int        `yaml:"remote_port" json:"remote_port"`
	Profile     string     `yaml:"profile" json:"profile"`
	AutoConnect bool       `yaml:"auto_connect" json:"auto_connect"`
	SSHOptions  []string   `yaml:"ssh_options" json:"ssh_options,omitempty"`
}

// Validate checks if the tunnel configuration is valid.
func (t *Tunnel) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}
	if t.SSHHost == "" {
		return fmt.Errorf("SSH host is required")
	}

	switch t.Type {
	case LocalForward:
		if err := validatePort(t.LocalPort, "local"); err != nil {
			return err
		}
		if err := validatePort(t.RemotePort, "remote"); err != nil {
			return err
		}
		if t.RemoteHost == "" {
			t.RemoteHost = "127.0.0.1"
		}
	case RemoteForward:
		if err := validatePort(t.LocalPort, "local"); err != nil {
			return err
		}
		if err := validatePort(t.RemotePort, "remote"); err != nil {
			return err
		}
	case DynamicForward:
		if err := validatePort(t.LocalPort, "local"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid tunnel type: %s", t.Type)
	}

	return nil
}

func validatePort(port int, label string) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid %s port: %d", label, port)
	}
	return nil
}

// BuildSSHCommand constructs the full SSH command for this tunnel.
func (t *Tunnel) BuildSSHCommand() []string {
	args := []string{"ssh"}

	switch t.Type {
	case LocalForward:
		args = append(args, "-L",
			fmt.Sprintf("%s:%d:%s:%d", t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort))
	case RemoteForward:
		args = append(args, "-R",
			fmt.Sprintf("%d:%s:%d", t.RemotePort, t.LocalHost, t.LocalPort))
	case DynamicForward:
		args = append(args, "-D",
			fmt.Sprintf("%s:%d", t.LocalHost, t.LocalPort))
	}

	args = append(args,
		"-N",
		"-T",
		"-o", "ServerAliveInterval=60",
		"-o", "ServerAliveCountMax=3",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ControlMaster=no",
		"-o", "ControlPath=none",
	)

	args = append(args, t.SSHOptions...)
	args = append(args, t.SSHHost)

	return args
}

// Clone creates a deep copy of the tunnel.
func (t *Tunnel) Clone() *Tunnel {
	clone := *t
	if len(t.SSHOptions) > 0 {
		clone.SSHOptions = make([]string, len(t.SSHOptions))
		copy(clone.SSHOptions, t.SSHOptions)
	}
	return &clone
}
