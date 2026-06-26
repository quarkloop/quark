package handler

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quarkloop/quark/server/internal/health"
)

func TestHealthHandler_Live(t *testing.T) {
	app := fiber.New()
	h := NewHealthHandler(health.New(nil))
	h.Register(app)

	req := httptest.NewRequest("GET", "/health/live", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	assert.Equal(t, "UP", m["status"])
}

func TestHealthHandler_Ready_NATSDown(t *testing.T) {
	app := fiber.New()
	// nil NATS connection simulates a not-yet-connected state
	h := NewHealthHandler(health.New(nil))
	h.Register(app)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	// Endpoint should return 200 with status=DOWN (we don't fail the request,
	// we just report the status in the JSON body — same as the Java server).
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	// NATS is down (nil conn) → status should be DOWN
	assert.Equal(t, "DOWN", m["status"])
}
