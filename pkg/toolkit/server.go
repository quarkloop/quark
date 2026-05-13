package toolkit

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

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

	if schemaName := tool.Schema().Name; schemaName != "" {
		app.Post("/"+schemaName, func(c *fiber.Ctx) error {
			return handleSchemaInvoke(c, tool)
		})
	}

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
			return dispatchCommand(c, &cmdDef, reqBody)
		})
	}

	return app
}

func handleSchemaInvoke(c *fiber.Ctx, tool AgentTool) error {
	var reqBody map[string]any
	if err := c.BodyParser(&reqBody); err != nil {
		return writeError(c, "invalid request: "+err.Error())
	}
	cmd, ok := commandForSchemaCall(tool, reqBody)
	if !ok {
		return writeError(c, "could not resolve command for tool: "+tool.Schema().Name)
	}
	return dispatchCommand(c, cmd, reqBody)
}

func commandForSchemaCall(tool AgentTool, body map[string]any) (*Command, bool) {
	if raw, ok := bodyValue(body, "command"); ok {
		if name, ok := stringValue(raw); ok {
			if cmd, ok := findCommand(tool, name); ok {
				return cmd, true
			}
		}
	}
	if name := tool.Schema().Name; name != "" {
		if cmd, ok := findCommand(tool, name); ok {
			return cmd, true
		}
	}
	commands := tool.Commands()
	if len(commands) == 1 {
		return &commands[0], true
	}
	return nil, false
}

func dispatchCommand(c *fiber.Ctx, cmd *Command, body map[string]any) error {
	input, err := inputFromBody(cmd, body)
	if err != nil {
		return writeError(c, err.Error())
	}
	out, err := cmd.Handler(c.Context(), input)
	if err != nil {
		out.Error = err.Error()
	}
	return writeOutput(c, out)
}

func inputFromBody(cmd *Command, body map[string]any) (Input, error) {
	input := Input{Args: make(map[string]string), Flags: make(map[string]any)}
	for _, arg := range cmd.Args {
		input.Args[arg.Name] = arg.Default
		if v, ok := bodyValue(body, arg.Name); ok {
			s, ok := stringValue(v)
			if !ok {
				return input, fmt.Errorf("arg %s must be a string", arg.Name)
			}
			input.Args[arg.Name] = s
		}
	}
	for _, flag := range cmd.Flags {
		input.Flags[flag.Name] = flag.Default
		if v, ok := bodyValue(body, flag.Name); ok {
			converted, err := convertFlag(flag, v)
			if err != nil {
				return input, err
			}
			input.Flags[flag.Name] = converted
		}
	}
	return input, nil
}

func bodyValue(body map[string]any, name string) (any, bool) {
	candidates := []string{name, strings.ReplaceAll(name, "-", "_"), strings.ReplaceAll(name, "_", "-")}
	if name == "query" {
		candidates = append(candidates, "q")
	}
	if name == "cmd" {
		candidates = append(candidates, "command")
	}
	for _, candidate := range candidates {
		if v, ok := body[candidate]; ok {
			return v, true
		}
	}
	return nil, false
}

func stringValue(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case fmt.Stringer:
		return t.String(), true
	default:
		return "", false
	}
}

func convertFlag(flag Flag, v any) (any, error) {
	switch flag.Type {
	case "int":
		switch t := v.(type) {
		case int:
			return t, nil
		case float64:
			return int(t), nil
		case string:
			n, err := strconv.Atoi(t)
			if err != nil {
				return nil, fmt.Errorf("flag %s must be an int", flag.Name)
			}
			return n, nil
		default:
			return nil, fmt.Errorf("flag %s must be an int", flag.Name)
		}
	case "bool":
		switch t := v.(type) {
		case bool:
			return t, nil
		case string:
			b, err := strconv.ParseBool(t)
			if err != nil {
				return nil, fmt.Errorf("flag %s must be a bool", flag.Name)
			}
			return b, nil
		default:
			return nil, fmt.Errorf("flag %s must be a bool", flag.Name)
		}
	default:
		s, ok := stringValue(v)
		if !ok {
			return nil, fmt.Errorf("flag %s must be a string", flag.Name)
		}
		return s, nil
	}
}

func writeOutput(c *fiber.Ctx, out Output) error {
	return c.JSON(out)
}

func writeError(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusBadRequest).JSON(Output{Error: msg})
}
