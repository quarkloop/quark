package buildrelease

import (
	"context"
	"encoding/json"
	"os"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/pkg/toolkit"
)

// Tool implements the build-release plugin.
type Tool struct {
	manifest *plugin.Manifest
}

func (t *Tool) SetManifest(m *plugin.Manifest) {
	t.manifest = m
}

func (t *Tool) Name() string {
	return t.manifest.Name
}

func (t *Tool) Version() string {
	return t.manifest.Version
}

func (t *Tool) Description() string {
	return t.manifest.Description
}

func (t *Tool) Schema() plugin.ToolSchema {
	if t.manifest.Tool != nil {
		return t.manifest.Tool.Schema
	}
	return plugin.ToolSchema{
		Name:        "build-release",
		Description: "Build Go release artifacts with cross-compilation, checksums, and install scripts",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type": "string",
					"enum": []string{"release", "dryrun", "init", "schema", "health", "skill"},
				},
				"config": map[string]any{
					"type":        "string",
					"description": "Path to build_release.json config file",
				},
				"version": map[string]any{
					"type":        "string",
					"description": "Override version string",
				},
				"parallel": map[string]any{
					"type":        "integer",
					"description": "Number of parallel builds",
				},
				"skip_tests": map[string]any{
					"type":        "boolean",
					"description": "Skip running tests before build",
				},
			},
			"required": []string{"command"},
		},
	}
}

func (t *Tool) Commands() []toolkit.Command {
	return []toolkit.Command{
		{
			Name:        "release",
			Description: "Run the full release pipeline",
			Args: []toolkit.Arg{
				{Name: "config", Description: "Config file path", Required: false, Default: "build_release.json"},
			},
			Flags: []toolkit.Flag{
				{Name: "version", Type: "string", Description: "Override version", Default: ""},
				{Name: "parallel", Type: "int", Description: "Parallel builds", Default: 0},
				{Name: "skip-tests", Type: "bool", Description: "Skip tests", Default: false},
			},
			Handler: t.handleRelease,
		},
		{
			Name:        "dryrun",
			Description: "Preview what would be built without compiling",
			Args: []toolkit.Arg{
				{Name: "config", Description: "Config file path", Required: false, Default: "build_release.json"},
			},
			Flags: []toolkit.Flag{
				{Name: "version", Type: "string", Description: "Override version", Default: ""},
			},
			Handler: t.handleDryRun,
		},
		{
			Name:        "init",
			Description: "Scaffold a new project with build_release files",
			Handler:     t.handleInit,
		},
	}
}

func (t *Tool) handleRelease(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
	pipeCtx := t.makeContext(input, false)
	pipeline := NewPipeline(
		LoadConfigStage{ConfigFile: pipeCtx.ConfigFile},
		ResolveVersionStage{},
		ValidateStage{},
		TestStage{},
		PrepareStage{},
		BuildStage{},
		InstallScriptStage{},
		ChecksumStage{},
		SignStage{},
		ReadmeStage{},
		MetadataStage{},
	)
	if err := pipeline.Run(pipeCtx); err != nil {
		return toolkit.Output{Error: err.Error()}, nil
	}
	return toolkit.Output{Data: map[string]any{
		"version":     pipeCtx.Version,
		"artifacts":   len(pipeCtx.Artifacts),
		"release_dir": pipeCtx.Config.ReleaseDir,
	}}, nil
}

func (t *Tool) handleDryRun(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
	pipeCtx := t.makeContext(input, true)
	pipeline := NewPipeline(
		LoadConfigStage{ConfigFile: pipeCtx.ConfigFile},
		ResolveVersionStage{},
		ValidateStage{},
		BuildStage{},
	)
	if err := pipeline.Run(pipeCtx); err != nil {
		return toolkit.Output{Error: err.Error()}, nil
	}
	var planned []map[string]any
	for _, a := range pipeCtx.Artifacts {
		planned = append(planned, map[string]any{
			"archive": a.ArchiveName,
			"os":      a.OS,
			"arch":    a.Arch,
		})
	}
	return toolkit.Output{Data: map[string]any{
		"version": pipeCtx.Version,
		"planned": planned,
	}}, nil
}

func (t *Tool) handleInit(ctx context.Context, input toolkit.Input) (toolkit.Output, error) {
	cfg := ReleaseConfig{
		PackageName: "mytool",
		ReleaseDir:  "dist",
		LDFlags:     "-s -w",
		Builds: []BuildTarget{
			{Name: "mytool", SourcePath: ".", BinaryName: "mytool", SourceDir: "."},
		},
		Targets: []Target{
			{OS: "linux", Arch: "amd64"},
			{OS: "linux", Arch: "arm64"},
			{OS: "darwin", Arch: "amd64"},
			{OS: "darwin", Arch: "arm64"},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile("build_release.json", data, 0644); err != nil {
		return toolkit.Output{Error: err.Error()}, nil
	}
	return toolkit.Output{Data: map[string]any{"created": "build_release.json"}}, nil
}

func (t *Tool) makeContext(input toolkit.Input, dryRun bool) *PipelineContext {
	ctx := &PipelineContext{
		Config: ReleaseConfig{
			PackageName: "mytool",
			ReleaseDir:  "dist",
			LDFlags:     "-s -w",
			Builds: []BuildTarget{
				{Name: "mytool", SourcePath: ".", BinaryName: "mytool", SourceDir: "."},
			},
			Targets: []Target{
				{OS: "linux", Arch: "amd64"},
				{OS: "linux", Arch: "arm64"},
				{OS: "darwin", Arch: "amd64"},
				{OS: "darwin", Arch: "arm64"},
			},
		},
		DryRun:      dryRun,
		Parallelism: 4,
	}
	cfgFile := input.Args["config"]
	if cfgFile == "" {
		cfgFile = "build_release.json"
	}
	ctx.ConfigFile = cfgFile
	if v, ok := input.Flags["version"].(string); ok && v != "" {
		ctx.Config.Version = v
	}
	if p, ok := input.Flags["parallel"].(int); ok && p > 0 {
		ctx.Parallelism = p
	}
	if s, ok := input.Flags["skip-tests"].(bool); ok {
		ctx.SkipTests = s
	}
	return ctx
}
