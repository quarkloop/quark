package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quarkloop/pkg/toolkit"
)

func TestSchemaIncludesPDFExtraction(t *testing.T) {
	schema := (&Tool{}).Schema()
	props, ok := schema.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties missing: %#v", schema.Parameters)
	}
	command, ok := props["command"].(map[string]any)
	if !ok {
		t.Fatalf("command schema missing: %#v", props["command"])
	}
	enum, ok := command["enum"].([]string)
	if !ok {
		t.Fatalf("command enum missing: %#v", command["enum"])
	}
	for _, value := range enum {
		if value == "extract_pdf" {
			return
		}
	}
	t.Fatalf("command enum missing extract_pdf: %#v", enum)
}

func TestIntFlagAcceptsSchemaAlias(t *testing.T) {
	got, err := intFlag(map[string]any{"max_chars": float64(1200)}, "max-chars", defaultPDFMaxChars)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1200 {
		t.Fatalf("max chars = %d, want 1200", got)
	}
}

func TestListEntriesSupportsReadOnlyDirectoryHashAndTimestamps(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	file := filepath.Join(nested, "source.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Chmod(nested, 0o555); err != nil {
		t.Fatalf("chmod nested read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(nested, 0o755) })

	entries, err := listEntries(dir, true, true)
	if err != nil {
		t.Fatalf("list read-only directory: %v", err)
	}
	var found fileEntry
	for _, entry := range entries {
		if entry.RelativePath == "nested/source.txt" {
			found = entry
			break
		}
	}
	if found.RelativePath == "" {
		t.Fatalf("source file missing from recursive entries: %+v", entries)
	}
	if found.SHA256 == "" || found.Modified == "" || found.Path == "" {
		t.Fatalf("entry missing identity fields: %+v", found)
	}
}

func TestStatIncludesSHA256ForRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	out, err := handleStat(toolkitInput(path, true))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("stat returned error: %s", out.Error)
	}
	if out.Data["sha256"] == "" {
		t.Fatalf("stat missing sha256: %+v", out.Data)
	}
}

func toolkitInput(path string, includeHash bool) toolkit.Input {
	return toolkit.Input{
		Args:  map[string]string{"path": path},
		Flags: map[string]any{"include-hash": includeHash},
	}
}
