package server

import (
	"log/slog"
	"net"

	"github.com/gofiber/fiber/v2"
	"github.com/quarkloop/supervisor/pkg/api"
)

// handleHealth serves GET /v1/health — liveness probe.
func (s *Server) handleHealth(c *fiber.Ctx) error {
	return c.JSON(api.HealthResponse{Status: "ok"})
}

func writeJSON(c *fiber.Ctx, status int, body any) error {
	return c.Status(status).JSON(body)
}

func writeError(c *fiber.Ctx, status int, msg string) error {
	return writeJSON(c, status, api.ErrorResponse{Error: msg})
}

// reservePort finds an available TCP port on the loopback interface.
func reservePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		slog.Error("failed to close temp listener", "error", err)
	}
	return port, nil
}
