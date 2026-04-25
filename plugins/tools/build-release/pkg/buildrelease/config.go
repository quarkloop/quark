package buildrelease

import "time"

// BuildTarget describes a single binary to compile.
type BuildTarget struct {
	Name         string   `json:"name"`
	SourcePath   string   `json:"source_path"`
	BinaryName   string   `json:"binary_name"`
	SourceDir    string   `json:"source_dir"`
	LDFlags      string   `json:"ldflags"`
	ExtraLDFlags string   `json:"extra_ldflags"`
	IncludeFiles []string `json:"include_files,omitempty"`
}

// Target describes a single cross-compilation target.
type Target struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	ARM  string `json:"arm,omitempty"`
}

// Artifact is a completed build: an archive on disk with metadata.
type Artifact struct {
	BuildName   string
	Target
	Filename    string
	ArchiveName string
	Checksum    string
	Size        int64
	Duration    time.Duration
	Error       string
}

// ReleaseConfig holds every knob a user might want to tweak.
type ReleaseConfig struct {
	PackageName  string        `json:"package_name"`
	Version      string        `json:"version"`
	BinaryName   string        `json:"binary_name"`
	Description  string        `json:"description"`
	Homepage     string        `json:"homepage"`
	License      string        `json:"license"`
	Author       string        `json:"author"`
	InstallDir   string        `json:"install_dir"`
	ReleaseDir   string        `json:"release_dir"`
	LDFlags      string        `json:"ldflags"`
	ExtraLDFlags string        `json:"extra_ldflags"`
	Compress     bool          `json:"compress"`
	Checksums    bool          `json:"checksums"`
	Sign         bool          `json:"sign"`
	SignKey      string        `json:"sign_key"`
	Builds       []BuildTarget `json:"builds"`
	Targets      []Target      `json:"targets"`
}

// PipelineContext is shared mutable state threaded through every stage.
type PipelineContext struct {
	Config      ReleaseConfig
	ConfigFile  string
	Version     string
	Commit      string
	BuildTime   string
	GoVersion   string
	Artifacts   []Artifact
	DryRun      bool
	Parallelism int
	SkipTests   bool
}

// Stage is a single, named unit of pipeline work.
type Stage interface {
	Name() string
	Run(ctx *PipelineContext) error
}

// Pipeline runs a sequence of Stages, halting on the first error.
type Pipeline struct {
	stages []Stage
}

// NewPipeline constructs a Pipeline from an ordered list of stages.
func NewPipeline(stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages}
}

// Run executes each stage in order.
func (p *Pipeline) Run(ctx *PipelineContext) error {
	for _, s := range p.stages {
		if err := s.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}
