package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseServiceManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(`name: indexer
version: "1.0.0"
type: service
mode: api
description: Indexer service
service:
  address_env: QUARK_INDEXER_ADDR
  skill: SKILL.md
  proto_services:
    - quark.indexer.v1.IndexerService
`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if manifest.Type != TypeService {
		t.Fatalf("type = %s, want %s", manifest.Type, TypeService)
	}
	if manifest.Service == nil || manifest.Service.AddressEnv != "QUARK_INDEXER_ADDR" {
		t.Fatalf("service config = %+v", manifest.Service)
	}
}

func TestServiceManifestDefaultsSkill(t *testing.T) {
	manifest := &Manifest{
		Name:    "embedding",
		Version: "1.0.0",
		Type:    TypeService,
		Mode:    ModeAPI,
		Service: &ServiceConfig{},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if manifest.Service.Skill != "SKILL.md" {
		t.Fatalf("skill = %q, want SKILL.md", manifest.Service.Skill)
	}
}
