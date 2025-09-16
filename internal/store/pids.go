// Package store provides PID tracking for running tunnels using XDG Base Directory Specification.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// FilePidStore implements PidStore using file system storage
type FilePidStore struct {
	mu       sync.RWMutex
	filePath string
}

// NewFilePidStore creates a new file-based PID store
func NewFilePidStore() (*FilePidStore, error) {
	pidPath, err := getPidPath()
	if err != nil {
		return nil, err
	}
	return &FilePidStore{
		filePath: pidPath,
	}, nil
}

// getPidPath returns the PID file path based on XDG Base Directory Specification
func getPidPath() (string, error) {
	var stateDir string

	switch runtime.GOOS {
	case "windows":
		// Windows: Use %LocalAppData%
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = os.Getenv("USERPROFILE")
			if localAppData == "" {
				return "", fmt.Errorf("cannot determine Windows state directory")
			}
			localAppData = filepath.Join(localAppData, "AppData", "Local")
		}
		stateDir = filepath.Join(localAppData, "tunnelman")

	default:
		// Unix-like (Linux, macOS, BSD): Use XDG_STATE_HOME
		xdgStateHome := os.Getenv("XDG_STATE_HOME")
		if xdgStateHome == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot determine home directory: %w", err)
			}
			xdgStateHome = filepath.Join(homeDir, ".local", "state")
		}
		stateDir = filepath.Join(xdgStateHome, "tunnelman")
	}

	// Ensure the state directory exists
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create state directory: %w", err)
	}

	return filepath.Join(stateDir, "pids.json"), nil
}

// LoadPids loads all stored PIDs from the XDG-compliant state file
func (fps *FilePidStore) LoadPids() (*PidData, error) {
	fps.mu.RLock()
	defer fps.mu.RUnlock()

	// Read the PID file
	data, err := os.ReadFile(fps.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty store if file doesn't exist
			return &PidData{
				Pids: make(map[string]PidInfo),
			}, nil
		}
		return nil, fmt.Errorf("failed to read PID file: %w", err)
	}

	// Parse the PID store
	var pidData PidData
	if err := json.Unmarshal(data, &pidData); err != nil {
		return nil, fmt.Errorf("failed to parse PID file: %w", err)
	}

	// Initialize map if nil
	if pidData.Pids == nil {
		pidData.Pids = make(map[string]PidInfo)
	}

	// Clean up stale PIDs (processes that no longer exist)
	cleanedData := &PidData{
		Pids: make(map[string]PidInfo),
	}
	for tunnelID, entry := range pidData.Pids {
		if isProcessRunning(entry.PID) {
			cleanedData.Pids[tunnelID] = entry
		}
	}

	// Save cleaned store if any PIDs were removed
	if len(cleanedData.Pids) != len(pidData.Pids) {
		// Save cleaned store asynchronously
		go func() {
			_ = fps.SavePids(cleanedData)
		}()
	}

	return cleanedData, nil
}

// SavePids saves all PIDs to the XDG-compliant state file
func (fps *FilePidStore) SavePids(pidData *PidData) error {
	if pidData == nil {
		return fmt.Errorf("pidData cannot be nil")
	}

	fps.mu.Lock()
	defer fps.mu.Unlock()

	// If store is empty, remove the file
	if len(pidData.Pids) == 0 {
		if err := os.Remove(fps.filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove empty PID file: %w", err)
		}
		return nil
	}

	// Marshal PID store to JSON with pretty formatting
	data, err := json.MarshalIndent(pidData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PIDs: %w", err)
	}

	// Write to temporary file first for atomic operation
	tempFile := fps.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		// Log error to stderr
		fmt.Fprintf(os.Stderr, "ERROR: Failed to write PID file: %v\n", err)
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Atomic rename to ensure data integrity
	if err := os.Rename(tempFile, fps.filePath); err != nil {
		// Clean up temporary file if rename fails
		os.Remove(tempFile)
		return fmt.Errorf("failed to save PID file: %w", err)
	}

	return nil
}

// AddPid adds a new PID entry for a tunnel
func (fps *FilePidStore) AddPid(tunnelID string, pid int) error {
	pidData, err := fps.LoadPids()
	if err != nil {
		return fmt.Errorf("failed to load PIDs: %w", err)
	}

	// Create new PID entry
	entry := NewPidInfo(pid, tunnelID)
	pidData.Pids[tunnelID] = *entry

	return fps.SavePids(pidData)
}

// RemovePid removes a PID entry for a tunnel
func (fps *FilePidStore) RemovePid(tunnelID string) error {
	pidData, err := fps.LoadPids()
	if err != nil {
		return fmt.Errorf("failed to load PIDs: %w", err)
	}

	delete(pidData.Pids, tunnelID)

	return fps.SavePids(pidData)
}

// GetPid retrieves a PID entry for a tunnel
func (fps *FilePidStore) GetPid(tunnelID string) (*PidInfo, error) {
	pidData, err := fps.LoadPids()
	if err != nil {
		return nil, fmt.Errorf("failed to load PIDs: %w", err)
	}

	entry, exists := pidData.Pids[tunnelID]
	if !exists {
		return nil, fmt.Errorf("no PID entry found for tunnel %s", tunnelID)
	}

	// Verify the process is still running
	if !isProcessRunning(entry.PID) {
		// Clean up stale entry
		_ = fps.RemovePid(tunnelID)
		return nil, fmt.Errorf("process %d is no longer running", entry.PID)
	}

	return &entry, nil
}

// CleanupStalePids removes PID entries for processes that are no longer running
func (fps *FilePidStore) CleanupStalePids() (int, error) {
	pidData, err := fps.LoadPids()
	if err != nil {
		return 0, fmt.Errorf("failed to load PIDs: %w", err)
	}

	cleaned := 0

	// Check each PID and remove if process is not running
	for tunnelID, entry := range pidData.Pids {
		if !isProcessRunning(entry.PID) {
			delete(pidData.Pids, tunnelID)
			cleaned++
		}
	}

	// Save if any PIDs were cleaned
	if cleaned > 0 {
		if err := fps.SavePids(pidData); err != nil {
			return cleaned, fmt.Errorf("cleaned %d stale PIDs but failed to save: %w", cleaned, err)
		}
	}

	return cleaned, nil
}

// GetPidPath returns the current PID file path
func (fps *FilePidStore) GetPidPath() (string, error) {
	return fps.filePath, nil
}

// isProcessRunning checks if a process with the given PID is still running
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	if runtime.GOOS == "windows" {
		// On Windows, use os.FindProcess
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		// On Windows, FindProcess always succeeds if PID is positive
		// We need to send a signal to check if process exists
		err = process.Signal(syscall.Signal(0))
		return err == nil
	} else {
		// On Unix-like systems, send signal 0 to check if process exists
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		err = process.Signal(syscall.Signal(0))
		if err != nil {
			// Process doesn't exist, log for debugging if not expected error
			if err != syscall.ESRCH && err != os.ErrProcessDone {
				fmt.Fprintf(os.Stderr, "DEBUG: Failed to signal process %d: %v\n", pid, err)
			}
			return false
		}
		return true
	}
}

// Helper functions for backward compatibility

// LoadPids loads PIDs using default path
func LoadPids() (*PidData, error) {
	store, err := NewFilePidStore()
	if err != nil {
		return nil, err
	}
	return store.LoadPids()
}

// SavePids saves PIDs using default path
func SavePids(pidData *PidData) error {
	store, err := NewFilePidStore()
	if err != nil {
		return err
	}
	return store.SavePids(pidData)
}

// AddPid adds a PID using default path
func AddPid(tunnelID string, pid int) error {
	store, err := NewFilePidStore()
	if err != nil {
		return err
	}
	return store.AddPid(tunnelID, pid)
}

// RemovePid removes a PID using default path
func RemovePid(tunnelID string) error {
	store, err := NewFilePidStore()
	if err != nil {
		return err
	}
	return store.RemovePid(tunnelID)
}

// GetPid gets a PID using default path
func GetPid(tunnelID string) (*PidInfo, error) {
	store, err := NewFilePidStore()
	if err != nil {
		return nil, err
	}
	return store.GetPid(tunnelID)
}

// CleanupStalePids cleans up stale PIDs using default path
func CleanupStalePids() (int, error) {
	store, err := NewFilePidStore()
	if err != nil {
		return 0, err
	}
	return store.CleanupStalePids()
}

// GetPidPath returns the default PID file path
func GetPidPath() (string, error) {
	return getPidPath()
}

// GetRunningTunnelCount returns the number of tunnels with running processes
func GetRunningTunnelCount() (int, error) {
	pidData, err := LoadPids()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range pidData.Pids {
		if isProcessRunning(entry.PID) {
			count++
		}
	}

	return count, nil
}

// GetOldestRunningTunnel returns the tunnel that has been running the longest
func GetOldestRunningTunnel() (*PidInfo, error) {
	pidData, err := LoadPids()
	if err != nil {
		return nil, err
	}

	var oldest *PidInfo
	var oldestTime time.Time

	for _, entry := range pidData.Pids {
		if !isProcessRunning(entry.PID) {
			continue
		}

		startTime, err := time.Parse(time.RFC3339, entry.Started)
		if err != nil {
			continue
		}

		if oldest == nil || startTime.Before(oldestTime) {
			entryCopy := entry
			oldest = &entryCopy
			oldestTime = startTime
		}
	}

	if oldest == nil {
		return nil, fmt.Errorf("no running tunnels found")
	}

	return oldest, nil
}

// Backward compatibility types

// PIDStore is deprecated, use FilePidStore instead
type PIDStore = FilePidStore

// NewPIDStore is deprecated, use NewFilePidStore instead
func NewPIDStore() (*FilePidStore, error) {
	return NewFilePidStore()
}

// ConfigStore is deprecated, use FileConfigStore instead
type ConfigStore = FileConfigStore

// NewConfigStore is deprecated, use NewFileConfigStore with custom path instead
func NewConfigStore(configPath string) (*FileConfigStore, error) {
	if configPath == "" {
		return NewFileConfigStore()
	}
	// For custom path, create a store with the specified path
	return &FileConfigStore{configPath: configPath}, nil
}