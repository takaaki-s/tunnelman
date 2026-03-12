package tunnel

import (
	"testing"
)

func TestTunnelValidate(t *testing.T) {
	tests := []struct {
		name    string
		tunnel  Tunnel
		wantErr bool
	}{
		{
			name: "valid local forward",
			tunnel: Tunnel{
				ID: "t1", Name: "web", Type: LocalForward,
				SSHHost: "bastion", LocalHost: "127.0.0.1",
				LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
			},
		},
		{
			name: "valid remote forward",
			tunnel: Tunnel{
				ID: "t2", Name: "expose", Type: RemoteForward,
				SSHHost: "bastion", LocalHost: "127.0.0.1",
				LocalPort: 3000, RemoteHost: "0.0.0.0", RemotePort: 8080,
			},
		},
		{
			name: "valid dynamic forward",
			tunnel: Tunnel{
				ID: "t3", Name: "socks", Type: DynamicForward,
				SSHHost: "bastion", LocalHost: "127.0.0.1", LocalPort: 1080,
			},
		},
		{
			name: "missing name",
			tunnel: Tunnel{
				ID: "t4", Type: LocalForward, SSHHost: "bastion",
				LocalHost: "127.0.0.1", LocalPort: 8080,
				RemoteHost: "db", RemotePort: 5432,
			},
			wantErr: true,
		},
		{
			name: "missing ssh_host",
			tunnel: Tunnel{
				ID: "t5", Name: "web", Type: LocalForward,
				LocalHost: "127.0.0.1", LocalPort: 8080,
				RemoteHost: "db", RemotePort: 5432,
			},
			wantErr: true,
		},
		{
			name: "local port out of range (0)",
			tunnel: Tunnel{
				ID: "t6", Name: "web", Type: LocalForward,
				SSHHost: "bastion", LocalHost: "127.0.0.1",
				LocalPort: 0, RemoteHost: "db", RemotePort: 5432,
			},
			wantErr: true,
		},
		{
			name: "local port out of range (65536)",
			tunnel: Tunnel{
				ID: "t7", Name: "web", Type: LocalForward,
				SSHHost: "bastion", LocalHost: "127.0.0.1",
				LocalPort: 65536, RemoteHost: "db", RemotePort: 5432,
			},
			wantErr: true,
		},
		{
			name: "remote port out of range for local forward",
			tunnel: Tunnel{
				ID: "t8", Name: "web", Type: LocalForward,
				SSHHost: "bastion", LocalHost: "127.0.0.1",
				LocalPort: 8080, RemoteHost: "db", RemotePort: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid tunnel type",
			tunnel: Tunnel{
				ID: "t9", Name: "web", Type: "invalid",
				SSHHost: "bastion", LocalHost: "127.0.0.1", LocalPort: 8080,
			},
			wantErr: true,
		},
		{
			name: "dynamic forward ignores remote fields",
			tunnel: Tunnel{
				ID: "t10", Name: "socks", Type: DynamicForward,
				SSHHost: "bastion", LocalHost: "127.0.0.1", LocalPort: 1080,
				RemoteHost: "", RemotePort: 0,
			},
		},
		{
			name: "local forward defaults remote host",
			tunnel: Tunnel{
				ID: "t11", Name: "web", Type: LocalForward,
				SSHHost: "bastion", LocalHost: "127.0.0.1",
				LocalPort: 8080, RemotePort: 80,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tunnel.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildSSHCommand(t *testing.T) {
	tests := []struct {
		name     string
		tunnel   Tunnel
		wantArgs []string
	}{
		{
			name: "local forward",
			tunnel: Tunnel{
				Type: LocalForward, SSHHost: "bastion",
				LocalHost: "127.0.0.1", LocalPort: 8080,
				RemoteHost: "db", RemotePort: 5432,
			},
			wantArgs: []string{
				"ssh",
				"-L", "127.0.0.1:8080:db:5432",
				"-N", "-T",
			},
		},
		{
			name: "remote forward",
			tunnel: Tunnel{
				Type: RemoteForward, SSHHost: "bastion",
				LocalHost: "127.0.0.1", LocalPort: 3000,
				RemoteHost: "0.0.0.0", RemotePort: 8080,
			},
			wantArgs: []string{
				"ssh",
				"-R", "8080:127.0.0.1:3000",
				"-N", "-T",
			},
		},
		{
			name: "dynamic forward",
			tunnel: Tunnel{
				Type: DynamicForward, SSHHost: "bastion",
				LocalHost: "127.0.0.1", LocalPort: 1080,
			},
			wantArgs: []string{
				"ssh",
				"-D", "127.0.0.1:1080",
				"-N", "-T",
			},
		},
		{
			name: "with ssh_options",
			tunnel: Tunnel{
				Type: LocalForward, SSHHost: "bastion",
				LocalHost: "127.0.0.1", LocalPort: 8080,
				RemoteHost: "db", RemotePort: 5432,
				SSHOptions: []string{"-v", "-o", "ProxyJump=jump"},
			},
			wantArgs: []string{
				"ssh",
				"-L", "127.0.0.1:8080:db:5432",
				"-N", "-T",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tunnel.BuildSSHCommand()

			// Check required args are present in order
			for _, want := range tt.wantArgs {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("BuildSSHCommand() missing arg %q, got %v", want, got)
				}
			}

			// Last arg must be SSHHost
			if got[len(got)-1] != tt.tunnel.SSHHost {
				t.Errorf("BuildSSHCommand() last arg = %q, want %q", got[len(got)-1], tt.tunnel.SSHHost)
			}

			// SSHOptions should appear before SSHHost
			if len(tt.tunnel.SSHOptions) > 0 {
				for _, opt := range tt.tunnel.SSHOptions {
					found := false
					for _, g := range got {
						if g == opt {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("BuildSSHCommand() missing ssh_option %q, got %v", opt, got)
					}
				}
			}
		})
	}
}

func TestTunnelClone(t *testing.T) {
	original := &Tunnel{
		ID: "t1", Name: "web", Type: LocalForward,
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
		Profile: "dev", AutoConnect: true,
		SSHOptions: []string{"-v", "-o", "ProxyJump=jump"},
	}

	clone := original.Clone()

	// Verify equality
	if clone.ID != original.ID || clone.Name != original.Name ||
		clone.Type != original.Type || clone.SSHHost != original.SSHHost ||
		clone.LocalHost != original.LocalHost || clone.LocalPort != original.LocalPort ||
		clone.RemoteHost != original.RemoteHost || clone.RemotePort != original.RemotePort ||
		clone.Profile != original.Profile || clone.AutoConnect != original.AutoConnect {
		t.Error("Clone() fields do not match original")
	}

	// Verify SSHOptions is a deep copy
	if len(clone.SSHOptions) != len(original.SSHOptions) {
		t.Fatalf("Clone() SSHOptions length mismatch")
	}

	// Mutate clone and verify independence
	clone.Name = "modified"
	clone.SSHOptions[0] = "-vv"
	if original.Name == "modified" {
		t.Error("Clone() Name mutation affected original")
	}
	if original.SSHOptions[0] == "-vv" {
		t.Error("Clone() SSHOptions mutation affected original")
	}
}

func TestTunnelStatusConstants(t *testing.T) {
	tests := []struct {
		status TunnelStatus
		want   string
	}{
		{StatusStopped, "stopped"},
		{StatusStarting, "starting"},
		{StatusRunning, "running"},
		{StatusUnhealthy, "unhealthy"},
		{StatusError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("status = %q, want %q", tt.status, tt.want)
			}
		})
	}
}
