package daemon

import (
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHealthCheckerPortReachable(t *testing.T) {
	// Start a local TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)

	hc := NewHealthChecker(100*time.Millisecond, 50*time.Millisecond, 3)

	var called atomic.Int32
	hc.SetOnUnhealthy(func(tunnelID string) {
		called.Add(1)
	})

	hc.Start(t.Name(), "127.0.0.1", addr.Port)
	defer hc.Stop(t.Name())

	// Wait for a few check cycles
	time.Sleep(350 * time.Millisecond)

	if !hc.IsHealthy(t.Name()) {
		t.Error("expected tunnel to be healthy")
	}
	if called.Load() != 0 {
		t.Errorf("onUnhealthy should not have been called, got %d", called.Load())
	}
}

func TestHealthCheckerConsecutiveFailures(t *testing.T) {
	// Use a port that is not listening
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // close immediately so the port is unreachable

	maxFailures := 3
	hc := NewHealthChecker(50*time.Millisecond, 20*time.Millisecond, maxFailures)

	var mu sync.Mutex
	var unhealthyIDs []string
	hc.SetOnUnhealthy(func(tunnelID string) {
		mu.Lock()
		unhealthyIDs = append(unhealthyIDs, tunnelID)
		mu.Unlock()
	})

	hc.Start("test-tunnel", "127.0.0.1", port)
	defer hc.Stop("test-tunnel")

	// Wait for enough check cycles: maxFailures * interval + buffer
	time.Sleep(time.Duration(maxFailures+2) * 50 * time.Millisecond)

	if hc.IsHealthy("test-tunnel") {
		t.Error("expected tunnel to be unhealthy after consecutive failures")
	}

	mu.Lock()
	if len(unhealthyIDs) == 0 {
		t.Error("expected onUnhealthy callback to have been called")
	}
	mu.Unlock()
}

func TestHealthCheckerRecovery(t *testing.T) {
	maxFailures := 2
	hc := NewHealthChecker(50*time.Millisecond, 20*time.Millisecond, maxFailures)

	var unhealthyCount atomic.Int32
	hc.SetOnUnhealthy(func(tunnelID string) {
		unhealthyCount.Add(1)
	})

	// Start with closed port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	hc.Start("test-tunnel", "127.0.0.1", port)
	defer hc.Stop("test-tunnel")

	// Wait until unhealthy
	time.Sleep(time.Duration(maxFailures+2) * 50 * time.Millisecond)

	if hc.IsHealthy("test-tunnel") {
		t.Error("expected tunnel to be unhealthy")
	}

	// Re-open the listener on the same port
	ln2, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Skipf("could not re-listen on port %d: %v", port, err)
	}
	defer ln2.Close()
	go func() {
		for {
			conn, err := ln2.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// Wait for recovery
	time.Sleep(200 * time.Millisecond)

	if !hc.IsHealthy("test-tunnel") {
		t.Error("expected tunnel to recover to healthy")
	}
}

func TestHealthCheckerStopAll(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	hc := NewHealthChecker(50*time.Millisecond, 20*time.Millisecond, 3)

	hc.Start("t1", "127.0.0.1", addr.Port)
	hc.Start("t2", "127.0.0.1", addr.Port)

	hc.StopAll()

	// After StopAll, IsHealthy should return false (entry removed)
	if hc.IsHealthy("t1") {
		t.Error("expected t1 to not be healthy after StopAll")
	}
	if hc.IsHealthy("t2") {
		t.Error("expected t2 to not be healthy after StopAll")
	}
}

func TestHealthCheckDisabled(t *testing.T) {
	// When HealthChecker is nil (config disabled), nothing should happen.
	// This tests the pattern used in server.go where healthChecker is only
	// created when config.HealthCheck.Enabled is true.
	var hc *HealthChecker // nil

	// All nil-guard patterns used in server.go should work without panic
	if hc != nil {
		hc.Start("t1", "127.0.0.1", 8080)
	}
	if hc != nil {
		hc.Stop("t1")
	}
	if hc != nil {
		hc.StopAll()
	}
	// If we reach here without panic, the test passes.
}

func TestHealthCheckerUnknownTunnel(t *testing.T) {
	hc := NewHealthChecker(50*time.Millisecond, 20*time.Millisecond, 3)
	if hc.IsHealthy("nonexistent") {
		t.Error("expected false for unknown tunnel")
	}
	// Stop on unknown tunnel should not panic
	hc.Stop("nonexistent")
}
