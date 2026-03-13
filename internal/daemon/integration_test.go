package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/takaaki-s/tunnelman/internal/config"
)

func TestDaemonFullLifecycle(t *testing.T) {
	_, client := setupTestServer(t)

	// Add tunnel
	err := client.Add(AddRequest{
		ID: "web", Name: "Web Server", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// List → 1 tunnel
	lr, err := client.List(ListRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(lr.Tunnels) != 1 {
		t.Fatalf("List() count = %d, want 1", len(lr.Tunnels))
	}
	if lr.Tunnels[0].ID != "web" {
		t.Errorf("List()[0].ID = %q, want %q", lr.Tunnels[0].ID, "web")
	}

	// Start tunnel
	if err := client.Start("web"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Status → running
	info, err := client.Status("web")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if info.Status != "running" {
		t.Errorf("Status() = %q, want %q", info.Status, "running")
	}
	if info.PID <= 0 {
		t.Errorf("Status().PID = %d, want > 0", info.PID)
	}

	// Stop tunnel
	if err := client.Stop("web"); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Status → stopped
	info, err = client.Status("web")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if info.Status != "stopped" {
		t.Errorf("Status() after stop = %q, want %q", info.Status, "stopped")
	}

	// Remove tunnel
	if err := client.Remove("web"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// List → 0 tunnels
	lr, err = client.List(ListRequest{})
	if err != nil {
		t.Fatalf("List() after remove error = %v", err)
	}
	if len(lr.Tunnels) != 0 {
		t.Errorf("List() after remove = %d, want 0", len(lr.Tunnels))
	}

	// DaemonStatus
	ds, err := client.DaemonStatus()
	if err != nil {
		t.Fatalf("DaemonStatus() error = %v", err)
	}
	if ds.PID <= 0 {
		t.Errorf("DaemonStatus().PID = %d, want > 0", ds.PID)
	}
	if ds.Tunnels != 0 {
		t.Errorf("DaemonStatus().Tunnels = %d, want 0", ds.Tunnels)
	}
}

func TestDaemonConfigPersistence(t *testing.T) {
	// Setup shared directory for two server instances
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

	// Start first server
	srv1 := NewServer(socketPath, cfg, configPath, sm)
	srv1.processManager = NewProcessManager(fakeCommander{})
	srv1.processManager.SetOnExit(func(tunnelID string) {
		srv1.handleProcessExit(tunnelID)
	})

	go func() { _ = srv1.Start() }()
	client1 := NewClient(socketPath)
	waitForServer(t, client1)

	// Add tunnels via first server
	err = client1.Add(AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})
	if err != nil {
		t.Fatalf("Add(t1) error = %v", err)
	}
	err = client1.Add(AddRequest{
		ID: "t2", Name: "api", Type: "remote",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 9090, RemoteHost: "api", RemotePort: 3000,
	})
	if err != nil {
		t.Fatalf("Add(t2) error = %v", err)
	}

	// Stop first server. Stop() is synchronous — config is persisted before it returns.
	srv1.Stop()

	// Verify config file was persisted
	persistedCfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(persistedCfg.Tunnels) != 2 {
		t.Fatalf("persisted tunnels = %d, want 2", len(persistedCfg.Tunnels))
	}

	// Start second server with same config dir
	sm2, err := config.NewStateManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	srv2 := NewServer(socketPath, persistedCfg, configPath, sm2)
	srv2.processManager = NewProcessManager(fakeCommander{})
	srv2.processManager.SetOnExit(func(tunnelID string) {
		srv2.handleProcessExit(tunnelID)
	})

	go func() { _ = srv2.Start() }()
	client2 := NewClient(socketPath)
	waitForServer(t, client2)
	t.Cleanup(func() { srv2.Stop() })

	// Verify tunnels survived restart
	lr, err := client2.List(ListRequest{})
	if err != nil {
		t.Fatalf("List() on second server error = %v", err)
	}
	if len(lr.Tunnels) != 2 {
		t.Errorf("List() on second server = %d, want 2", len(lr.Tunnels))
	}
}

func TestDaemonConcurrentClients(t *testing.T) {
	_, client := setupTestServer(t)

	const numWorkers = 10
	var wg sync.WaitGroup
	errs := make(chan error, numWorkers*3)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			id := fmt.Sprintf("t%d", idx)
			name := fmt.Sprintf("tunnel-%d", idx)

			// Add
			if err := client.Add(AddRequest{
				ID: id, Name: name, Type: "local",
				SSHHost: "bastion", LocalHost: "127.0.0.1",
				LocalPort: 8080 + idx, RemoteHost: "db", RemotePort: 5432,
			}); err != nil {
				errs <- fmt.Errorf("Add(%s): %w", id, err)
				return
			}

			// List
			if _, err := client.List(ListRequest{}); err != nil {
				errs <- fmt.Errorf("List() from worker %d: %w", idx, err)
				return
			}

			// Status
			if _, err := client.Status(id); err != nil {
				errs <- fmt.Errorf("Status(%s): %w", id, err)
				return
			}

			// Remove
			if err := client.Remove(id); err != nil {
				errs <- fmt.Errorf("Remove(%s): %w", id, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}

	// Verify clean state
	lr, err := client.List(ListRequest{})
	if err != nil {
		t.Fatalf("final List() error = %v", err)
	}
	if len(lr.Tunnels) != 0 {
		t.Errorf("final List() = %d tunnels, want 0", len(lr.Tunnels))
	}
}

// waitForServer polls until the client can reach the server.
func waitForServer(t *testing.T, client *Client) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("server did not become reachable within 10 seconds")
		case <-ticker.C:
			if client.IsRunning() {
				return
			}
		}
	}
}
