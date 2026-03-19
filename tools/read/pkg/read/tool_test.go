package read

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyReadsWholeFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "notes.txt")
	content := "alpha\nbeta\n"
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	res, err := Apply(Request{Path: target})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.Content != content {
		t.Fatalf("expected full content %q, got %q", content, res.Content)
	}
	if res.StartLine != 1 || res.EndLine != 2 || res.TotalLines != 2 {
		t.Fatalf("expected full file line metadata, got %+v", res)
	}
	if res.BytesRead != len(content) {
		t.Fatalf("expected bytes_read=%d, got %+v", len(content), res)
	}
}

func TestApplyReadsLineRange(t *testing.T) {
	target := filepath.Join(t.TempDir(), "app.py")
	content := "def greet(name):\n    return name\n\nprint(greet(\"quark\"))\n"
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	res, err := Apply(Request{
		Path:      target,
		StartLine: 2,
		EndLine:   3,
	})
	if err != nil {
		t.Fatalf("read range: %v", err)
	}
	if want := "    return name\n\n"; res.Content != want {
		t.Fatalf("expected range content %q, got %q", want, res.Content)
	}
	if res.StartLine != 2 || res.EndLine != 3 || res.TotalLines != 4 {
		t.Fatalf("expected range line metadata, got %+v", res)
	}
	if !strings.Contains(res.ContentPreview, "return name") {
		t.Fatalf("expected preview to include selected content, got %+v", res)
	}
}

func TestApplyRejectsMissingFile(t *testing.T) {
	_, err := Apply(Request{Path: filepath.Join(t.TempDir(), "missing.txt")})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

func TestApplyRejectsInvalidRange(t *testing.T) {
	target := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(target, []byte("alpha\nbeta\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	_, err := Apply(Request{
		Path:      target,
		StartLine: 3,
		EndLine:   4,
	})
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected out of range error, got %v", err)
	}
}

func TestApplyRejectsNonRegularFile(t *testing.T) {
	_, err := Apply(Request{Path: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("expected regular file error, got %v", err)
	}
}

func TestApplyRejectsBinaryFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "binary.txt")
	if err := os.WriteFile(target, []byte("a\x00b"), 0644); err != nil {
		t.Fatalf("seed binary file: %v", err)
	}

	_, err := Apply(Request{Path: target})
	if err == nil || !strings.Contains(err.Error(), "text file") {
		t.Fatalf("expected text file error, got %v", err)
	}
}
