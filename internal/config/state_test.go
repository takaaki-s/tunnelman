package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateManagerLoadSave(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatalf("NewStateManager() error = %v", err)
	}

	now := time.Now()
	sm.SetTunnel("t1", &TunnelState{
		PID:       1234,
		StartedAt: now,
		Status:    "running",
	})

	if err := sm.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create a new StateManager and load
	sm2, err := NewStateManager(dir)
	if err != nil {
		t.Fatalf("NewStateManager() error = %v", err)
	}
	if err := sm2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ts := sm2.GetTunnel("t1")
	if ts == nil {
		t.Fatal("GetTunnel(t1) returned nil")
	}
	if ts.PID != 1234 || ts.Status != "running" {
		t.Errorf("TunnelState = %+v", ts)
	}
}

func TestStateManagerAddRemoveTunnel(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatalf("NewStateManager() error = %v", err)
	}

	sm.SetTunnel("t1", &TunnelState{PID: 100, Status: "running"})
	sm.SetTunnel("t2", &TunnelState{PID: 200, Status: "running"})

	all := sm.GetAllTunnels()
	if len(all) != 2 {
		t.Errorf("GetAllTunnels() count = %d, want 2", len(all))
	}

	sm.RemoveTunnel("t1")
	if sm.GetTunnel("t1") != nil {
		t.Error("GetTunnel(t1) should be nil after remove")
	}

	all = sm.GetAllTunnels()
	if len(all) != 1 {
		t.Errorf("GetAllTunnels() count = %d, want 1", len(all))
	}
}

func TestStateManagerCleanStalePIDs(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatalf("NewStateManager() error = %v", err)
	}

	// Use a PID that almost certainly doesn't exist
	sm.SetTunnel("t1", &TunnelState{PID: 999999999, Status: "running"})
	// Use current process PID (should be running)
	sm.SetTunnel("t2", &TunnelState{PID: os.Getpid(), Status: "running"})

	cleaned := sm.CleanStalePIDs()
	if cleaned != 1 {
		t.Errorf("CleanStalePIDs() = %d, want 1", cleaned)
	}

	if sm.GetTunnel("t1") != nil {
		t.Error("stale tunnel t1 should be removed")
	}
	if sm.GetTunnel("t2") == nil {
		t.Error("running tunnel t2 should remain")
	}
}

func TestStateManagerDaemonInfo(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatalf("NewStateManager() error = %v", err)
	}

	now := time.Now()
	sm.SetDaemon(&DaemonState{
		PID:       5678,
		StartedAt: now,
	})

	if err := sm.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	sm2, err := NewStateManager(dir)
	if err != nil {
		t.Fatalf("NewStateManager() error = %v", err)
	}
	if err := sm2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ds := sm2.GetDaemon()
	if ds == nil {
		t.Fatal("GetDaemon() returned nil")
	}
	if ds.PID != 5678 {
		t.Errorf("Daemon PID = %d, want 5678", ds.PID)
	}
}

func TestStateEmptyFile(t *testing.T) {
	dir := t.TempDir()
	sm, err := NewStateManager(dir)
	if err != nil {
		t.Fatalf("NewStateManager() error = %v", err)
	}

	// Load without any file existing
	if err := sm.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	all := sm.GetAllTunnels()
	if len(all) != 0 {
		t.Errorf("GetAllTunnels() count = %d, want 0", len(all))
	}

	// Also test with empty file
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(statePath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	sm2, err := NewStateManager(dir)
	if err != nil {
		t.Fatalf("NewStateManager() error = %v", err)
	}
	if err := sm2.Load(); err != nil {
		t.Fatalf("Load() with empty file error = %v", err)
	}

	all = sm2.GetAllTunnels()
	if len(all) != 0 {
		t.Errorf("GetAllTunnels() count = %d, want 0", len(all))
	}
}
