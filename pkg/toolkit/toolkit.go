// Package toolkit provides a framework for building quark tool plugins.
// Tool authors implement AgentTool with a set of Commands; the framework
// generates CLI, HTTP server, pipe, and lib-mode surfaces automatically.
package toolkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/quarkloop/pkg/plugin"
	"github.com/spf13/cobra"
)

// Command describes a single subcommand that a tool exposes.
type Command struct {
	Name        string
	Description string
	Args        []Arg
	Flags       []Flag
	Handler     func(ctx context.Context, input Input) (Output, error)
}

// Arg describes a positional argument.
type Arg struct {
	Name        string
	Description string
	Required    bool
	Default     string
}

// Flag describes a named flag.
type Flag struct {
	Name        string
	Short       string
	Description string
	Type        string // string | int | bool
	Default     any
}

// Input is passed to every Command handler.
type Input struct {
	Args  map[string]string // positional args by name
	Flags map[string]any    // flag values by name
}

// Output is returned by every Command handler.
type Output struct {
	Data  map[string]any `json:"data"`  // structured payload
	Error string         `json:"error"` // non-fatal error message
}

// AgentTool is the interface every tool must implement.
type AgentTool interface {
	Name() string
	Version() string
	Description() string
	Schema() plugin.ToolSchema
	Commands() []Command
}

// Execute runs the cobra command and exits on error.
func Execute(cmd *cobra.Command) {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// findCommand looks up a command by name.
func findCommand(tool AgentTool, name string) (*Command, bool) {
	for i := range tool.Commands() {
		if tool.Commands()[i].Name == name {
			return &tool.Commands()[i], true
		}
	}
	return nil, false
}

// LoadManifest finds and parses a manifest.yaml file.
// It searches relative to the current working directory (for development),
// then relative to the executable directory (for deployed plugins).
func LoadManifest() (*plugin.Manifest, error) {
	searchPaths := []string{
		"manifest.yaml",
		"../manifest.yaml",
		"../../manifest.yaml",
	}
	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			return plugin.ParseManifest(p)
		}
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(exeDir, "manifest.yaml"),
			filepath.Join(exeDir, "..", "manifest.yaml"),
			filepath.Join(exeDir, "..", "..", "manifest.yaml"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return plugin.ParseManifest(p)
			}
		}
	}
	// For go run: resolve via the caller's source file (skip = 1 = whoever called LoadManifest)
	_, file, _, ok := runtime.Caller(1)
	if ok {
		callerDir := filepath.Dir(file)
		candidates := []string{
			filepath.Join(callerDir, "..", "..", "..", "manifest.yaml"), // from pkg/<tool>/tool.go
			filepath.Join(callerDir, "..", "..", "manifest.yaml"),       // from pkg/<tool>/<file>.go
			filepath.Join(callerDir, "..", "manifest.yaml"),             // from cmd/<tool>/main.go
		}
		for _, p := range candidates {
			if abs, err := filepath.Abs(p); err == nil {
				if _, err := os.Stat(abs); err == nil {
					return plugin.ParseManifest(abs)
				}
			}
		}
	}
	return nil, fmt.Errorf("manifest.yaml not found")
}
