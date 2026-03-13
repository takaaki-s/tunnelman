package daemon

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReconnectExponentialBackoff(t *testing.T) {
	rm := NewReconnectManager("exponential", 100*time.Millisecond, 800*time.Millisecond, 5)

	// Verify delay calculation: 100, 200, 400, 800, 800 (capped)
	expected := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		800 * time.Millisecond, // capped at max
	}

	for i, want := range expected {
		got := rm.calcDelay(i)
		if got != want {
			t.Errorf("attempt %d: got %v, want %v", i, got, want)
		}
	}
}

func TestReconnectFixedBackoff(t *testing.T) {
	rm := NewReconnectManager("fixed", 200*time.Millisecond, 5*time.Second, 3)

	for i := 0; i < 5; i++ {
		got := rm.calcDelay(i)
		if got != 200*time.Millisecond {
			t.Errorf("attempt %d: got %v, want %v", i, got, 200*time.Millisecond)
		}
	}
}

func TestReconnectScheduleAndExecute(t *testing.T) {
	rm := NewReconnectManager("fixed", 50*time.Millisecond, 1*time.Second, 3)

	var reconnectCount atomic.Int32
	rm.SetOnReconnect(func(tunnelID string) error {
		reconnectCount.Add(1)
		return nil // success: stop reconnecting
	})

	rm.Schedule("test-tunnel")

	// Wait for the first reconnect attempt
	time.Sleep(150 * time.Millisecond)

	if reconnectCount.Load() < 1 {
		t.Error("expected at least one reconnect attempt")
	}

	// After successful reconnect, tunnel should no longer be reconnecting
	time.Sleep(100 * time.Millisecond)
	if rm.IsReconnecting("test-tunnel") {
		t.Error("expected tunnel to stop reconnecting after success")
	}
}

func TestReconnectMaxRetries(t *testing.T) {
	maxRetries := 3
	rm := NewReconnectManager("fixed", 30*time.Millisecond, 1*time.Second, maxRetries)

	var attempts atomic.Int32
	var mu sync.Mutex
	var exhaustedIDs []string

	rm.SetOnReconnect(func(tunnelID string) error {
		attempts.Add(1)
		return errReconnectFailed // always fail
	})
	rm.SetOnExhausted(func(tunnelID string) {
		mu.Lock()
		exhaustedIDs = append(exhaustedIDs, tunnelID)
		mu.Unlock()
	})

	rm.Schedule("test-tunnel")

	// Wait for all retries to complete: maxRetries * delay + buffer
	time.Sleep(time.Duration(maxRetries+2) * 50 * time.Millisecond)

	if int(attempts.Load()) < maxRetries {
		t.Errorf("expected at least %d attempts, got %d", maxRetries, attempts.Load())
	}

	mu.Lock()
	if len(exhaustedIDs) == 0 {
		t.Error("expected onExhausted callback to have been called")
	}
	mu.Unlock()

	if rm.IsReconnecting("test-tunnel") {
		t.Error("expected tunnel to stop reconnecting after max retries")
	}
}

func TestReconnectCancel(t *testing.T) {
	rm := NewReconnectManager("fixed", 200*time.Millisecond, 1*time.Second, 10)

	var attempts atomic.Int32
	rm.SetOnReconnect(func(tunnelID string) error {
		attempts.Add(1)
		return errReconnectFailed
	})

	rm.Schedule("test-tunnel")
	time.Sleep(50 * time.Millisecond) // let it schedule but not yet fire

	rm.Cancel("test-tunnel")

	before := attempts.Load()
	time.Sleep(300 * time.Millisecond) // wait past the scheduled attempt

	if attempts.Load() > before {
		t.Error("expected no more reconnect attempts after cancel")
	}
	if rm.IsReconnecting("test-tunnel") {
		t.Error("expected tunnel to not be reconnecting after cancel")
	}
}

func TestReconnectCancelAll(t *testing.T) {
	rm := NewReconnectManager("fixed", 200*time.Millisecond, 1*time.Second, 10)

	rm.SetOnReconnect(func(tunnelID string) error {
		return errReconnectFailed
	})

	rm.Schedule("t1")
	rm.Schedule("t2")

	rm.CancelAll()

	if rm.IsReconnecting("t1") {
		t.Error("expected t1 to not be reconnecting after CancelAll")
	}
	if rm.IsReconnecting("t2") {
		t.Error("expected t2 to not be reconnecting after CancelAll")
	}
}

func TestReconnectCancelUnknown(t *testing.T) {
	rm := NewReconnectManager("fixed", 100*time.Millisecond, 1*time.Second, 3)
	// Should not panic
	rm.Cancel("nonexistent")
}
