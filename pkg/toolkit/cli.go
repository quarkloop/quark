package toolkit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// BuildCLI generates a cobra command tree for the given tool.
func BuildCLI(tool AgentTool) *cobra.Command {
	root := &cobra.Command{
		Use:   tool.Name(),
		Short: tool.Description(),
		SilenceUsage: true,
	}

	// Global --pipe flag
	var pipeMode bool
	root.Flags().BoolVar(&pipeMode, "pipe", false, "Read JSON from stdin and write JSON to stdout")

	root.RunE = func(cmd *cobra.Command, args []string) error {
		if pipeMode {
			return RunPipe(tool)
		}
		return cmd.Help()
	}

	// Add schema subcommand
	root.AddCommand(&cobra.Command{
		Use:   "schema",
		Short: "Print the tool's JSON schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(tool.Schema())
		},
	})

	// Add health subcommand
	root.AddCommand(&cobra.Command{
		Use:   "health",
		Short: "Print health status JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"status":  "ok",
				"name":    tool.Name(),
				"version": tool.Version(),
			})
		},
	})

	// Add skill subcommand
	root.AddCommand(&cobra.Command{
		Use:   "skill",
		Short: "Print SKILL.md content",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Try to read SKILL.md from current directory
			data, err := os.ReadFile("SKILL.md")
			if err != nil {
				fmt.Fprintln(os.Stderr, "SKILL.md not found")
				return nil
			}
			fmt.Print(string(data))
			return nil
		},
	})

	// Add serve subcommand
	root.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, _ := cmd.Flags().GetString("addr")
			return RunServer(tool, addr)
		},
	})
	root.PersistentFlags().String("addr", "127.0.0.1:0", "Address to listen on")

	// Add command subcommands
	for _, cmdDef := range tool.Commands() {
		cmdDef := cmdDef // capture range variable
		cmd := &cobra.Command{
			Use:   buildUseLine(cmdDef),
			Short: cmdDef.Description,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runCommand(tool, cmdDef, cmd, args, false)
			},
		}

		// Add --json flag
		cmd.Flags().Bool("json", false, "Output result as JSON")

		// Add defined flags
		for _, f := range cmdDef.Flags {
			addFlag(cmd, f)
		}

		root.AddCommand(cmd)
	}

	return root
}

func buildUseLine(cmd Command) string {
	parts := []string{cmd.Name}
	for _, arg := range cmd.Args {
		if arg.Required {
			parts = append(parts, "<"+arg.Name+">")
		} else {
			parts = append(parts, "["+arg.Name+"]")
		}
	}
	return strings.Join(parts, " ")
}

func addFlag(cmd *cobra.Command, f Flag) {
	switch f.Type {
	case "int":
		def := 0
		if f.Default != nil {
			def = f.Default.(int)
		}
		if f.Short != "" {
			cmd.Flags().IntP(f.Name, f.Short, def, f.Description)
		} else {
			cmd.Flags().Int(f.Name, def, f.Description)
		}
	case "bool":
		def := false
		if f.Default != nil {
			def = f.Default.(bool)
		}
		if f.Short != "" {
			cmd.Flags().BoolP(f.Name, f.Short, def, f.Description)
		} else {
			cmd.Flags().Bool(f.Name, def, f.Description)
		}
	default: // string
		def := ""
		if f.Default != nil {
			def = f.Default.(string)
		}
		if f.Short != "" {
			cmd.Flags().StringP(f.Name, f.Short, def, f.Description)
		} else {
			cmd.Flags().String(f.Name, def, f.Description)
		}
	}
}

func runCommand(tool AgentTool, cmdDef Command, cmd *cobra.Command, args []string, forceJSON bool) error {
	jsonFlag, _ := cmd.Flags().GetBool("json")
	useJSON := jsonFlag || forceJSON

	input := Input{
		Args:  make(map[string]string),
		Flags: make(map[string]any),
	}

	// Map positional args
	argIdx := 0
	for _, arg := range cmdDef.Args {
		if argIdx < len(args) {
			input.Args[arg.Name] = args[argIdx]
			argIdx++
		} else if arg.Required {
			return fmt.Errorf("missing required argument: %s", arg.Name)
		} else {
			input.Args[arg.Name] = arg.Default
		}
	}

	// Map flags
	for _, f := range cmdDef.Flags {
		var val any
		switch f.Type {
		case "int":
			val, _ = cmd.Flags().GetInt(f.Name)
		case "bool":
			val, _ = cmd.Flags().GetBool(f.Name)
		default:
			val, _ = cmd.Flags().GetString(f.Name)
		}
		input.Flags[f.Name] = val
	}

	out, err := cmdDef.Handler(context.Background(), input)
	if err != nil {
		return err
	}

	if useJSON {
		return json.NewEncoder(os.Stdout).Encode(out)
	}

	// Simple text output
	if out.Error != "" {
		fmt.Fprintln(os.Stderr, "Error:", out.Error)
	}
	for k, v := range out.Data {
		fmt.Printf("%s: %v\n", k, v)
	}
	return nil
}
