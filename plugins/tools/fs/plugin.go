//go:build plugin

// Plugin export for lib-mode loading.
// Build with: go build -buildmode=plugin -tags plugin -o plugin.so ./
package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/pkg/toolkit"
	"github.com/quarkloop/plugins/tools/fs/pkg/fs"
)

// NewPlugin creates a new FSTool instance for lib-mode loading.
func NewPlugin() plugin.Plugin {
	return &FSTool{tool: &fs.Tool{}}
}

// FSTool implements the ToolPlugin interface, sourcing metadata from manifest.yaml.
type FSTool struct {
	manifest *plugin.Manifest
	tool     *fs.Tool
}

func (t *FSTool) Name() string            { return t.manifest.Name }
func (t *FSTool) Version() string         { return t.manifest.Version }
func (t *FSTool) Type() plugin.PluginType { return t.manifest.Type }

func (t *FSTool) Initialize(ctx context.Context, cfg map[string]any) error {
	manifest, err := plugin.ParseManifest(plugin.ManifestPathFromConfig(cfg))
	if err != nil {
		return err
	}
	t.manifest = manifest
	t.tool.SetManifest(manifest)
	return nil
}

func (t *FSTool) Shutdown(ctx context.Context) error { return nil }

func (t *FSTool) Schema() plugin.ToolSchema {
	if t.manifest.Tool != nil {
		return t.manifest.Tool.Schema
	}
	return t.tool.Schema()
}

func (t *FSTool) Execute(ctx context.Context, args map[string]any) (plugin.ToolResult, error) {
	cmdName, _ := stringValue(argsValue(args, "command"))
	for _, cmd := range t.tool.Commands() {
		if cmd.Name != cmdName {
			continue
		}
		input, err := inputFromArgs(cmd, args)
		if err != nil {
			return plugin.ToolResult{IsError: true, Error: err.Error()}, nil
		}
		out, err := cmd.Handler(ctx, input)
		if err != nil {
			return plugin.ToolResult{IsError: true, Error: err.Error()}, nil
		}
		return plugin.ToolResult{Output: out.Data, Error: out.Error, IsError: out.Error != ""}, nil
	}
	return plugin.ToolResult{IsError: true, Error: "unknown command: " + cmdName}, nil
}

func inputFromArgs(cmd toolkit.Command, args map[string]any) (toolkit.Input, error) {
	input := toolkit.Input{Args: make(map[string]string), Flags: make(map[string]any)}
	for _, arg := range cmd.Args {
		input.Args[arg.Name] = arg.Default
		if v, ok := argsValue(args, arg.Name); ok {
			input.Args[arg.Name] = fmt.Sprintf("%v", v)
		}
	}
	for _, flag := range cmd.Flags {
		input.Flags[flag.Name] = flag.Default
		if v, ok := argsValue(args, flag.Name); ok {
			converted, err := convertFlag(flag, v)
			if err != nil {
				return input, err
			}
			input.Flags[flag.Name] = converted
		}
	}
	return input, nil
}

func argsValue(args map[string]any, name string) (any, bool) {
	for _, candidate := range []string{name, strings.ReplaceAll(name, "-", "_"), strings.ReplaceAll(name, "_", "-")} {
		if v, ok := args[candidate]; ok {
			return v, true
		}
	}
	return nil, false
}

func stringValue(v any, ok bool) (string, bool) {
	if !ok {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case fmt.Stringer:
		return t.String(), true
	default:
		return "", false
	}
}

func convertFlag(flag toolkit.Flag, v any) (any, error) {
	switch flag.Type {
	case "int":
		switch t := v.(type) {
		case int:
			return t, nil
		case int32:
			return int(t), nil
		case int64:
			return int(t), nil
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
		if s, ok := v.(string); ok {
			return s, nil
		}
		if s, ok := v.(fmt.Stringer); ok {
			return s.String(), nil
		}
		return nil, fmt.Errorf("flag %s must be a string", flag.Name)
	}
}
