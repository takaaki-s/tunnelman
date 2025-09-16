// Package core provides core data types and validation for SSH tunnel management.
package core

import (
	"runtime"
	"time"
)


// PidEntry represents a running tunnel process
type PidEntry struct {
	// Process ID
	PID int `json:"pid"`

	// Timestamp when the tunnel was started (ISO 8601 UTC)
	Started string `json:"started"`

	// Associated tunnel ID
	TunnelID string `json:"tunnelId,omitempty"`
}


// NewPidEntry creates a new PID entry with the current UTC timestamp
func NewPidEntry(pid int, tunnelID string) *PidEntry {
	return &PidEntry{
		PID:      pid,
		Started:  time.Now().UTC().Format(time.RFC3339),
		TunnelID: tunnelID,
	}
}

// GetStartedTime parses the Started timestamp
func (p *PidEntry) GetStartedTime() (time.Time, error) {
	return time.Parse(time.RFC3339, p.Started)
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsMacOS returns true if running on macOS
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}