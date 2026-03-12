package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `version: "2"
profiles:
  - name: dev
    description: Development tunnels
tunnels:
  - id: t1
    name: web
    type: local
    ssh_host: bastion
    local_host: "127.0.0.1"
    local_port: 8080
    remote_host: db
    remote_port: 5432
    profile: dev
    auto_connect: false
health_check:
  enabled: true
  interval_seconds: 30
  timeout_seconds: 5
  max_failures: 3
reconnect:
  enabled: true
  strategy: exponential
  initial_delay_seconds: 1
  max_delay_seconds: 300
  max_retries: 10
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Version != "2" {
		t.Errorf("Version = %q, want %q", cfg.Version, "2")
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "dev" {
		t.Errorf("Profiles = %+v, want 1 profile named dev", cfg.Profiles)
	}
	if len(cfg.Tunnels) != 1 {
		t.Fatalf("Tunnels count = %d, want 1", len(cfg.Tunnels))
	}
	tun := cfg.Tunnels[0]
	if tun.ID != "t1" || tun.Name != "web" || tun.Type != "local" {
		t.Errorf("Tunnel = %+v", tun)
	}
	if tun.LocalPort != 8080 || tun.RemotePort != 5432 {
		t.Errorf("Tunnel ports: local=%d remote=%d", tun.LocalPort, tun.RemotePort)
	}
	if cfg.HealthCheck == nil || !cfg.HealthCheck.Enabled {
		t.Error("HealthCheck not loaded")
	}
	if cfg.Reconnect == nil || cfg.Reconnect.Strategy != "exponential" {
		t.Error("Reconnect not loaded")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.yaml")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Version != "2" {
		t.Errorf("Version = %q, want %q", cfg.Version, "2")
	}
	if len(cfg.Tunnels) != 0 {
		t.Errorf("Tunnels count = %d, want 0", len(cfg.Tunnels))
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("version: [\ninvalid:\n  - {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("LoadConfig() expected error for invalid YAML")
	}
}

func TestSaveConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := DefaultConfig()
	original.Tunnels = append(original.Tunnels, TunnelConfig{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})
	original.Profiles = append(original.Profiles, ProfileConfig{
		Name: "dev", Description: "Development",
	})

	if err := SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("Version = %q, want %q", loaded.Version, original.Version)
	}
	if len(loaded.Tunnels) != 1 {
		t.Fatalf("Tunnels count = %d, want 1", len(loaded.Tunnels))
	}
	if loaded.Tunnels[0].ID != "t1" || loaded.Tunnels[0].Name != "web" {
		t.Errorf("Tunnel mismatch: %+v", loaded.Tunnels[0])
	}
	if len(loaded.Profiles) != 1 || loaded.Profiles[0].Name != "dev" {
		t.Errorf("Profiles mismatch: %+v", loaded.Profiles)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Version: "2",
				Tunnels: []TunnelConfig{
					{ID: "t1", Name: "a", Type: "local", SSHHost: "h", LocalPort: 8080, RemotePort: 80},
					{ID: "t2", Name: "b", Type: "local", SSHHost: "h", LocalPort: 9090, RemotePort: 80},
				},
			},
		},
		{
			name: "duplicate tunnel ID",
			cfg: Config{
				Version: "2",
				Tunnels: []TunnelConfig{
					{ID: "t1", Name: "a", Type: "local", SSHHost: "h", LocalPort: 8080, RemotePort: 80},
					{ID: "t1", Name: "b", Type: "local", SSHHost: "h", LocalPort: 9090, RemotePort: 80},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid tunnel port",
			cfg: Config{
				Version: "2",
				Tunnels: []TunnelConfig{
					{ID: "t1", Name: "a", Type: "local", SSHHost: "h", LocalPort: 0, RemotePort: 80},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestXDGPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)

	configPath := DefaultConfigPath()
	if configPath != filepath.Join(dir, "tunnelman", "config.yaml") {
		t.Errorf("DefaultConfigPath() = %q, want %q", configPath, filepath.Join(dir, "tunnelman", "config.yaml"))
	}

	statePath := DefaultStatePath()
	if statePath != filepath.Join(dir, "tunnelman", "state.json") {
		t.Errorf("DefaultStatePath() = %q, want %q", statePath, filepath.Join(dir, "tunnelman", "state.json"))
	}

	socketPath := DefaultSocketPath()
	if socketPath != filepath.Join(dir, "tunnelman", "tunnelman.sock") {
		t.Errorf("DefaultSocketPath() = %q, want %q", socketPath, filepath.Join(dir, "tunnelman", "tunnelman.sock"))
	}
}
