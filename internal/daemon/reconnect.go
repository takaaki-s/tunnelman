package daemon

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// errReconnectFailed is returned by onReconnect to signal that reconnection failed.
var errReconnectFailed = errors.New("reconnect failed")

// reconnectState tracks the reconnection state for a single tunnel.
type reconnectState struct {
	attempt int
	cancel  context.CancelFunc
}

// ReconnectManager handles automatic reconnection with backoff.
type ReconnectManager struct {
	strategy     string // "exponential" or "fixed"
	initialDelay time.Duration
	maxDelay     time.Duration
	maxRetries   int
	reconnecting map[string]*reconnectState
	mu           sync.Mutex
	onReconnect  func(tunnelID string) error
	onExhausted  func(tunnelID string)
}

// NewReconnectManager creates a new ReconnectManager.
// If maxRetries < 1, it defaults to 3 to prevent silent immediate exhaustion.
func NewReconnectManager(strategy string, initialDelay, maxDelay time.Duration, maxRetries int) *ReconnectManager {
	if maxRetries < 1 {
		maxRetries = 3
	}
	return &ReconnectManager{
		strategy:     strategy,
		initialDelay: initialDelay,
		maxDelay:     maxDelay,
		maxRetries:   maxRetries,
		reconnecting: make(map[string]*reconnectState),
	}
}

// SetOnReconnect sets the callback that performs the actual reconnection.
// Must be called before Schedule. Changes take effect from the next retry attempt.
// Return nil on success, errReconnectFailed (or any error) on failure.
func (rm *ReconnectManager) SetOnReconnect(fn func(tunnelID string) error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.onReconnect = fn
}

// SetOnExhausted sets a callback invoked when max retries are exhausted.
// Must be called before Schedule. Changes take effect from the next retry attempt.
func (rm *ReconnectManager) SetOnExhausted(fn func(tunnelID string)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.onExhausted = fn
}

// Schedule begins the reconnection loop for the given tunnel.
func (rm *ReconnectManager) Schedule(tunnelID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Cancel any existing reconnect for this tunnel
	if existing, ok := rm.reconnecting[tunnelID]; ok {
		existing.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	rs := &reconnectState{
		attempt: 0,
		cancel:  cancel,
	}
	rm.reconnecting[tunnelID] = rs

	go rm.reconnectLoop(ctx, tunnelID)
}

// Cancel stops reconnection attempts for the given tunnel.
func (rm *ReconnectManager) Cancel(tunnelID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rs, ok := rm.reconnecting[tunnelID]; ok {
		rs.cancel()
		delete(rm.reconnecting, tunnelID)
	}
}

// CancelAll stops all reconnection attempts.
func (rm *ReconnectManager) CancelAll() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for id, rs := range rm.reconnecting {
		rs.cancel()
		delete(rm.reconnecting, id)
	}
}

// IsReconnecting returns whether the given tunnel is currently reconnecting.
func (rm *ReconnectManager) IsReconnecting(tunnelID string) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	_, ok := rm.reconnecting[tunnelID]
	return ok
}

// calcDelay computes the delay for the given attempt number.
func (rm *ReconnectManager) calcDelay(attempt int) time.Duration {
	if rm.strategy == "fixed" {
		return rm.initialDelay
	}
	// exponential backoff
	delay := rm.initialDelay
	for i := 0; i < attempt; i++ {
		// Check before multiplication to prevent int64 overflow on large delays.
		if delay > rm.maxDelay/2 {
			return rm.maxDelay
		}
		delay *= 2
		if delay > rm.maxDelay {
			return rm.maxDelay
		}
	}
	// Guard for the case where initialDelay > maxDelay (attempt=0, loop not entered).
	if delay > rm.maxDelay {
		return rm.maxDelay
	}
	return delay
}

func (rm *ReconnectManager) reconnectLoop(ctx context.Context, tunnelID string) {
	for {
		rm.mu.Lock()
		rs, ok := rm.reconnecting[tunnelID]
		if !ok {
			rm.mu.Unlock()
			return
		}
		attempt := rs.attempt
		if attempt >= rm.maxRetries {
			onExhausted := rm.onExhausted
			delete(rm.reconnecting, tunnelID)
			rm.mu.Unlock()
			if onExhausted != nil {
				onExhausted(tunnelID)
			}
			return
		}
		delay := rm.calcDelay(attempt)
		onReconnect := rm.onReconnect
		rm.mu.Unlock()

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		if onReconnect == nil {
			rm.mu.Lock()
			delete(rm.reconnecting, tunnelID)
			rm.mu.Unlock()
			return
		}

		slog.Info("attempting reconnect", "tunnel", tunnelID, "attempt", attempt+1)
		err := onReconnect(tunnelID)

		rm.mu.Lock()
		rs, ok = rm.reconnecting[tunnelID]
		if !ok {
			// Cancelled during reconnect
			rm.mu.Unlock()
			return
		}

		if err == nil {
			// Success
			delete(rm.reconnecting, tunnelID)
			rm.mu.Unlock()
			slog.Info("reconnect succeeded", "tunnel", tunnelID)
			return
		}

		rs.attempt++
		rm.mu.Unlock()
	}
}
