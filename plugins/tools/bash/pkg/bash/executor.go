// Package bash implements the quark bash tool — shell command execution.
package bash

import (
	"fmt"
	"os/exec"

	"github.com/gofiber/fiber/v2"
)

// Execute runs a shell command and returns its combined output.
func Execute(command string) ([]byte, error) {
	return exec.Command("bash", "-c", command).CombinedOutput()
}

// RunHandler returns an HTTP handler that executes shell commands.
// Accepts POST with body {"cmd":"..."} and returns {"output":"...","exit_code":0}.
func RunHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			Cmd string `json:"cmd"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(`{"error":"invalid request"}`)
		}
		if req.Cmd == "" {
			return c.Status(fiber.StatusBadRequest).SendString(`{"error":"cmd is required"}`)
		}
		out, err := Execute(req.Cmd)
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
		return c.JSON(fiber.Map{
			"output":    string(out),
			"exit_code": exitCode,
		})
	}
}

// Serve starts an HTTP server for the bash tool on the given address (api mode).
func Serve(addr string) error {
	app := fiber.New()
	app.Post("/bash", RunHandler())
	fmt.Printf("bash tool listening on %s\n", addr)
	return app.Listen(addr)
}
