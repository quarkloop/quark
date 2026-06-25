// Package middleware contains Fiber middleware used by the HTTP server.
//
// Each middleware is a small, single-purpose wrapper:
//   - RequestID: adds a UUID to every request, exposes it via X-Request-ID
//     response header and via the request context for loggers.
//   - Logger: structured per-request logging (method, path, status,
//     duration) using zap.
//   - Recoverer: catches panics, logs them, returns 500.
//   - CORS: permissive CORS (for browser-based CLI tools).
package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ctxKey is an unexported type for context keys defined in this package.
type ctxKey int

// RequestIDKey is the context key for the per-request UUID.
const RequestIDKey ctxKey = 1

// RequestID middleware: assigns a UUID to every request, sets the
// X-Request-ID response header, and stores the UUID in the request
// context for loggers.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Locals("requestId", id)
		c.Set("X-Request-ID", id)
		// Also stash in user context for non-Fiber callers.
		c.SetUserContext(context.WithValue(c.UserContext(), RequestIDKey, id))
		return c.Next()
	}
}

// GetRequestID returns the request ID from a context, or "" if absent.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}

// Logger middleware: structured per-request logging.
func Logger(log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		reqId := c.Locals("requestId")

		fields := []zap.Field{
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
		}
		if reqId, ok := reqId.(string); ok && reqId != "" {
			fields = append(fields, zap.String("request_id", reqId))
		}
		if err != nil {
			fields = append(fields, zap.Error(err))
		}

		if status >= 500 {
			log.Error("request completed", fields...)
		} else if status >= 400 {
			log.Warn("request completed", fields...)
		} else {
			log.Info("request completed", fields...)
		}
		return err
	}
}

// Recoverer middleware: catches panics, logs them, returns 500.
func Recoverer(log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered",
					zap.String("method", c.Method()),
					zap.String("path", c.Path()),
					zap.Any("panic", r),
				)
				err = c.Status(500).JSON(fiber.Map{
					"code":    "INTERNAL",
					"message": fmt.Sprintf("internal server error: %v", r),
				})
			}
		}()
		return c.Next()
	}
}

// CORS middleware: permissive (allows any origin). Suitable for dev
// and for browser-based CLI tools.
func CORS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		if c.Method() == "OPTIONS" {
			return c.SendStatus(204)
		}
		return c.Next()
	}
}
