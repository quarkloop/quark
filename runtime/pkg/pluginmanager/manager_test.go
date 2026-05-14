package pluginmanager

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeWithoutCatalogDoesNotScanFilesystem(t *testing.T) {
	pluginsDir := t.TempDir()
	toolDir := filepath.Join(pluginsDir, "tools", "fake")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir tool: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "manifest.yaml"), []byte(`name: fake
version: "1.0.0"
type: tool
mode: api
description: fake
tool:
  schema:
    name: fake
    description: fake
    parameters:
      type: object
`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	m := NewManager(pluginsDir)
	if err := m.Initialize(context.Background()); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if tools := m.GetTools(); len(tools) != 0 {
		t.Fatalf("expected no filesystem-discovered tools, got %+v", tools)
	}
}

func TestNormalizeToolResponseUnwrapsToolkitOutput(t *testing.T) {
	got := normalizeToolResponse([]byte(`{"data":{"output":"quark-ok\n","exit_code":0},"error":""}`))
	want := `{"output":"quark-ok\n","exit_code":0}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestNormalizeToolResponseKeepsCustomPayload(t *testing.T) {
	got := normalizeToolResponse([]byte(`{"results":[{"title":"ok"}]}`))
	want := `{"results":[{"title":"ok"}]}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestNormalizeToolResponseConvertsToolkitError(t *testing.T) {
	got := normalizeToolResponse([]byte(`{"data":null,"error":"nope"}`))
	want := `{"error":"nope","is_error":true}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
