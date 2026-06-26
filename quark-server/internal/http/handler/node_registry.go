// Package handler — node package registry endpoints (proxied to Catalog).
//
// These endpoints proxy requests to the Catalog's registry.node.*
// NATS subjects. The Go server doesn't decode the response — it
// returns the raw JSON byte-for-byte. This keeps the wire format
// flexible as the Catalog's package types evolve.
package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nats-io/nats.go"
)

// natsTimeout is the per-request NATS timeout for registry.node.* calls.
const natsTimeout = 5 * time.Second

// NodeRegistryHandler handles /api/v1/registry/nodes endpoints.
type NodeRegistryHandler struct {
	nc *nats.Conn
}

// NewNodeRegistryHandler constructs a NodeRegistryHandler.
func NewNodeRegistryHandler(nc *nats.Conn) *NodeRegistryHandler {
	return &NodeRegistryHandler{nc: nc}
}

// Register wires the node package registry routes.
func (h *NodeRegistryHandler) Register(r fiber.Router) {
	r.Get("/", h.list)
	r.Post("/", h.push)
	r.Post("/info", h.info)
	r.Post("/pull", h.pull)
	r.Get("/search", h.search)
}

// list proxies GET /api/v1/registry/nodes to NATS registry.node.list.
func (h *NodeRegistryHandler) list(c *fiber.Ctx) error {
	return h.proxyJSON(c, "registry.node.list", fiber.Map{})
}

// info proxies POST /api/v1/registry/nodes/info to NATS registry.node.info.
func (h *NodeRegistryHandler) info(c *fiber.Ctx) error {
	var body map[string]string
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	uri := body["uri"]
	if uri == "" {
		return c.Status(400).JSON(fiber.Map{"error": "uri is required"})
	}
	resp, err := h.natsRequest(c.Context(), "registry.node.info", fiber.Map{"uri": uri})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if isErrorResponse(resp) {
		return c.Status(404).Type("json").Send(resp)
	}
	return c.Type("json").Send(resp)
}

// pull proxies POST /api/v1/registry/nodes/pull to NATS registry.node.pull.
func (h *NodeRegistryHandler) pull(c *fiber.Ctx) error {
	var body map[string]string
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	uri := body["uri"]
	if uri == "" {
		return c.Status(400).JSON(fiber.Map{"error": "uri is required"})
	}
	resp, err := h.natsRequest(c.Context(), "registry.node.pull", fiber.Map{"uri": uri})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if isErrorResponse(resp) {
		return c.Status(404).Type("json").Send(resp)
	}
	return c.Type("json").Send(resp)
}

// search proxies GET /api/v1/registry/nodes/search?keyword=... to NATS registry.node.search.
func (h *NodeRegistryHandler) search(c *fiber.Ctx) error {
	keyword := c.Query("keyword")
	return h.proxyJSON(c, "registry.node.search", fiber.Map{"keyword": keyword})
}

// push proxies POST /api/v1/registry/nodes to NATS registry.node.push.
// The body may contain a base64-encoded "content" field which the
// handler decodes before forwarding.
func (h *NodeRegistryHandler) push(c *fiber.Ctx) error {
	var body map[string]any
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	// Decode base64 content if present (CLI sends base64-encoded zip bytes)
	if content, ok := body["content"].(string); ok {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "content is not valid base64: " + err.Error()})
		}
		body["content"] = decoded
	}
	resp, err := h.natsRequestWithBody(c.Context(), "registry.node.push", body)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Type("json").Send(resp)
}

// proxyJSON sends a JSON request to subject and returns the raw reply
// as the response body (also JSON). Used for list/search/etc.
func (h *NodeRegistryHandler) proxyJSON(c *fiber.Ctx, subject string, req any) error {
	resp, err := h.natsRequest(c.Context(), subject, req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Type("json").Send(resp)
}

// natsRequest sends req as JSON to subject, waits up to natsTimeout,
// returns the raw reply bytes.
func (h *NodeRegistryHandler) natsRequest(ctx context.Context, subject string, req any) ([]byte, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	reply, err := h.nc.RequestWithContext(ctx, subject, payload)
	if err != nil {
		return nil, fmt.Errorf("nats request %s: %w", subject, err)
	}
	return reply.Data, nil
}

// natsRequestWithBody is the same as natsRequest but uses json.Encoder
// to allow []byte fields to be encoded as base64 (Go's json.Marshal
// does this automatically for []byte).
func (h *NodeRegistryHandler) natsRequestWithBody(ctx context.Context, subject string, req any) ([]byte, error) {
	return h.natsRequest(ctx, subject, req)
}

// isErrorResponse returns true if the JSON response body represents a
// Catalog error envelope ({ "success": false, "error": "..." }).
func isErrorResponse(body []byte) bool {
	var env struct {
		Error   string `json:"error"`
		Success bool   `json:"success"`
	}
	if json.Unmarshal(body, &env) != nil {
		return false
	}
	return !env.Success && env.Error != ""
}
