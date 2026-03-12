package daemon

import (
	"testing"
)

func TestClientIsRunning(t *testing.T) {
	_, client := setupTestServer(t)
	if !client.IsRunning() {
		t.Error("IsRunning() should return true for running server")
	}

	// Non-existent socket
	badClient := NewClient("/tmp/nonexistent-tunnelman-test.sock")
	if badClient.IsRunning() {
		t.Error("IsRunning() should return false for non-existent socket")
	}
}

func TestClientAddListDaemonStatus(t *testing.T) {
	_, client := setupTestServer(t)

	// Add
	err := client.Add(AddRequest{
		ID: "t1", Name: "web", Type: "local",
		SSHHost: "bastion", LocalHost: "127.0.0.1",
		LocalPort: 8080, RemoteHost: "db", RemotePort: 5432,
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// List
	lr, err := client.List(ListRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(lr.Tunnels) != 1 {
		t.Fatalf("List() count = %d, want 1", len(lr.Tunnels))
	}
	if lr.Tunnels[0].Name != "web" {
		t.Errorf("List()[0].Name = %q, want %q", lr.Tunnels[0].Name, "web")
	}

	// DaemonStatus
	ds, err := client.DaemonStatus()
	if err != nil {
		t.Fatalf("DaemonStatus() error = %v", err)
	}
	if ds.Tunnels != 1 {
		t.Errorf("DaemonStatus().Tunnels = %d, want 1", ds.Tunnels)
	}
}
