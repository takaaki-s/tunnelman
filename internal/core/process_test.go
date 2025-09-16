// Package core provides process management tests for SSH tunnels.
package core

import (
	"context"
	"testing"
	"time"
)

// TestProcessManagerCreation tests the creation of ProcessManager
func TestProcessManagerCreation(t *testing.T) {
	tests := []struct {
		name  string
		debug bool
	}{
		{
			name:  "Create without debug",
			debug: false,
		},
		{
			name:  "Create with debug",
			debug: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewProcessManager(WithDebug(tt.debug))
			if pm == nil {
				t.Fatal("ProcessManager should not be nil")
			}
			if pm.debug != tt.debug {
				t.Errorf("Expected debug=%v, got %v", tt.debug, pm.debug)
			}
			if pm.processes == nil {
				t.Fatal("processes map should be initialized")
			}
		})
	}
}

// TestBuildSSHArgs tests SSH argument construction
func TestBuildSSHArgs(t *testing.T) {
	pm := NewProcessManager()

	tests := []struct {
		name     string
		tunnel   *Tunnel
		expected []string
	}{
		{
			name: "Local forward tunnel",
			tunnel: &Tunnel{
				ID:         "test-local",
				Name:       "Test Local",
				Type:       LocalForward,
				LocalHost:  "127.0.0.1",
				LocalPort:  8080,
				RemoteHost: "192.168.1.1",
				RemotePort: 80,
				SSHHost:    "example.com",
			},
			expected: []string{
				"-L", "127.0.0.1:8080:192.168.1.1:80",
				"-N", "-T",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "StrictHostKeyChecking=accept-new",
				"example.com",
			},
		},
		{
			name: "Remote forward tunnel",
			tunnel: &Tunnel{
				ID:         "test-remote",
				Name:       "Test Remote",
				Type:       RemoteForward,
				LocalHost:  "127.0.0.1",
				LocalPort:  3000,
				RemotePort: 3000,
				SSHHost:    "example.com",
			},
			expected: []string{
				"-R", "3000:127.0.0.1:3000",
				"-N", "-T",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "StrictHostKeyChecking=accept-new",
				"example.com",
			},
		},
		{
			name: "Dynamic forward tunnel",
			tunnel: &Tunnel{
				ID:        "test-dynamic",
				Name:      "Test Dynamic",
				Type:      DynamicForward,
				LocalHost: "127.0.0.1",
				LocalPort: 1080,
				SSHHost:   "example.com",
			},
			expected: []string{
				"-D", "127.0.0.1:1080",
				"-N", "-T",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "StrictHostKeyChecking=accept-new",
				"example.com",
			},
		},
		{
			name: "Tunnel with extra args",
			tunnel: &Tunnel{
				ID:         "test-extra",
				Name:       "Test Extra",
				Type:       LocalForward,
				LocalHost:  "127.0.0.1",
				LocalPort:  8080,
				RemoteHost: "localhost",
				RemotePort: 80,
				SSHHost:    "example.com",
				ExtraArgs:  []string{"-p", "2222", "-l", "myuser"},
			},
			expected: []string{
				"-L", "127.0.0.1:8080:localhost:80",
				"-N", "-T",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "StrictHostKeyChecking=accept-new",
				"-p", "2222", "-l", "myuser",
				"example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := pm.buildSSHArgs(tt.tunnel)

			// Check length
			if len(args) != len(tt.expected) {
				t.Errorf("Expected %d args, got %d", len(tt.expected), len(args))
				t.Logf("Expected: %v", tt.expected)
				t.Logf("Got: %v", args)
				return
			}

			// Check each argument
			for i, expected := range tt.expected {
				if args[i] != expected {
					t.Errorf("Arg[%d]: expected %q, got %q", i, expected, args[i])
				}
			}
		})
	}
}

// TestBuildSSHArgsWithDebug tests SSH arguments with debug mode
func TestBuildSSHArgsWithDebug(t *testing.T) {
	pm := NewProcessManager(WithDebug(true))

	tunnel := &Tunnel{
		ID:         "test-debug",
		Name:       "Test Debug",
		Type:       LocalForward,
		LocalHost:  "127.0.0.1",
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
		SSHHost:    "example.com",
	}

	args := pm.buildSSHArgs(tunnel)

	// Check for verbose flag
	verboseFound := false
	for _, arg := range args {
		if arg == "-v" {
			verboseFound = true
			break
		}
	}

	if !verboseFound {
		t.Error("Expected -v flag in debug mode")
	}
}

// TestProcessInfoManagement tests process info storage and retrieval
func TestProcessInfoManagement(t *testing.T) {
	pm := NewProcessManager()

	// Test GetProcessInfo with non-existent ID
	info, exists := pm.GetProcessInfo("non-existent")
	if exists {
		t.Error("Should not find non-existent process")
	}
	if info != nil {
		t.Error("Info should be nil for non-existent process")
	}

	// Test GetAllProcesses on empty manager
	processes := pm.GetAllProcesses()
	if len(processes) != 0 {
		t.Error("Should have no processes initially")
	}
}

// TestIsProcessRunning tests process existence checking
func TestIsProcessRunning(t *testing.T) {
	pm := NewProcessManager()

	// Test with invalid PID
	if pm.IsProcessRunning(-1) {
		t.Error("Invalid PID should not be running")
	}

	if pm.IsProcessRunning(0) {
		t.Error("PID 0 should not be running")
	}

	// Test with current process (should be running)
	currentPID := int(time.Now().Unix() % 100000) // Use a likely non-existent PID
	if pm.IsProcessRunning(currentPID) {
		t.Error("Random PID should not be running")
	}
}

// TestCleanupEmptyManager tests cleanup with no processes
func TestCleanupEmptyManager(t *testing.T) {
	pm := NewProcessManager()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := pm.Cleanup(ctx)
	if err != nil {
		t.Errorf("Cleanup should succeed with no processes: %v", err)
	}
}

// TestTunnelValidation tests tunnel configuration validation
func TestTunnelValidation(t *testing.T) {
	tests := []struct {
		name      string
		tunnel    *Tunnel
		expectErr bool
	}{
		{
			name: "Valid local forward",
			tunnel: &Tunnel{
				Name:       "Valid",
				Type:       LocalForward,
				LocalPort:  8080,
				RemotePort: 80,
				SSHHost:    "example.com",
			},
			expectErr: false,
		},
		{
			name: "Missing tunnel name",
			tunnel: &Tunnel{
				Type:       LocalForward,
				LocalPort:  8080,
				RemotePort: 80,
				SSHHost:    "example.com",
			},
			expectErr: true,
		},
		{
			name: "Missing SSH host",
			tunnel: &Tunnel{
				Name:       "Test",
				Type:       LocalForward,
				LocalPort:  8080,
				RemotePort: 80,
			},
			expectErr: true,
		},
		{
			name: "Invalid local port",
			tunnel: &Tunnel{
				Name:       "Test",
				Type:       LocalForward,
				LocalPort:  70000,
				RemotePort: 80,
				SSHHost:    "example.com",
			},
			expectErr: true,
		},
		{
			name: "Invalid remote port",
			tunnel: &Tunnel{
				Name:       "Test",
				Type:       LocalForward,
				LocalPort:  8080,
				RemotePort: -1,
				SSHHost:    "example.com",
			},
			expectErr: true,
		},
		{
			name: "Valid dynamic forward",
			tunnel: &Tunnel{
				Name:      "Valid Dynamic",
				Type:      DynamicForward,
				LocalPort: 1080,
				SSHHost:   "example.com",
			},
			expectErr: false,
		},
		{
			name: "Valid remote forward",
			tunnel: &Tunnel{
				Name:       "Valid Remote",
				Type:       RemoteForward,
				LocalPort:  3000,
				RemotePort: 3000,
				SSHHost:    "example.com",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tunnel.Validate()
			if tt.expectErr && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

// TestNewPidEntry tests PID entry creation
func TestNewPidEntry(t *testing.T) {
	pid := 12345
	tunnelID := "test-tunnel"

	entry := NewPidEntry(pid, tunnelID)

	if entry == nil {
		t.Fatal("PidEntry should not be nil")
	}

	if entry.PID != pid {
		t.Errorf("Expected PID %d, got %d", pid, entry.PID)
	}

	if entry.TunnelID != tunnelID {
		t.Errorf("Expected TunnelID %s, got %s", tunnelID, entry.TunnelID)
	}

	// Check timestamp format
	parsedTime, err := entry.GetStartedTime()
	if err != nil {
		t.Errorf("Failed to parse started time: %v", err)
	}

	// Time should be recent (within last minute)
	timeDiff := time.Since(parsedTime)
	if timeDiff < 0 || timeDiff > time.Minute {
		t.Errorf("Started time seems incorrect: %v", parsedTime)
	}
}