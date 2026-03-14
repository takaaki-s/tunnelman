package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takaaki-s/tunnelman/internal/config"
)

var testServerCount atomic.Int64

// fakeCommander returns long-running sleep processes for testing.
type fakeCommander struct{}

func (fakeCommander) Command(name string, args ...string) *exec.Cmd {
	return exec.Command("sleep", "3600")
}

func setupTestServer(t *testing.T) (*Server, *Client) {
	t.Helper()
	// Use a short temp dir to avoid exceeding macOS Unix socket path limit (104 bytes)
	n := testServerCount.Add(1)
	dir, err := os.MkdirTemp("", fmt.Sprintf("tm%d", n))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "s.sock")
	configPath := filepath.Join(dir, "config.yaml")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	sm, err := config.NewStateManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(socketPath, cfg, configPath, sm)
	srv.processManager = NewProcessManager(fakeCommander{})
	// Re-register onExit callback since replacing processManager loses it.
	srv.processManager.SetOnExit(func(tunnelID string) {
		srv.handleProcessExit(tunnelID)
	})

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	// Check for early Start() failure before polling
	client := NewClient(socketPath)
	select {
	case srvErr := <-errCh:
		t.Fatalf("Server.Start() returned early: %v", srvErr)
	case <-time.After(50 * time.Millisecond):
		// Server is starting, proceed to wait
	}

	waitForServer(t, client)
	t.Cleanup(func() { srv.Stop() })

	return srv, NewClient(socketPath)
}

func addTestTunnel(t *testing.T, client *Client, id, name string) {
	t.Helper()
	err := client.Add(AddRequest{
		ID: id, Name: name, Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})
	if err != nil {
		t.Fatalf("Add(%s) error = %v", id, err)
	}
}

func TestServerStartStop(t *testing.T) {
	_, client := setupTestServer(t)
	if !client.IsRunning() {
		t.Error("server should be running")
	}
}

func TestServerAddTunnel(t *testing.T) {
	_, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	lr, err := client.List(ListRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(lr.Tunnels) != 1 || lr.Tunnels[0].ID != "t1" {
		t.Errorf("List() = %+v, want 1 tunnel with ID t1", lr.Tunnels)
	}
}

func TestServerAddDuplicate(t *testing.T) {
	_, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	err := client.Add(AddRequest{
		ID: "t1", Name: "web2", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 9090, RemoteHost: "db", RemotePort: 5432,
	})
	if err == nil {
		t.Error("Add duplicate should return error")
	}
}

func TestServerAddInvalid(t *testing.T) {
	_, client := setupTestServer(t)
	err := client.Add(AddRequest{
		ID: "t1", Name: "", Type: "local",
		SSHHost: "bastion", LocalPort: 8080, RemotePort: 80,
	})
	if err == nil {
		t.Error("Add invalid (missing name) should return error")
	}
}

func TestServerRemoveTunnel(t *testing.T) {
	_, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	if err := client.Remove("t1"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	lr, err := client.List(ListRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(lr.Tunnels) != 0 {
		t.Errorf("List() after remove = %d tunnels, want 0", len(lr.Tunnels))
	}
}

func TestServerRemoveNotFound(t *testing.T) {
	_, client := setupTestServer(t)
	err := client.Remove("nonexistent")
	if err == nil {
		t.Error("Remove(nonexistent) should return error")
	}
}

func TestServerEditTunnel(t *testing.T) {
	_, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	newName := "web-edited"
	newPort := 9090
	if err := client.Edit(EditRequest{ID: "t1", Name: &newName, LocalPort: &newPort}); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}

	info, err := client.Status("t1")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if info.Name != "web-edited" || info.LocalPort != 9090 {
		t.Errorf("Status() = %+v, want name=web-edited, local_port=9090", info)
	}
}

func TestServerListTunnels(t *testing.T) {
	_, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	err := client.Add(AddRequest{
		ID: "t2", Name: "api", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 9090, RemoteHost: "api", RemotePort: 3000,
		Profile: "dev",
	})
	if err != nil {
		t.Fatal(err)
	}

	lr, err := client.List(ListRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(lr.Tunnels) != 2 {
		t.Errorf("List() count = %d, want 2", len(lr.Tunnels))
	}
}

func TestServerListWithProfileFilter(t *testing.T) {
	_, client := setupTestServer(t)

	err := client.Add(AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
		Profile: "dev",
	})
	if err != nil {
		t.Fatalf("Add(t1) error = %v", err)
	}

	err = client.Add(AddRequest{
		ID: "t2", Name: "api", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 9090, RemoteHost: "api", RemotePort: 3000,
		Profile: "prod",
	})
	if err != nil {
		t.Fatalf("Add(t2) error = %v", err)
	}

	lr, err := client.List(ListRequest{Profile: "dev"})
	if err != nil {
		t.Fatalf("List(profile=dev) error = %v", err)
	}
	if len(lr.Tunnels) != 1 || lr.Tunnels[0].ID != "t1" {
		t.Errorf("List(profile=dev) = %+v, want 1 tunnel t1", lr.Tunnels)
	}
}

func TestServerStatusTunnel(t *testing.T) {
	_, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	info, err := client.Status("t1")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if info.ID != "t1" || info.Status != "stopped" {
		t.Errorf("Status() = %+v, want ID=t1 Status=stopped", info)
	}
}

func TestServerStatusNotFound(t *testing.T) {
	_, client := setupTestServer(t)
	_, err := client.Status("nonexistent")
	if err == nil {
		t.Error("Status(nonexistent) should return error")
	}
}

func TestServerDaemonStatus(t *testing.T) {
	_, client := setupTestServer(t)
	ds, err := client.DaemonStatus()
	if err != nil {
		t.Fatalf("DaemonStatus() error = %v", err)
	}
	if ds.PID <= 0 {
		t.Errorf("DaemonStatus().PID = %d, want > 0", ds.PID)
	}
}

func TestServerStartTunnel(t *testing.T) {
	_, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	if err := client.Start("t1"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	info, err := client.Status("t1")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if info.Status != "running" || info.PID <= 0 {
		t.Errorf("Status() = %+v, want running with PID", info)
	}
}

func TestServerStopTunnel(t *testing.T) {
	_, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	if err := client.Start("t1"); err != nil {
		t.Fatal(err)
	}
	if err := client.Stop("t1"); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	info, err := client.Status("t1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != "stopped" {
		t.Errorf("Status() = %q, want stopped", info.Status)
	}
}

func TestServerProfileList(t *testing.T) {
	_, client := setupTestServer(t)

	pl, err := client.ProfileList()
	if err != nil {
		t.Fatalf("ProfileList() error = %v", err)
	}
	if len(pl.Profiles) != 0 {
		t.Errorf("ProfileList() count = %d, want 0", len(pl.Profiles))
	}
}

func TestServerProfileCreate(t *testing.T) {
	_, client := setupTestServer(t)

	if err := client.ProfileCreate("dev", "Development"); err != nil {
		t.Fatalf("ProfileCreate() error = %v", err)
	}

	pl, err := client.ProfileList()
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Profiles) != 1 || pl.Profiles[0].Name != "dev" {
		t.Errorf("ProfileList() = %+v, want 1 profile named dev", pl.Profiles)
	}
}

func TestServerProfileRemove(t *testing.T) {
	_, client := setupTestServer(t)
	_ = client.ProfileCreate("dev", "Development")

	if err := client.ProfileRemove("dev"); err != nil {
		t.Fatalf("ProfileRemove() error = %v", err)
	}

	pl, err := client.ProfileList()
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Profiles) != 0 {
		t.Errorf("ProfileList() after remove = %d, want 0", len(pl.Profiles))
	}
}

func TestReconnectManualStopNoReconnect(t *testing.T) {
	srv, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	// Replace reconnector for testing. This works because NewServer's onExit
	// closure captures `s` (the Server pointer), so it sees the replaced reconnector.
	srv.reconnector = NewReconnectManager("fixed", 50*time.Millisecond, 1*time.Second, 5)
	var reconnected atomic.Int32
	srv.reconnector.SetOnReconnect(func(tunnelID string) error {
		reconnected.Add(1)
		return nil
	})

	// Start the tunnel
	if err := client.Start("t1"); err != nil {
		t.Fatal(err)
	}

	// Manually stop the tunnel (should set manualStop flag)
	if err := client.Stop("t1"); err != nil {
		t.Fatal(err)
	}

	// Wait to see if reconnect fires (it should NOT)
	time.Sleep(200 * time.Millisecond)

	if reconnected.Load() != 0 {
		t.Errorf("expected 0 reconnect attempts after manual stop, got %d", reconnected.Load())
	}
}

func TestReconnectAfterManualStopAndRestart(t *testing.T) {
	srv, client := setupTestServer(t)
	addTestTunnel(t, client, "t1", "web")

	// Replace reconnector for testing (see TestReconnectManualStopNoReconnect comment).
	srv.reconnector = NewReconnectManager("fixed", 50*time.Millisecond, 1*time.Second, 5)
	var reconnected atomic.Int32
	srv.reconnector.SetOnReconnect(func(tunnelID string) error {
		reconnected.Add(1)
		return nil
	})

	// Start → Stop (sets manualStop) → Start again (clears manualStop)
	if err := client.Start("t1"); err != nil {
		t.Fatal(err)
	}
	if err := client.Stop("t1"); err != nil {
		t.Fatal(err)
	}
	if err := client.Start("t1"); err != nil {
		t.Fatal(err)
	}

	// Simulate crash: kill the process directly (bypass manualStop)
	pi := srv.processManager.GetProcessInfo("t1")
	if pi == nil {
		t.Fatal("process not found for t1")
	}
	_ = pi.Cmd.Process.Kill()

	// Wait for reconnect to fire (it SHOULD because manualStop was cleared by Start)
	time.Sleep(300 * time.Millisecond)

	if reconnected.Load() < 1 {
		t.Errorf("expected reconnect after stop→start→crash, got %d attempts", reconnected.Load())
	}
}

func TestServerProfileRemoveWithTunnels(t *testing.T) {
	_, client := setupTestServer(t)
	_ = client.ProfileCreate("dev", "Development")

	_ = client.Add(AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
		Profile: "dev",
	})

	err := client.ProfileRemove("dev")
	if err == nil {
		t.Error("ProfileRemove with tunnels should return error")
	}
}
