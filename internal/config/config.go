// Package config provides configuration and state management for tunnelman.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Config represents the tunnelman configuration file.
type Config struct {
	Version     string             `yaml:"version"`
	Profiles    []ProfileConfig    `yaml:"profiles,omitempty"`
	Tunnels     []TunnelConfig     `yaml:"tunnels,omitempty"`
	HealthCheck *HealthCheckConfig `yaml:"health_check,omitempty"`
	Reconnect   *ReconnectConfig   `yaml:"reconnect,omitempty"`
}

// TunnelConfig represents a tunnel entry in the config file.
// Fields mirror tunnel.Tunnel but config does not import tunnel.
type TunnelConfig struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Type        string   `yaml:"type" json:"type"`
	SSHHost     string   `yaml:"ssh_host" json:"ssh_host"`
	LocalHost   string   `yaml:"local_host" json:"local_host"`
	LocalPort   int      `yaml:"local_port" json:"local_port"`
	RemoteHost  string   `yaml:"remote_host" json:"remote_host"`
	RemotePort  int      `yaml:"remote_port" json:"remote_port"`
	Profile     string   `yaml:"profile" json:"profile"`
	AutoConnect bool     `yaml:"auto_connect" json:"auto_connect"`
	SSHOptions  []string `yaml:"ssh_options,omitempty" json:"ssh_options,omitempty"`
}

// ProfileConfig represents a profile entry in the config file.
type ProfileConfig struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
}

// HealthCheckConfig configures health checking behavior.
type HealthCheckConfig struct {
	Enabled         bool `yaml:"enabled" json:"enabled"`
	IntervalSeconds int  `yaml:"interval_seconds" json:"interval_seconds"`
	TimeoutSeconds  int  `yaml:"timeout_seconds" json:"timeout_seconds"`
	MaxFailures     int  `yaml:"max_failures" json:"max_failures"`
}

// ReconnectConfig configures automatic reconnection behavior.
type ReconnectConfig struct {
	Enabled             bool   `yaml:"enabled" json:"enabled"`
	Strategy            string `yaml:"strategy" json:"strategy"`
	InitialDelaySeconds int    `yaml:"initial_delay_seconds" json:"initial_delay_seconds"`
	MaxDelaySeconds     int    `yaml:"max_delay_seconds" json:"max_delay_seconds"`
	MaxRetries          int    `yaml:"max_retries" json:"max_retries"`
}

// LoadConfig reads a config file from path. Returns default config if file does not exist.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes config to path using atomic write.
func SaveConfig(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// DefaultConfig returns a new config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Version: "2",
		Tunnels: []TunnelConfig{},
	}
}

// Validate checks the config for errors.
func (c *Config) Validate() error {
	seen := make(map[string]bool, len(c.Tunnels))
	for _, t := range c.Tunnels {
		if seen[t.ID] {
			return fmt.Errorf("duplicate tunnel ID: %s", t.ID)
		}
		seen[t.ID] = true

		if t.LocalPort <= 0 || t.LocalPort > 65535 {
			return fmt.Errorf("tunnel %s: invalid local port: %d", t.ID, t.LocalPort)
		}
		switch t.Type {
		case "local", "remote":
			if t.RemotePort <= 0 || t.RemotePort > 65535 {
				return fmt.Errorf("tunnel %s: invalid remote port: %d", t.ID, t.RemotePort)
			}
		case "dynamic":
			// remote port not required
		default:
			return fmt.Errorf("tunnel %s: invalid type: %s", t.ID, t.Type)
		}
	}
	return nil
}

// DefaultConfigPath returns the default config file path following XDG.
func DefaultConfigPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

// DefaultStatePath returns the default state file path following XDG.
func DefaultStatePath() string {
	return filepath.Join(stateDir(), "state.json")
}

// DefaultSocketPath returns the default Unix socket path following XDG.
func DefaultSocketPath() string {
	return filepath.Join(runtimeDir(), "tunnelman.sock")
}

// DefaultLogPath returns the default log file path following XDG.
func DefaultLogPath() string {
	return filepath.Join(stateDir(), "tunnelman.log")
}

func configDir() string {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("APPDATA"); d != "" {
			return filepath.Join(d, "tunnelman")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Roaming", "tunnelman")
	}
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "tunnelman")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tunnelman")
}

func stateDir() string {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "tunnelman")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local", "tunnelman")
	}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "tunnelman")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "tunnelman")
}

func runtimeDir() string {
	if runtime.GOOS == "windows" {
		return stateDir()
	}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "tunnelman")
	}
	return stateDir()
}
