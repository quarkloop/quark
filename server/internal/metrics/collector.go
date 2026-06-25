// Package metrics subscribes to quark.data.heartbeat.> on NATS and
// aggregates per-namespace throughput/CPU/error rates over a sliding
// 2-second window.
//
// Same flow as the Java NamespaceMetricsCollector:
//   - On startup, subscribe to quark.data.heartbeat.> (wildcard).
//   - For each heartbeat message, decode the JSON map of namespace →
//     Snapshot and merge it into the local state.
//   - A ticker goroutine computes rates (delta / elapsed) every 2s.
//   - GetRates() returns the latest per-namespace rates for the REST API.
package metrics

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	"github.com/quarkloop/quark/server/internal/dataplane"
)

// Snapshot is the per-namespace metrics snapshot. Same shape as the
// Java NamespaceMetrics.Snapshot record (sent over NATS in the
// heartbeat payload).
type Snapshot struct {
	MessagesPublished int64  `json:"messagesPublished"`
	MessagesReceived  int64  `json:"messagesReceived"`
	Errors            int64  `json:"errors"`
	CPUTimeNanos      int64  `json:"cpuTimeNanos"`
}

// Rate is the per-namespace computed rate (deltas over the last interval).
type Rate struct {
	MessagesPublished int64
	MessagesReceived  int64
	Errors            int64
	CPUTimeNanos      int64

	PublishRate float64 // msgs/sec
	ReceiveRate float64 // msgs/sec
	ErrorRate   float64 // errs/sec
	CPUPercent  float64 // % of one CPU core
}

// Collector subscribes to NATS heartbeats and computes per-namespace rates.
type Collector struct {
	log *zap.Logger
	nc  *nats.Conn

	mu              sync.RWMutex
	remoteSnapshots map[string]Snapshot // namespace → latest snapshot
	latest          map[string]Snapshot
	previous        map[string]Snapshot
	rates           map[string]Rate
	lastTickTime    time.Time

	sub       *nats.Subscription
	ticker    *time.Ticker
	tickerDone chan struct{}
}

const interval = 2 * time.Second

// New constructs a Collector.
func New(log *zap.Logger, nc *nats.Conn) *Collector {
	return &Collector{
		log:             log,
		nc:              nc,
		remoteSnapshots: make(map[string]Snapshot),
		latest:          make(map[string]Snapshot),
		previous:        make(map[string]Snapshot),
		rates:           make(map[string]Rate),
	}
}

// Start subscribes to heartbeats and launches the ticker goroutine.
func (c *Collector) Start() error {
	sub, err := c.nc.Subscribe(dataplane.HeartbeatWildcard, c.handleHeartbeat)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.sub = sub
	c.mu.Unlock()

	c.ticker = time.NewTicker(interval)
	c.tickerDone = make(chan struct{})
	go c.loop()

	c.log.Info("metrics collector started",
		zap.String("subject", dataplane.HeartbeatWildcard),
		zap.Duration("interval", interval))
	return nil
}

// Stop drains the subscription and stops the ticker.
func (c *Collector) Stop() {
	c.mu.Lock()
	if c.sub != nil {
		_ = c.sub.Drain()
		c.sub = nil
	}
	if c.ticker != nil {
		c.ticker.Stop()
	}
	if c.tickerDone != nil {
		// Signal the loop to exit. The loop's recv on this channel
		// returns immediately.
		close(c.tickerDone)
		c.tickerDone = nil
	}
	c.mu.Unlock()
	c.log.Info("metrics collector stopped")
}

// handleHeartbeat is the NATS callback. The payload is a JSON map of
// namespace → Snapshot.
func (c *Collector) handleHeartbeat(msg *nats.Msg) {
	var remote map[string]Snapshot
	if err := json.Unmarshal(msg.Data, &remote); err != nil {
		c.log.Warn("decode heartbeat failed",
			zap.String("subject", msg.Subject), zap.Error(err))
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for ns, snap := range remote {
		c.remoteSnapshots[ns] = snap
	}
}

// loop is the ticker goroutine. Calls takeSnapshot() every interval.
func (c *Collector) loop() {
	for {
		select {
		case <-c.ticker.C:
			c.takeSnapshot()
		case <-c.tickerDone:
			return
		}
	}
}

// takeSnapshot computes rates from the current vs previous snapshots.
func (c *Collector) takeSnapshot() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	current := make(map[string]Snapshot, len(c.remoteSnapshots))
	for ns, snap := range c.remoteSnapshots {
		current[ns] = snap
	}

	elapsedMs := c.lastTickTime.IsZero() || now.Sub(c.lastTickTime) <= 0
	elapsed := interval
	if !elapsedMs {
		elapsed = now.Sub(c.lastTickTime)
	}
	elapsedSec := elapsed.Seconds()
	windowNanos := float64(elapsed.Milliseconds()) * 1_000_000.0

	newRates := make(map[string]Rate, len(current))
	for ns, cur := range current {
		prev, hasPrev := c.previous[ns]
		var prevPub, prevRcv, prevErr, prevCPU int64
		if hasPrev {
			prevPub, prevRcv, prevErr, prevCPU = prev.MessagesPublished, prev.MessagesReceived, prev.Errors, prev.CPUTimeNanos
		}
		pubRate := 0.0
		rcvRate := 0.0
		errRate := 0.0
		cpuPercent := 0.0
		if elapsedSec > 0 {
			pubRate = float64(cur.MessagesPublished-prevPub) / elapsedSec
			rcvRate = float64(cur.MessagesReceived-prevRcv) / elapsedSec
			errRate = float64(cur.Errors-prevErr) / elapsedSec
		}
		if windowNanos > 0 {
			cpuPercent = float64(cur.CPUTimeNanos-prevCPU) / windowNanos * 100.0
		}
		newRates[ns] = Rate{
			MessagesPublished: cur.MessagesPublished,
			MessagesReceived:  cur.MessagesReceived,
			Errors:            cur.Errors,
			CPUTimeNanos:      cur.CPUTimeNanos,
			PublishRate:       pubRate,
			ReceiveRate:       rcvRate,
			ErrorRate:         errRate,
			CPUPercent:        cpuPercent,
		}
	}

	// Replace previous with the old latest; replace latest with current.
	c.previous = c.latest
	c.latest = current
	// Drop rates for namespaces that no longer have a snapshot.
	c.rates = newRates
	c.lastTickTime = now
}

// GetRate returns the per-namespace rate, or nil if unknown.
func (c *Collector) GetRate(namespace string) *Rate {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if r, ok := c.rates[namespace]; ok {
		return &r
	}
	return nil
}

// AllRates returns a copy of the current per-namespace rate map.
// Safe for callers to mutate the returned map.
func (c *Collector) AllRates() map[string]Rate {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]Rate, len(c.rates))
	for k, v := range c.rates {
		out[k] = v
	}
	return out
}

// SnapshotForTesting allows tests to inject snapshots directly without
// going through NATS. Not used in production code.
func (c *Collector) SnapshotForTesting(ctx context.Context, ns string, snap Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remoteSnapshots[ns] = snap
}
