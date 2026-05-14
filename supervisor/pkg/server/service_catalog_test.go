package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/supervisor/pkg/pluginmanager"
)

func TestRuntimePluginCatalogEntryIncludesToolSchemaAndSkill(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("Use the tool carefully.\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	entry := runtimePluginCatalogEntryFromInstalled(pluginmanager.InstalledPlugin{
		Path: dir,
		Manifest: &plugin.Manifest{
			Name: "fs",
			Type: plugin.TypeTool,
			Tool: &plugin.ToolConfig{
				Schema: plugin.ToolSchema{
					Name:        "fs",
					Description: "filesystem",
				},
			},
		},
	})

	if entry.Name != "fs" || entry.Type != string(plugin.TypeTool) || entry.Path != dir {
		t.Fatalf("unexpected entry identity: %+v", entry)
	}
	if entry.Schema == nil || entry.Schema.Name != "fs" {
		t.Fatalf("tool schema missing: %+v", entry)
	}
	if entry.Skill != "Use the tool carefully." {
		t.Fatalf("skill = %q", entry.Skill)
	}
}
