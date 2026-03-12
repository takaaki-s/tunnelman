package sshconfig

import (
	"os"
	"path/filepath"
	"testing"
)

const testSSHConfig = `# Test SSH config
Host bastion
    HostName 10.0.0.1
    User admin
    Port 2222
    LocalForward 8080 localhost:80
    LocalForward 127.0.0.1:5432 db:5432
    RemoteForward 9090 localhost:3000
    DynamicForward 1080

Host web-*
    User deploy

Host jump
    HostName jump.example.com
    DynamicForward 127.0.0.1:1080
`

func writeTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(testSSHConfig), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseHost(t *testing.T) {
	path := writeTestConfig(t)
	p := NewParser(path)

	host, err := p.ParseHost("bastion")
	if err != nil {
		t.Fatalf("ParseHost() error = %v", err)
	}
	if host == nil {
		t.Fatal("ParseHost() returned nil")
	}
	if host.HostName != "10.0.0.1" {
		t.Errorf("HostName = %q, want %q", host.HostName, "10.0.0.1")
	}
	if host.User != "admin" {
		t.Errorf("User = %q, want %q", host.User, "admin")
	}
	if host.Port != 2222 {
		t.Errorf("Port = %d, want %d", host.Port, 2222)
	}
}

func TestParseLocalForward(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want *ForwardSpec
	}{
		{
			name: "port only bind",
			spec: "8080 localhost:80",
			want: &ForwardSpec{BindAddress: "0.0.0.0", BindPort: 8080, Host: "localhost", HostPort: 80},
		},
		{
			name: "explicit bind address",
			spec: "127.0.0.1:5432 db:5432",
			want: &ForwardSpec{BindAddress: "127.0.0.1", BindPort: 5432, Host: "db", HostPort: 5432},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLocalForward(tt.spec)
			if got == nil {
				t.Fatal("parseLocalForward() returned nil")
			}
			if got.BindAddress != tt.want.BindAddress || got.BindPort != tt.want.BindPort ||
				got.Host != tt.want.Host || got.HostPort != tt.want.HostPort {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseRemoteForward(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want *ForwardSpec
	}{
		{
			name: "simple remote forward",
			spec: "9090 localhost:3000",
			want: &ForwardSpec{BindAddress: "0.0.0.0", BindPort: 9090, Host: "localhost", HostPort: 3000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRemoteForward(tt.spec)
			if got == nil {
				t.Fatal("parseRemoteForward() returned nil")
			}
			if got.BindAddress != tt.want.BindAddress || got.BindPort != tt.want.BindPort ||
				got.Host != tt.want.Host || got.HostPort != tt.want.HostPort {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseDynamicForward(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want *DynamicSpec
	}{
		{
			name: "port only",
			spec: "1080",
			want: &DynamicSpec{BindAddress: "0.0.0.0", BindPort: 1080},
		},
		{
			name: "explicit bind address",
			spec: "127.0.0.1:1080",
			want: &DynamicSpec{BindAddress: "127.0.0.1", BindPort: 1080},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDynamicForward(tt.spec)
			if got == nil {
				t.Fatal("parseDynamicForward() returned nil")
			}
			if got.BindAddress != tt.want.BindAddress || got.BindPort != tt.want.BindPort {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestListHosts(t *testing.T) {
	path := writeTestConfig(t)
	p := NewParser(path)

	hosts, err := p.ListHosts()
	if err != nil {
		t.Fatalf("ListHosts() error = %v", err)
	}

	want := map[string]bool{"bastion": false, "web-*": false, "jump": false}
	for _, h := range hosts {
		if _, ok := want[h]; ok {
			want[h] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("ListHosts() missing host %q", name)
		}
	}
}

func TestParseHostNotFound(t *testing.T) {
	path := writeTestConfig(t)
	p := NewParser(path)

	host, err := p.ParseHost("nonexistent")
	if err != nil {
		t.Fatalf("ParseHost() error = %v", err)
	}
	if host != nil {
		t.Errorf("ParseHost(nonexistent) = %+v, want nil", host)
	}
}

func TestParseHostForwards(t *testing.T) {
	path := writeTestConfig(t)
	p := NewParser(path)

	host, err := p.ParseHost("bastion")
	if err != nil {
		t.Fatalf("ParseHost() error = %v", err)
	}

	if len(host.LocalForwards) != 2 {
		t.Errorf("LocalForwards count = %d, want 2", len(host.LocalForwards))
	}
	if len(host.RemoteForwards) != 1 {
		t.Errorf("RemoteForwards count = %d, want 1", len(host.RemoteForwards))
	}
	if len(host.DynamicForwards) != 1 {
		t.Errorf("DynamicForwards count = %d, want 1", len(host.DynamicForwards))
	}
}
