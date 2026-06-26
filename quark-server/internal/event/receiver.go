// Package event subscribes to quark.data.event.> on NATS and persists
// every lifecycle event forwarded from a data-plane process to the
// Catalog via the EventStore.
//
// Same flow as the Java ControlPlaneEventReceiver:
//   - Subscribe to quark.data.event.> (wildcard) on startup.
//   - For each message, decode the JSON NodeEvent and call
//     EventStore.Append(ctx, event).
//
// Only the control plane subscribes to this subject — data planes
// publish events but don't subscribe to their own forwards.
package event

import (
        "context"
        "encoding/json"
        "fmt"
        "sync"

        "github.com/nats-io/nats.go"
        "go.uber.org/zap"

        "github.com/quarkloop/quark/server/internal/dataplane"
        "github.com/quarkloop/quark/server/internal/domain"
        "github.com/quarkloop/quark/server/internal/store"
)

// Receiver subscribes to quark.data.event.> and persists events.
//
// Use New() to construct; Start() to begin receiving; Stop() to
// drain the subscription on graceful shutdown.
type Receiver struct {
        log       *zap.Logger
        nc        *nats.Conn
        eventStore store.EventStore

        mu   sync.Mutex
        sub  *nats.Subscription
}

// New constructs a Receiver.
func New(log *zap.Logger, nc *nats.Conn, eventStore store.EventStore) *Receiver {
        return &Receiver{log: log, nc: nc, eventStore: eventStore}
}

// Start subscribes to quark.data.event.> and processes messages
// concurrently on the NATS dispatcher's goroutine.
func (r *Receiver) Start() error {
        sub, err := r.nc.Subscribe(dataplane.EventWildcard, r.handleEvent)
        if err != nil {
                return fmt.Errorf("subscribe to %s: %w", dataplane.EventWildcard, err)
        }
        r.mu.Lock()
        r.sub = sub
        r.mu.Unlock()
        r.log.Info("event receiver subscribed", zap.String("subject", dataplane.EventWildcard))
        return nil
}

// Stop drains the subscription. Safe to call multiple times.
func (r *Receiver) Stop() {
        r.mu.Lock()
        defer r.mu.Unlock()
        if r.sub == nil {
                return
        }
        // Drain waits for in-flight messages to be processed.
        _ = r.sub.Drain()
        r.sub = nil
        r.log.Info("event receiver stopped")
}

// handleEvent is the NATS message callback.
func (r *Receiver) handleEvent(msg *nats.Msg) {
        var ev domain.NodeEvent
        if err := json.Unmarshal(msg.Data, &ev); err != nil {
                r.log.Error("decode event failed",
                        zap.String("subject", msg.Subject), zap.Error(err))
                return
        }
        // Normalize the timestamp to a string before persisting. The Java
        // data plane serializes Instant as epoch-millis (number); the
        // Catalog stores it as an RFC3339 string.
        ev.Timestamp = ev.TimestampString()
        // Use a background context — NATS callbacks have no request
        // context. The Catalog's own 5s timeout bounds the latency.
        if err := r.eventStore.AppendEvent(context.Background(), &ev); err != nil {
                r.log.Error("persist event failed",
                        zap.String("subject", msg.Subject),
                        zap.String("namespace", ev.Namespace),
                        zap.String("system", ev.SystemName),
                        zap.String("node", ev.NodeName),
                        zap.String("kind", ev.Kind),
                        zap.Error(err))
                return
        }
        r.log.Debug("persisted event",
                zap.String("namespace", ev.Namespace),
                zap.String("system", ev.SystemName),
                zap.String("node", ev.NodeName),
                zap.String("kind", ev.Kind))
}
