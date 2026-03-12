// Package sshconfig parses SSH config files for tunnel import.
package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SSHConfigHost represents a host entry from an SSH config file.
type SSHConfigHost struct {
	Name            string
	HostName        string
	User            string
	Port            int
	LocalForwards   []ForwardSpec
	RemoteForwards  []ForwardSpec
	DynamicForwards []DynamicSpec
}

// ForwardSpec represents a local or remote port forwarding spec.
type ForwardSpec struct {
	BindAddress string
	BindPort    int
	Host        string
	HostPort    int
}

// DynamicSpec represents a dynamic (SOCKS) forwarding spec.
type DynamicSpec struct {
	BindAddress string
	BindPort    int
}

// Parser reads and parses SSH config files.
type Parser struct {
	path string
}

// NewParser creates a new parser for the given SSH config file path.
func NewParser(path string) *Parser {
	return &Parser{path: path}
}

// ParseHost parses the SSH config for a specific host alias.
// Returns nil if the host is not found.
func (p *Parser) ParseHost(hostAlias string) (*SSHConfigHost, error) {
	file, err := os.Open(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open SSH config: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var current *SSHConfigHost
	inTarget := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(strings.ToLower(line), "host ") {
			hosts := strings.Fields(line[5:])
			inTarget = false
			for _, h := range hosts {
				if h == hostAlias {
					current = &SSHConfigHost{Name: hostAlias}
					inTarget = true
					break
				}
			}
			continue
		}

		if !inTarget || current == nil {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(parts[0])
		value := strings.Join(parts[1:], " ")

		switch key {
		case "hostname":
			current.HostName = value
		case "user":
			current.User = value
		case "port":
			if port, err := strconv.Atoi(value); err == nil {
				current.Port = port
			}
		case "localforward":
			if fwd := parseLocalForward(value); fwd != nil {
				current.LocalForwards = append(current.LocalForwards, *fwd)
			}
		case "remoteforward":
			if fwd := parseRemoteForward(value); fwd != nil {
				current.RemoteForwards = append(current.RemoteForwards, *fwd)
			}
		case "dynamicforward":
			if dyn := parseDynamicForward(value); dyn != nil {
				current.DynamicForwards = append(current.DynamicForwards, *dyn)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SSH config: %w", err)
	}
	return current, nil
}

// ListHosts returns all host aliases in the SSH config.
func (p *Parser) ListHosts() ([]string, error) {
	file, err := os.Open(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open SSH config: %w", err)
	}
	defer file.Close()

	var hosts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "host ") {
			for _, h := range strings.Fields(line[5:]) {
				hosts = append(hosts, h)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SSH config: %w", err)
	}
	return hosts, nil
}

// parseLocalForward parses "LocalForward [bind_address:]port host:hostport".
func parseLocalForward(spec string) *ForwardSpec {
	parts := strings.Fields(spec)
	if len(parts) != 2 {
		return nil
	}

	bindAddr, bindPort := parseBindSpec(parts[0])

	destParts := strings.Split(parts[1], ":")
	if len(destParts) != 2 {
		return nil
	}
	hostPort, _ := strconv.Atoi(destParts[1])

	return &ForwardSpec{
		BindAddress: bindAddr,
		BindPort:    bindPort,
		Host:        destParts[0],
		HostPort:    hostPort,
	}
}

// parseRemoteForward parses "RemoteForward [bind_address:]port host:hostport".
func parseRemoteForward(spec string) *ForwardSpec {
	return parseLocalForward(spec)
}

// parseDynamicForward parses "DynamicForward [bind_address:]port".
func parseDynamicForward(spec string) *DynamicSpec {
	addr, port := parseBindSpec(spec)
	return &DynamicSpec{BindAddress: addr, BindPort: port}
}

// parseBindSpec splits "[address:]port" into address and port.
func parseBindSpec(spec string) (string, int) {
	parts := strings.Split(spec, ":")
	if len(parts) == 2 {
		port, _ := strconv.Atoi(parts[1])
		return parts[0], port
	}
	port, _ := strconv.Atoi(parts[0])
	return "0.0.0.0", port
}
