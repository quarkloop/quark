package write

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func applyWrite(req normalizedRequest, info os.FileInfo, exists bool) (Result, error) {
	if err := os.MkdirAll(filepath.Dir(req.Path), 0755); err != nil {
		return Result{}, fmt.Errorf("creating parent dir: %w", err)
	}
	if err := writeFile(req.Path, req.Content, fileMode(info, exists)); err != nil {
		return Result{}, err
	}
	return buildResult(req.Path, "write", resultStats{
		Created:      !exists,
		Changed:      true,
		BytesWritten: len(req.Content),
	})
}

func applyAppend(req normalizedRequest, info os.FileInfo, exists bool) (Result, error) {
	if err := os.MkdirAll(filepath.Dir(req.Path), 0755); err != nil {
		return Result{}, fmt.Errorf("creating parent dir: %w", err)
	}

	f, err := os.OpenFile(req.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, fileMode(info, exists))
	if err != nil {
		return Result{}, fmt.Errorf("opening file for append: %w", err)
	}
	defer f.Close()

	n, err := f.WriteString(req.Content)
	if err != nil {
		return Result{}, fmt.Errorf("appending file: %w", err)
	}

	return buildResult(req.Path, "append", resultStats{
		Created:      !exists,
		Changed:      n > 0,
		BytesWritten: n,
	})
}

func applyReplace(req normalizedRequest, exists bool) (Result, error) {
	if !exists {
		return Result{}, fmt.Errorf("replace requires an existing file")
	}

	data, err := os.ReadFile(req.Path)
	if err != nil {
		return Result{}, fmt.Errorf("reading file: %w", err)
	}

	content := string(data)
	replacements := strings.Count(content, req.Find)
	if replacements == 0 {
		return Result{}, fmt.Errorf("text %q not found in %s", req.Find, req.Path)
	}

	updated := strings.ReplaceAll(content, req.Find, req.ReplaceWith)
	if err := writeFile(req.Path, updated, fileModeFromPath(req.Path)); err != nil {
		return Result{}, err
	}

	return buildResult(req.Path, "replace", resultStats{
		Changed:      updated != content,
		BytesWritten: len(updated),
		Replacements: replacements,
	})
}

func applyEdit(req normalizedRequest, exists bool) (Result, error) {
	if !exists {
		return Result{}, fmt.Errorf("edit requires an existing file")
	}

	data, err := os.ReadFile(req.Path)
	if err != nil {
		return Result{}, fmt.Errorf("reading file: %w", err)
	}

	updated, applied, err := applyLineEdits(string(data), req.Edits)
	if err != nil {
		return Result{}, err
	}
	changed := updated != string(data)
	if changed {
		if err := writeFile(req.Path, updated, fileModeFromPath(req.Path)); err != nil {
			return Result{}, err
		}
	}

	return buildResult(req.Path, "edit", resultStats{
		Changed:      changed,
		BytesWritten: len(updated),
		EditsApplied: applied,
	})
}

func fileModeFromPath(path string) os.FileMode {
	info, err := os.Stat(path)
	if err == nil {
		return info.Mode().Perm()
	}
	return 0644
}
