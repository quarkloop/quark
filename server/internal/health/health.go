// Package health implements the liveness and readiness endpoints.
//
// Liveness: GET /health/live — always returns 200 if the process is
// running. Used by orchestrators (Kubernetes, systemd) to decide
// whether to restart the container.
//
// Readiness: GET /health/ready — returns 200 only when the server is
// ready to serve traffic (NATS connected, Catalog reachable). Used by
// load balancers to decide whether to route requests.
package health

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Checker is the health-check service. It's safe for concurrent use.
type Checker struct {
	nc *nats.Conn

	mu      sync.RWMutex
	natsOK  bool
	catOK   bool
}

// New constructs a Checker. The NATS connection is used to ping the
// Catalog on readiness checks.
func New(nc *nats.Conn) *Checker {
	return &Checker{nc: nc}
}

// SetNATSOK is called by the main goroutine when NATS connects.
func (c *Checker) SetNATSOK(ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.natsOK = ok
}

// SetCatalogOK is called by the main goroutine after the first
// successful Catalog request.
func (c *Checker) SetCatalogOK(ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.catOK = ok
}

// Check is the body returned by both endpoints.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Response is the JSON envelope returned by both endpoints.
type Response struct {
	Status string  `json:"status"`
	Checks []Check `json:"checks"`
}

// Live returns the liveness response. Always 200/UP — if the process
// is alive enough to answer, it's live.
func (c *Checker) Live() Response {
	return Response{
		Status: "UP",
		Checks: []Check{{Name: "process", Status: "UP"}},
	}
}

// Ready returns the readiness response. UP only when NATS is connected
// AND the Catalog is reachable.
func (c *Checker) Ready(ctx context.Context) Response {
	// Ping the Catalog by sending a no-op request to catalog.registry.list.
	// If it responds within 2s, we're ready.
	catalogOK := false
	if c.nc != nil && c.nc.Status() == nats.CONNECTED {
		_, err := c.nc.RequestWithContext(ctx, "catalog.registry.list", []byte("{}"))
		catalogOK = err == nil
	}
	c.SetCatalogOK(catalogOK)

	c.mu.RLock()
	natsOK := c.natsOK
	c.mu.RUnlock()

	status := "UP"
	if !natsOK || !catalogOK {
		status = "DOWN"
	}
	natsStatus := "UP"
	if !natsOK {
		natsStatus = "DOWN"
	}
	catStatus := "UP"
	if !catalogOK {
		catStatus = "DOWN"
	}
	return Response{
		Status: status,
		Checks: []Check{
			{Name: "nats", Status: natsStatus},
			{Name: "catalog", Status: catStatus},
		},
	}
}

// MarshalJSON helper for Response — kept here so the handler doesn't
// need to import encoding/json.
func (r Response) MarshalJSON() ([]byte, error) {
	type alias Response
	return json.Marshal(alias(r))
}

// pingTimeout is the readiness check's Catalog request timeout.
const pingTimeout = 2 * time.Second
