// Package core provides SSH config parsing functionality
package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SSHConfigHost represents a host configuration from SSH config
type SSHConfigHost struct {
	Name           string
	HostName       string
	User           string
	Port           int
	LocalForwards  []ForwardSpec
	RemoteForwards []ForwardSpec
	DynamicForwards []DynamicSpec
}

// ForwardSpec represents a port forwarding specification
type ForwardSpec struct {
	BindAddress string
	BindPort    int
	Host        string
	HostPort    int
}

// DynamicSpec represents a dynamic forwarding specification
type DynamicSpec struct {
	BindAddress string
	BindPort    int
}

// SSHConfigParser parses SSH config files
type SSHConfigParser struct {
	configPath string
}

// NewSSHConfigParser creates a new SSH config parser
func NewSSHConfigParser() *SSHConfigParser {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".ssh", "config")
	return &SSHConfigParser{
		configPath: configPath,
	}
}

// ParseHost parses SSH config for a specific host
func (p *SSHConfigParser) ParseHost(hostAlias string) (*SSHConfigHost, error) {
	file, err := os.Open(p.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No SSH config file, return nil
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open SSH config: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentHost *SSHConfigHost
	inTargetHost := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for Host directive
		if strings.HasPrefix(strings.ToLower(line), "host ") {
			hostLine := strings.TrimSpace(line[5:])
			hosts := strings.Fields(hostLine)

			// Check if this is our target host
			inTargetHost = false
			for _, h := range hosts {
				if h == hostAlias || matchesPattern(hostAlias, h) {
					currentHost = &SSHConfigHost{
						Name: hostAlias,
					}
					inTargetHost = true
					break
				}
			}
			continue
		}

		// Skip if not in target host
		if !inTargetHost || currentHost == nil {
			continue
		}

		// Parse host configuration
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(parts[0])
		value := strings.Join(parts[1:], " ")

		switch key {
		case "hostname":
			currentHost.HostName = value
		case "user":
			currentHost.User = value
		case "port":
			if port, err := strconv.Atoi(value); err == nil {
				currentHost.Port = port
			}
		case "localforward":
			if forward := parseLocalForward(value); forward != nil {
				currentHost.LocalForwards = append(currentHost.LocalForwards, *forward)
			}
		case "remoteforward":
			if forward := parseRemoteForward(value); forward != nil {
				currentHost.RemoteForwards = append(currentHost.RemoteForwards, *forward)
			}
		case "dynamicforward":
			if dynamic := parseDynamicForward(value); dynamic != nil {
				currentHost.DynamicForwards = append(currentHost.DynamicForwards, *dynamic)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SSH config: %w", err)
	}

	return currentHost, nil
}

// parseLocalForward parses a LocalForward specification
// Format: [bind_address:]port host:hostport
func parseLocalForward(spec string) *ForwardSpec {
	parts := strings.Fields(spec)
	if len(parts) != 2 {
		return nil
	}

	// Parse bind address and port
	bindParts := strings.Split(parts[0], ":")
	var bindAddress string
	var bindPort int

	if len(bindParts) == 2 {
		bindAddress = bindParts[0]
		bindPort, _ = strconv.Atoi(bindParts[1])
	} else {
		bindAddress = "0.0.0.0"
		bindPort, _ = strconv.Atoi(bindParts[0])
	}

	// Parse destination
	destParts := strings.Split(parts[1], ":")
	if len(destParts) != 2 {
		return nil
	}

	hostPort, _ := strconv.Atoi(destParts[1])

	return &ForwardSpec{
		BindAddress: bindAddress,
		BindPort:    bindPort,
		Host:        destParts[0],
		HostPort:    hostPort,
	}
}

// parseRemoteForward parses a RemoteForward specification
// Format: [bind_address:]port host:hostport
func parseRemoteForward(spec string) *ForwardSpec {
	// Same format as LocalForward
	return parseLocalForward(spec)
}

// parseDynamicForward parses a DynamicForward specification
// Format: [bind_address:]port
func parseDynamicForward(spec string) *DynamicSpec {
	parts := strings.Split(spec, ":")
	var bindAddress string
	var bindPort int

	if len(parts) == 2 {
		bindAddress = parts[0]
		bindPort, _ = strconv.Atoi(parts[1])
	} else {
		bindAddress = "0.0.0.0"
		bindPort, _ = strconv.Atoi(parts[0])
	}

	return &DynamicSpec{
		BindAddress: bindAddress,
		BindPort:    bindPort,
	}
}

// matchesPattern checks if a host matches a pattern (simple wildcard support)
func matchesPattern(host, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Simple wildcard matching (e.g., *.example.com)
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(host, suffix)
	}

	return host == pattern
}

// ConvertToTunnels converts SSH config host to Tunnelman tunnels
func (h *SSHConfigHost) ConvertToTunnels() []*Tunnel {
	var tunnels []*Tunnel

	// Convert LocalForwards
	for i, fwd := range h.LocalForwards {
		tunnel := &Tunnel{
			ID:         fmt.Sprintf("%s-local-%d", h.Name, i+1),
			Name:       fmt.Sprintf("%s Local %d→%d", h.Name, fwd.BindPort, fwd.HostPort),
			Type:       LocalForward,
			SSHHost:    h.Name,
			LocalHost:  fwd.BindAddress,
			LocalPort:  fwd.BindPort,
			RemoteHost: fwd.Host,
			RemotePort: fwd.HostPort,
		}
		tunnels = append(tunnels, tunnel)
	}

	// Convert RemoteForwards
	for i, fwd := range h.RemoteForwards {
		tunnel := &Tunnel{
			ID:         fmt.Sprintf("%s-remote-%d", h.Name, i+1),
			Name:       fmt.Sprintf("%s Remote %d←%d", h.Name, fwd.BindPort, fwd.HostPort),
			Type:       RemoteForward,
			SSHHost:    h.Name,
			LocalHost:  fwd.Host,
			LocalPort:  fwd.HostPort,
			RemotePort: fwd.BindPort,
		}
		tunnels = append(tunnels, tunnel)
	}

	// Convert DynamicForwards
	for i, dyn := range h.DynamicForwards {
		tunnel := &Tunnel{
			ID:        fmt.Sprintf("%s-dynamic-%d", h.Name, i+1),
			Name:      fmt.Sprintf("%s SOCKS %d", h.Name, dyn.BindPort),
			Type:      DynamicForward,
			SSHHost:   h.Name,
			LocalHost: dyn.BindAddress,
			LocalPort: dyn.BindPort,
			Profile:   "ssh-config",
		}
		tunnels = append(tunnels, tunnel)
	}

	return tunnels
}