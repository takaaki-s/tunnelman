// Package store provides persistent storage for tunnel configurations using XDG Base Directory Specification.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// FileConfigStore implements ConfigStore using file system storage
type FileConfigStore struct {
	configPath string
}

// NewFileConfigStore creates a new file-based configuration store
func NewFileConfigStore() (*FileConfigStore, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	return &FileConfigStore{configPath: configPath}, nil
}

// getConfigPath returns the configuration file path based on XDG Base Directory Specification
func getConfigPath() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		// Windows: Use %AppData%
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.Getenv("USERPROFILE")
			if appData == "" {
				return "", fmt.Errorf("cannot determine Windows config directory")
			}
			appData = filepath.Join(appData, "AppData", "Roaming")
		}
		configDir = filepath.Join(appData, "tunnelman")

	default:
		// Unix-like (Linux, macOS, BSD): Use XDG_CONFIG_HOME
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot determine home directory: %w", err)
			}
			xdgConfigHome = filepath.Join(homeDir, ".config")
		}
		configDir = filepath.Join(xdgConfigHome, "tunnelman")
	}

	// Ensure the config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(configDir, "config.json"), nil
}

// LoadConfig loads the tunnel configuration from the XDG-compliant config file
func (fcs *FileConfigStore) LoadConfig() (*AppConfig, error) {
	// Read the configuration file
	data, err := os.ReadFile(fcs.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default configuration if file doesn't exist
			return &AppConfig{
				Version: "1.0.0",
				Tunnels: []TunnelConfig{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the configuration
	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the tunnel configuration to the XDG-compliant config file
func (fcs *FileConfigStore) SaveConfig(config *AppConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Marshal configuration to JSON with pretty formatting
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to temporary file first for atomic operation
	tempFile := fcs.configPath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		// Log error to stderr for better visibility
		fmt.Fprintf(os.Stderr, "ERROR: Failed to write config file: %v\n", err)
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Atomic rename to ensure data integrity
	if err := os.Rename(tempFile, fcs.configPath); err != nil {
		// Clean up temporary file if rename fails
		os.Remove(tempFile)
		// Log error to stderr for better visibility
		fmt.Fprintf(os.Stderr, "ERROR: Failed to save config file: %v\n", err)
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the current configuration file path
func (fcs *FileConfigStore) GetConfigPath() (string, error) {
	return fcs.configPath, nil
}

// BackupConfig creates a backup of the current configuration
func (fcs *FileConfigStore) BackupConfig() error {
	// Check if config file exists
	if _, err := os.Stat(fcs.configPath); os.IsNotExist(err) {
		// Nothing to backup
		return nil
	}

	// Read current configuration
	data, err := os.ReadFile(fcs.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config for backup: %w", err)
	}

	// Write backup with timestamp suffix
	backupPath := fcs.configPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}

// RestoreConfig restores configuration from the backup file
func (fcs *FileConfigStore) RestoreConfig() error {
	backupPath := fcs.configPath + ".backup"

	// Check if backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup file found at %s", backupPath)
	}

	// Read backup
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Validate backup content
	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("backup file is corrupted: %w", err)
	}

	// Restore configuration
	if err := os.WriteFile(fcs.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}

	return nil
}

// Helper functions for backwards compatibility

// LoadConfig loads configuration using default path
func LoadConfig() (*AppConfig, error) {
	store, err := NewFileConfigStore()
	if err != nil {
		return nil, err
	}
	return store.LoadConfig()
}

// SaveConfig saves configuration using default path
func SaveConfig(config *AppConfig) error {
	store, err := NewFileConfigStore()
	if err != nil {
		return err
	}
	return store.SaveConfig(config)
}

// GetConfigPath returns the default configuration path
func GetConfigPath() (string, error) {
	return getConfigPath()
}

// BackupConfig creates a backup using default path
func BackupConfig() error {
	store, err := NewFileConfigStore()
	if err != nil {
		return err
	}
	return store.BackupConfig()
}

// RestoreConfig restores from backup using default path
func RestoreConfig() error {
	store, err := NewFileConfigStore()
	if err != nil {
		return err
	}
	return store.RestoreConfig()
}