package daemon

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"
)

// healthState tracks the health check state for a single tunnel.
type healthState struct {
	localHost        string
	localPort        int
	consecutiveFails int
	healthy          bool
	cancel           context.CancelFunc
}

// HealthChecker periodically checks TCP port reachability for running tunnels.
type HealthChecker struct {
	interval    time.Duration
	timeout     time.Duration
	maxFailures int
	tunnels     map[string]*healthState
	mu          sync.Mutex
	onUnhealthy func(tunnelID string)
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker(interval, timeout time.Duration, maxFailures int) *HealthChecker {
	return &HealthChecker{
		interval:    interval,
		timeout:     timeout,
		maxFailures: maxFailures,
		tunnels:     make(map[string]*healthState),
	}
}

// SetOnUnhealthy sets a callback invoked when a tunnel becomes unhealthy.
func (hc *HealthChecker) SetOnUnhealthy(fn func(tunnelID string)) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.onUnhealthy = fn
}

// Start begins health checking for the given tunnel.
func (hc *HealthChecker) Start(tunnelID, localHost string, localPort int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Stop existing check if any
	if existing, ok := hc.tunnels[tunnelID]; ok {
		existing.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	hs := &healthState{
		localHost: localHost,
		localPort: localPort,
		healthy:   true,
		cancel:    cancel,
	}
	hc.tunnels[tunnelID] = hs

	go hc.checkLoop(ctx, tunnelID)
}

// Stop stops health checking for the given tunnel.
func (hc *HealthChecker) Stop(tunnelID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hs, ok := hc.tunnels[tunnelID]; ok {
		hs.cancel()
		delete(hc.tunnels, tunnelID)
	}
}

// StopAll stops all health checks.
func (hc *HealthChecker) StopAll() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	for id, hs := range hc.tunnels {
		hs.cancel()
		delete(hc.tunnels, id)
	}
}

// IsHealthy returns whether the given tunnel is currently healthy.
// Returns false for unknown tunnels.
func (hc *HealthChecker) IsHealthy(tunnelID string) bool {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hs, ok := hc.tunnels[tunnelID]
	if !ok {
		return false
	}
	return hs.healthy
}

// checkLoop runs periodic health checks. The first check fires after one interval
// (not immediately), so there is a startup delay equal to the configured interval.
func (hc *HealthChecker) checkLoop(ctx context.Context, tunnelID string) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.doCheck(tunnelID)
		}
	}
}

func (hc *HealthChecker) doCheck(tunnelID string) {
	hc.mu.Lock()
	hs, ok := hc.tunnels[tunnelID]
	if !ok {
		hc.mu.Unlock()
		return
	}
	addr := net.JoinHostPort(hs.localHost, strconv.Itoa(hs.localPort))
	timeout := hc.timeout
	maxFail := hc.maxFailures
	hc.mu.Unlock()

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		conn.Close()
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Re-check: tunnel might have been stopped during dial
	hs, ok = hc.tunnels[tunnelID]
	if !ok {
		return
	}

	if err != nil {
		hs.consecutiveFails++
		// Callback fires once per healthy→unhealthy transition.
		// After recovery (healthy=true), a new failure cycle can trigger it again.
		if hs.consecutiveFails >= maxFail && hs.healthy {
			hs.healthy = false
			if hc.onUnhealthy != nil {
				fn := hc.onUnhealthy
				// Run in a new goroutine to avoid blocking the health check ticker
				// while the callback (which may acquire Server.mu) executes.
				go fn(tunnelID)
			}
		}
	} else {
		hs.consecutiveFails = 0
		hs.healthy = true
	}
}
