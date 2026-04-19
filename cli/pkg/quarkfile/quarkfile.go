// Package quarkfile is the CLI's minimal local Quarkfile handler. The CLI
// is permitted to read and write exactly one file on the user's machine:
// the Quarkfile in the current working directory. All other state lives in
// the supervisor.
package quarkfile

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Filename is the name of the manifest file inside the user's working dir.
const Filename = "Quarkfile"

// meta is the minimal shape the CLI needs to extract from the raw Quarkfile.
type meta struct {
	Meta struct {
		Name string `yaml:"name"`
	} `yaml:"meta"`
}

// Read returns the raw bytes of the Quarkfile located in dir.
func Read(dir string) ([]byte, error) {
	path := filepath.Join(dir, Filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Quarkfile: %w", err)
	}
	return data, nil
}

// Write writes raw Quarkfile bytes into dir, overwriting any existing file.
func Write(dir string, data []byte) error {
	path := filepath.Join(dir, Filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write Quarkfile: %w", err)
	}
	return nil
}

// Exists reports whether a Quarkfile is present in dir.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, Filename))
	return err == nil
}

// Name extracts meta.name from the raw Quarkfile bytes.
func Name(data []byte) (string, error) {
	var m meta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("parse Quarkfile: %w", err)
	}
	if m.Meta.Name == "" {
		return "", fmt.Errorf("Quarkfile missing meta.name")
	}
	return m.Meta.Name, nil
}

// NameFromDir reads the Quarkfile in dir and returns its meta.name.
func NameFromDir(dir string) (string, error) {
	data, err := Read(dir)
	if err != nil {
		return "", err
	}
	return Name(data)
}

// CurrentName returns the space name from the Quarkfile in the current
// working directory. It is the canonical helper for CLI commands that
// address the current space.
func CurrentName() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return NameFromDir(cwd)
}

// DefaultTemplate returns the default Quarkfile contents to scaffold a new
// space named name.
func DefaultTemplate(name string) []byte {
	const tmpl = `# Quarkfile — space definition
# https://quarkloop.com/docs/reference/quarkfile
quark: "1.0"

# ── Meta ────────────────────────────────────────────────────────────────────
meta:
  name: %s
  description: ""
  version: "0.1.0"
  author: ""
  labels: {}

# ── Plugins ──────────────────────────────────────────────────────────────────
plugins:
  - ref: quark/tool-bash

# ── Permissions ──────────────────────────────────────────────────────────────
permissions:
  filesystem:
    allowed_paths: ["."]
    read_only: ["Quarkfile"]
  network:
    allowed_hosts: ["*"]
    deny: ["169.254.0.0/16"]
  tools:
    allowed: ["*"]
    denied: []
  audit:
    log_tool_calls: true
    log_llm_responses: false
    retention_days: 30

# ── Capabilities ─────────────────────────────────────────────────────────────
capabilities:
  spawn_agents: true
  max_workers: 3
  create_plans: true
  approval_policy: auto

# ── Environment variables ────────────────────────────────────────────────────
env:
  - ANTHROPIC_API_KEY

# ── Model gateway (optional) ─────────────────────────────────────────────────
gateway:
  token_budget_per_hour: 100000
`
	return fmt.Appendf(nil, tmpl, name)
}
