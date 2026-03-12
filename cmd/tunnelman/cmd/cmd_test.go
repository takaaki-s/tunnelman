package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takaaki-s/tunnelman/internal/config"
	"github.com/takaaki-s/tunnelman/internal/daemon"
)

var cmdTestCount atomic.Int64

// fakeCommander returns long-running sleep processes for testing.
type fakeCommander struct{}

func (fakeCommander) Command(name string, args ...string) *exec.Cmd {
	return exec.Command("sleep", "3600")
}

func setupTestDaemon(t *testing.T) (*daemon.Server, *daemon.Client) {
	t.Helper()
	// Use short temp dir to avoid exceeding macOS Unix socket path limit (104 bytes)
	n := cmdTestCount.Add(1)
	dir, err := os.MkdirTemp("", fmt.Sprintf("tc%d", n))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s.sock")
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	sm, err := config.NewStateManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	srv := daemon.NewServer(sock, cfg, cfgPath, sm)
	srv.SetProcessManager(daemon.NewProcessManager(fakeCommander{}))

	go srv.Start()

	client := daemon.NewClient(sock)
	for i := 0; i < 50; i++ {
		if client.IsRunning() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { srv.Stop() })

	// Set globals for command helpers
	socketPath = sock
	configPath = cfgPath

	return srv, daemon.NewClient(sock)
}

func TestAddCommand(t *testing.T) {
	_, client := setupTestDaemon(t)

	err := client.Add(daemon.AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	lr, err := client.List(daemon.ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.Tunnels) != 1 || lr.Tunnels[0].Name != "web" {
		t.Errorf("List() = %+v, want 1 tunnel named web", lr.Tunnels)
	}
}

func TestListCommand(t *testing.T) {
	_, client := setupTestDaemon(t)

	_ = client.Add(daemon.AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})
	_ = client.Add(daemon.AddRequest{
		ID: "t2", Name: "api", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 9090, RemoteHost: "api", RemotePort: 3000,
		Profile: "dev",
	})

	// List all
	lr, err := client.List(daemon.ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.Tunnels) != 2 {
		t.Errorf("List() count = %d, want 2", len(lr.Tunnels))
	}

	// Filter by profile
	lr, err = client.List(daemon.ListRequest{Profile: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.Tunnels) != 1 || lr.Tunnels[0].ID != "t2" {
		t.Errorf("List(profile=dev) = %+v, want 1 tunnel t2", lr.Tunnels)
	}
}

func TestRemoveCommand(t *testing.T) {
	_, client := setupTestDaemon(t)

	_ = client.Add(daemon.AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})

	if err := client.Remove("t1"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	lr, _ := client.List(daemon.ListRequest{})
	if len(lr.Tunnels) != 0 {
		t.Errorf("List() after remove = %d, want 0", len(lr.Tunnels))
	}
}

func TestStatusCommand(t *testing.T) {
	_, client := setupTestDaemon(t)

	_ = client.Add(daemon.AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})

	info, err := client.Status("t1")
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "t1" || info.Status != "stopped" {
		t.Errorf("Status() = %+v, want ID=t1 status=stopped", info)
	}
}

func TestStartStopCommand(t *testing.T) {
	_, client := setupTestDaemon(t)

	_ = client.Add(daemon.AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})

	if err := client.Start("t1"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	info, _ := client.Status("t1")
	if info.Status != "running" {
		t.Errorf("Status() after start = %q, want running", info.Status)
	}

	if err := client.Stop("t1"); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	info, _ = client.Status("t1")
	if info.Status != "stopped" {
		t.Errorf("Status() after stop = %q, want stopped", info.Status)
	}
}

func TestProfileCommands(t *testing.T) {
	_, client := setupTestDaemon(t)

	// Create
	if err := client.ProfileCreate("dev", "Development"); err != nil {
		t.Fatal(err)
	}

	// List
	pl, err := client.ProfileList()
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Profiles) != 1 || pl.Profiles[0].Name != "dev" {
		t.Errorf("ProfileList() = %+v, want 1 profile named dev", pl.Profiles)
	}

	// Remove
	if err := client.ProfileRemove("dev"); err != nil {
		t.Fatal(err)
	}

	pl, _ = client.ProfileList()
	if len(pl.Profiles) != 0 {
		t.Errorf("ProfileList() after remove = %d, want 0", len(pl.Profiles))
	}
}

func TestDaemonStatusCommand(t *testing.T) {
	_, client := setupTestDaemon(t)

	ds, err := client.DaemonStatus()
	if err != nil {
		t.Fatal(err)
	}
	if ds.PID <= 0 {
		t.Errorf("DaemonStatus().PID = %d, want > 0", ds.PID)
	}
}

func TestEditCommand(t *testing.T) {
	_, client := setupTestDaemon(t)

	_ = client.Add(daemon.AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})

	newName := "web-edited"
	newPort := 9090
	if err := client.Edit(daemon.EditRequest{ID: "t1", Name: &newName, LocalPort: &newPort}); err != nil {
		t.Fatal(err)
	}

	info, _ := client.Status("t1")
	if info.Name != "web-edited" || info.LocalPort != 9090 {
		t.Errorf("Status() after edit = %+v, want name=web-edited port=9090", info)
	}
}

func TestDaemonNotRunning(t *testing.T) {
	badClient := daemon.NewClient("/tmp/nonexistent-tunnelman-test.sock")
	if badClient.IsRunning() {
		t.Error("IsRunning() should return false for non-existent socket")
	}

	_, err := badClient.List(daemon.ListRequest{})
	if err == nil {
		t.Error("List() should return error when daemon is not running")
	}
}
