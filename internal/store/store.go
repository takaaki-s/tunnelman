// Package store provides interfaces for persistent storage
package store

import (
	"time"
)

// TunnelConfig represents a tunnel configuration for storage
type TunnelConfig struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Host        string   `json:"host"`
	LocalPort   int      `json:"localPort"`
	RemotePort  int      `json:"remotePort"`
	Mode        string   `json:"mode"`
	Profile     string   `json:"profile,omitempty"`
	Options     []string `json:"options,omitempty"`
	AutoConnect bool     `json:"auto_connect,omitempty"`
}

// PidInfo represents process information for storage
type PidInfo struct {
	PID      int    `json:"pid"`
	Started  string `json:"started"`
	TunnelID string `json:"tunnelId,omitempty"`
}


// AppConfig represents the application configuration
type AppConfig struct {
	Version  string         `json:"version"`
	Tunnels  []TunnelConfig `json:"tunnels"`
	Profiles []Profile      `json:"profiles,omitempty"`
}

// Profile represents a named collection of tunnels
type Profile struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	TunnelIDs   []string `json:"tunnelIds"`
	AutoConnect bool     `json:"autoConnect,omitempty"`
}

// PidData represents the PID storage data
type PidData struct {
	Pids map[string]PidInfo `json:"pids"`
}

// NewPidInfo creates a new PID info entry
func NewPidInfo(pid int, tunnelID string) *PidInfo {
	return &PidInfo{
		PID:      pid,
		Started:  time.Now().UTC().Format(time.RFC3339),
		TunnelID: tunnelID,
	}
}