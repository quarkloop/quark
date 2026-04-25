package buildrelease

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"
)

// ── 1. LoadConfigStage ───────────────────────
type LoadConfigStage struct{ ConfigFile string }

func (LoadConfigStage) Name() string { return "Load configuration" }

func (s LoadConfigStage) Run(ctx *PipelineContext) error {
	data, err := os.ReadFile(s.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cfg := ctx.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse %s: %w", s.ConfigFile, err)
	}
	ctx.Config = cfg
	return nil
}

// ── 2. ResolveVersionStage ───────────────────
type ResolveVersionStage struct{}

func (ResolveVersionStage) Name() string { return "Resolve version" }

func (ResolveVersionStage) Run(ctx *PipelineContext) error {
	version := ctx.Config.Version
	if version == "" {
		v, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
		if err != nil {
			version = "dev"
		} else {
			version = strings.TrimSpace(string(v))
		}
	}
	ctx.Version = version

	commit, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		ctx.Commit = "unknown"
	} else {
		ctx.Commit = strings.TrimSpace(string(commit))
	}

	ctx.BuildTime = time.Now().Format(time.RFC3339)
	ctx.GoVersion = runtime.Version()
	return nil
}

// ── 3. ValidateStage ─────────────────────────
type ValidateStage struct{}

func (ValidateStage) Name() string { return "Validate" }

func (ValidateStage) Run(ctx *PipelineContext) error {
	if ctx.Config.PackageName == "" {
		return errors.New("package_name is required")
	}
	if len(ctx.Config.Builds) == 0 {
		return errors.New("at least one build target is required")
	}
	if len(ctx.Config.Targets) == 0 {
		return errors.New("at least one target is required")
	}
	return nil
}

// ── 4. TestStage ─────────────────────────────
type TestStage struct{}

func (TestStage) Name() string { return "Test" }

func (TestStage) Run(ctx *PipelineContext) error {
	if ctx.SkipTests {
		return nil
	}
	for _, build := range ctx.Config.Builds {
		cmd := exec.Command("go", "test", "./...")
		cmd.Dir = build.SourceDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tests failed for %s: %w", build.Name, err)
		}
	}
	return nil
}

// ── 5. PrepareStage ──────────────────────────
type PrepareStage struct{}

func (PrepareStage) Name() string { return "Prepare" }

func (PrepareStage) Run(ctx *PipelineContext) error {
	if err := os.MkdirAll(ctx.Config.ReleaseDir, 0755); err != nil {
		return fmt.Errorf("create release dir: %w", err)
	}
	return nil
}

// ── 6. BuildStage ────────────────────────────
type BuildStage struct{}

func (BuildStage) Name() string { return "Build" }

func (BuildStage) Run(ctx *PipelineContext) error {
	semaphore := make(chan struct{}, ctx.Parallelism)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, build := range ctx.Config.Builds {
		for _, target := range ctx.Config.Targets {
			wg.Add(1)
			go func(b BuildTarget, t Target) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				art := buildOne(ctx, b, t)
				mu.Lock()
				ctx.Artifacts = append(ctx.Artifacts, art)
				mu.Unlock()
			}(build, target)
		}
	}
	wg.Wait()

	var errs []string
	for _, a := range ctx.Artifacts {
		if a.Error != "" {
			errs = append(errs, fmt.Sprintf("%s: %s", a.ArchiveName, a.Error))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func buildOne(ctx *PipelineContext, build BuildTarget, target Target) Artifact {
	archiveBase := fmt.Sprintf("%s_%s_%s", ctx.Config.PackageName, target.OS, target.Arch)
	if target.ARM != "" {
		archiveBase = fmt.Sprintf("%s_%s_%s%s", ctx.Config.PackageName, target.OS, target.Arch, target.ARM)
	}
	archiveName := archiveBase + ".tar.gz"
	binName := build.BinaryName
	if target.OS == "windows" {
		binName += ".exe"
	}

	art := Artifact{
		BuildName:   build.Name,
		Target:      target,
		ArchiveName: archiveName,
	}

	if ctx.DryRun {
		return art
	}

	ldflags := ctx.Config.LDFlags
	if build.LDFlags != "" {
		ldflags = build.LDFlags
	}
	extra := build.ExtraLDFlags
	if extra == "" {
		extra = ctx.Config.ExtraLDFlags
	}
	extra = expandLDFlags(extra, ctx)
	if extra != "" {
		ldflags += " " + extra
	}

	env := os.Environ()
	env = append(env, "CGO_ENABLED=0")
	env = append(env, fmt.Sprintf("GOOS=%s", target.OS))
	env = append(env, fmt.Sprintf("GOARCH=%s", target.Arch))
	if target.ARM != "" {
		env = append(env, fmt.Sprintf("GOARM=%s", target.ARM))
	}

	tmpDir, err := os.MkdirTemp("", "buildrelease-*")
	if err != nil {
		art.Error = fmt.Sprintf("mkdtemp: %v", err)
		return art
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, binName)
	args := []string{"build", "-ldflags", ldflags, "-o", binPath}
	if ctx.Config.Compress {
		args = append(args, "-ldflags", ldflags+" -s -w")
	}
	args = append(args, build.SourcePath)

	cmd := exec.Command("go", args...)
	cmd.Dir = build.SourceDir
	cmd.Env = env

	start := time.Now()
	out, err := cmd.CombinedOutput()
	art.Duration = time.Since(start)

	if err != nil {
		art.Error = fmt.Sprintf("go build: %v\n%s", err, string(out))
		return art
	}

	// Include extra files
	for _, f := range build.IncludeFiles {
		src := filepath.Join(build.SourceDir, f)
		if _, st := os.Stat(src); st == nil {
			cp := filepath.Join(tmpDir, filepath.Base(f))
			_ = copyFile(src, cp)
		}
	}

	if ctx.Config.Compress {
		_ = exec.Command("upx", "-q", binPath).Run()
	}

	archivePath := filepath.Join(ctx.Config.ReleaseDir, archiveName)
	if err := createTarGz(tmpDir, archivePath, binName); err != nil {
		art.Error = fmt.Sprintf("archive: %v", err)
		return art
	}

	info, _ := os.Stat(archivePath)
	if info != nil {
		art.Size = info.Size()
		art.Filename = archivePath
	}

	return art
}

func expandLDFlags(flags string, ctx *PipelineContext) string {
	repl := strings.NewReplacer(
		"{{.Version}}", ctx.Version,
		"{{.Commit}}", ctx.Commit,
		"{{.BuildTime}}", ctx.BuildTime,
		"{{.Date}}", ctx.BuildTime,
	)
	return repl.Replace(flags)
}

func createTarGz(srcDir, dstPath, binName string) error {
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(srcDir, e.Name())
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = e.Name()
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !e.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				data.Close()
				return err
			}
			data.Close()
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// ── 7. InstallScriptStage ────────────────────
type InstallScriptStage struct{}

func (InstallScriptStage) Name() string { return "Generate install script" }

func (InstallScriptStage) Run(ctx *PipelineContext) error {
	if ctx.DryRun {
		return nil
	}

	repoOwner, repoName := splitRepo(ctx.Config.Homepage)
	data := struct {
		PackageName string
		BinaryName  string
		Version     string
		InstallDir  string
		Homepage    string
		RepoOwner   string
		RepoName    string
		Artifacts   []Artifact
		Date        string
	}{
		PackageName: ctx.Config.PackageName,
		BinaryName:  ctx.Config.BinaryName,
		Version:     ctx.Version,
		InstallDir:  ctx.Config.InstallDir,
		Homepage:    ctx.Config.Homepage,
		RepoOwner:   repoOwner,
		RepoName:    repoName,
		Artifacts:   ctx.Artifacts,
		Date:        time.Now().Format("2006-01-02"),
	}

	raw, err := os.ReadFile("scripts/install.sh.tmpl")
	if err != nil {
		return err
	}
	tmpl, err := template.New("install").Parse(string(raw))
	if err != nil {
		return err
	}

	out := filepath.Join(ctx.Config.ReleaseDir, "install.sh")
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, data)
}

func splitRepo(homepage string) (string, string) {
	parts := strings.Split(strings.TrimPrefix(homepage, "https://github.com/"), "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// ── 8. ChecksumStage ─────────────────────────
type ChecksumStage struct{}

func (ChecksumStage) Name() string { return "Generate checksums" }

func (ChecksumStage) Run(ctx *PipelineContext) error {
	if !ctx.Config.Checksums || ctx.DryRun {
		return nil
	}

	outPath := filepath.Join(ctx.Config.ReleaseDir, "checksums.txt")
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	for _, art := range ctx.Artifacts {
		if art.Error != "" || art.Filename == "" {
			continue
		}
		sum, err := sha256File(art.Filename)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s  %s\n", sum, art.ArchiveName)
		art.Checksum = sum
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ── 9. SignStage ─────────────────────────────
type SignStage struct{}

func (SignStage) Name() string { return "Sign artifacts" }

func (SignStage) Run(ctx *PipelineContext) error {
	if !ctx.Config.Sign || ctx.Config.SignKey == "" || ctx.DryRun {
		return nil
	}
	checksumPath := filepath.Join(ctx.Config.ReleaseDir, "checksums.txt")
	cmd := exec.Command("gpg", "--detach-sign", "--armor", "-u", ctx.Config.SignKey, checksumPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ── 10. ReadmeStage ──────────────────────────
type ReadmeStage struct{}

func (ReadmeStage) Name() string { return "Generate release README" }

func (ReadmeStage) Run(ctx *PipelineContext) error {
	if ctx.DryRun {
		return nil
	}
	data := struct {
		PackageName string
		Version     string
		Description string
		Homepage    string
		License     string
		Author      string
		Artifacts   []Artifact
	}{
		PackageName: ctx.Config.PackageName,
		Version:     ctx.Version,
		Description: ctx.Config.Description,
		Homepage:    ctx.Config.Homepage,
		License:     ctx.Config.License,
		Author:      ctx.Config.Author,
		Artifacts:   ctx.Artifacts,
	}

	raw, err := os.ReadFile("scripts/readme.md.tmpl")
	if err != nil {
		return err
	}
	tmpl, err := template.New("readme").Parse(string(raw))
	if err != nil {
		return err
	}

	out := filepath.Join(ctx.Config.ReleaseDir, "README.md")
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, data)
}

// ── 11. MetadataStage ────────────────────────
type MetadataStage struct{}

func (MetadataStage) Name() string { return "Generate release metadata" }

func (MetadataStage) Run(ctx *PipelineContext) error {
	if ctx.DryRun {
		return nil
	}
	meta := map[string]any{
		"package_name": ctx.Config.PackageName,
		"version":      ctx.Version,
		"commit":       ctx.Commit,
		"go_version":   ctx.GoVersion,
		"build_time":   ctx.BuildTime,
		"artifacts":    ctx.Artifacts,
	}
	out := filepath.Join(ctx.Config.ReleaseDir, "release.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(out, data, 0644)
}
