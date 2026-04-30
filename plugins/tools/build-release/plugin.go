//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import (
	"context"
	"fmt"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/pkg/toolkit"
	"github.com/quarkloop/plugins/tools/build-release/pkg/buildrelease"
)

// NewPlugin creates a new BuildReleaseTool instance for lib-mode loading.
func NewPlugin() plugin.Plugin {
	return &BuildReleaseTool{tool: &buildrelease.Tool{}}
}

// BuildReleaseTool implements the ToolPlugin interface, sourcing metadata from manifest.yaml.
type BuildReleaseTool struct {
	manifest *plugin.Manifest
	tool     *buildrelease.Tool
}

func (t *BuildReleaseTool) Name() string    { return t.manifest.Name }
func (t *BuildReleaseTool) Version() string { return t.manifest.Version }
func (t *BuildReleaseTool) Type() plugin.PluginType { return t.manifest.Type }
func (t *BuildReleaseTool) Initialize(ctx context.Context, cfg map[string]any) error {
	manifest, err := plugin.ParseManifest("manifest.yaml")
	if err != nil {
		return err
	}
	t.manifest = manifest
	t.tool.SetManifest(manifest)
	return nil
}
func (t *BuildReleaseTool) Shutdown(ctx context.Context) error { return nil }

func (t *BuildReleaseTool) Schema() plugin.ToolSchema {
	if t.manifest.Tool != nil {
		return t.manifest.Tool.Schema
	}
	return t.tool.Schema()
}

func (t *BuildReleaseTool) Execute(ctx context.Context, args map[string]any) (plugin.ToolResult, error) {
	cmdName, _ := args["command"].(string)
	for _, cmd := range t.tool.Commands() {
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
