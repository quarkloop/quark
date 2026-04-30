package toolkit

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
)

// RunServer starts an HTTP server with all standard endpoints.
func RunServer(tool AgentTool, addr string) error {
	app := BuildServer(tool)
	fmt.Printf("%s tool listening on %s\n", tool.Name(), addr)
	return app.Listen(addr)
}

// BuildServer returns a Fiber app with standard tool endpoints.
func BuildServer(tool AgentTool) *fiber.App {
	app := fiber.New()

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"name":    tool.Name(),
			"version": tool.Version(),
		})
	})

	app.Get("/schema", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		enc := json.NewEncoder(c)
		enc.SetIndent("", "  ")
		return enc.Encode(tool.Schema())
	})

	app.Get("/skill", func(c *fiber.Ctx) error {
		data, err := os.ReadFile("SKILL.md")
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("SKILL.md not found")
		}
		c.Set("Content-Type", "text/plain")
		return c.Send(data)
	})

	// Universal invoke endpoint
	app.Post("/invoke", func(c *fiber.Ctx) error {
		var req struct {
			Command string            `json:"command"`
			Args    map[string]string `json:"args"`
			Flags   map[string]any    `json:"flags"`
		}
		if err := c.BodyParser(&req); err != nil {
			return writeError(c, "invalid request: "+err.Error())
		}

		cmd, ok := findCommand(tool, req.Command)
		if !ok {
			return writeError(c, "unknown command: "+req.Command)
		}

		input := Input{Args: make(map[string]string), Flags: make(map[string]any)}
		for _, arg := range cmd.Args {
			if v, ok := req.Args[arg.Name]; ok {
				input.Args[arg.Name] = v
			} else {
				input.Args[arg.Name] = arg.Default
			}
		}
		for k, v := range req.Flags {
			input.Flags[k] = v
		}

		out, err := cmd.Handler(c.Context(), input)
		if err != nil {
			out.Error = err.Error()
		}
		return writeOutput(c, out)
	})

	// Per-command endpoints
	for _, cmdDef := range tool.Commands() {
		cmdDef := cmdDef
		app.Post("/"+cmdDef.Name, func(c *fiber.Ctx) error {
			// For per-command endpoints, the body IS the args map directly
			var reqBody map[string]any
			if err := c.BodyParser(&reqBody); err != nil {
				return writeError(c, "invalid request: "+err.Error())
			}

			input := Input{Args: make(map[string]string), Flags: make(map[string]any)}
			for _, arg := range cmdDef.Args {
				if v, ok := reqBody[arg.Name]; ok {
					input.Args[arg.Name] = v.(string)
				} else {
					input.Args[arg.Name] = arg.Default
				}
			}
			// Remaining keys are flags
			for k, v := range reqBody {
				if _, isArg := input.Args[k]; !isArg {
					input.Flags[k] = v
				}
			}

			out, err := cmdDef.Handler(c.Context(), input)
			if err != nil {
				out.Error = err.Error()
			}
			return writeOutput(c, out)
		})
	}

	return app
}

func writeOutput(c *fiber.Ctx, out Output) error {
	return c.JSON(out)
}

func writeError(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusBadRequest).JSON(Output{Error: msg})
}
