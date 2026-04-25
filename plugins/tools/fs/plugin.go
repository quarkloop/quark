//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/pkg/toolkit"
	"github.com/quarkloop/plugins/tools/fs/pkg/fs"
)

var (
	manifest *plugin.Manifest
	tool     = &fs.Tool{}
)

func init() {
	var err error
	manifest, err = toolkit.LoadManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fs: %v\n", err)
		os.Exit(1)
	}
}

// QuarkPlugin is the exported plugin instance for lib-mode loading.
var QuarkPlugin plugin.Plugin = &FSTool{}

// FSTool implements the ToolPlugin interface, sourcing metadata from manifest.yaml.
type FSTool struct{}

func (t *FSTool) Name() string    { return manifest.Name }
func (t *FSTool) Version() string { return manifest.Version }
func (t *FSTool) Type() plugin.PluginType { return manifest.Type }
func (t *FSTool) Initialize(ctx context.Context, cfg map[string]any) error { return nil }
func (t *FSTool) Shutdown(ctx context.Context) error { return nil }

func (t *FSTool) Schema() plugin.ToolSchema {
	if manifest.Tool != nil {
		return manifest.Tool.Schema
	}
	return tool.Schema()
}

func (t *FSTool) Execute(ctx context.Context, args map[string]any) (plugin.ToolResult, error) {
	cmdName, _ := args["command"].(string)
	for _, cmd := range tool.Commands() {
		if cmd.Name != cmdName {
			continue
		}
		input := toolkit.Input{Args: make(map[string]string), Flags: make(map[string]any)}
		for _, arg := range cmd.Args {
			if v, ok := args[arg.Name]; ok {
				input.Args[arg.Name] = fmt.Sprintf("%v", v)
			}
		}
		for _, flag := range cmd.Flags {
			if v, ok := args[flag.Name]; ok {
				input.Flags[flag.Name] = v
			}
		}
		out, err := cmd.Handler(ctx, input)
		if err != nil {
			return plugin.ToolResult{IsError: true, Error: err.Error()}, nil
		}
		return plugin.ToolResult{Output: out.Data, Error: out.Error, IsError: out.Error != ""}, nil
	}
	return plugin.ToolResult{IsError: true, Error: "unknown command: " + cmdName}, nil
}
