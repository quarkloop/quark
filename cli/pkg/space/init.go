// Package space provides space directory filesystem operations.
package space

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultQuarkfile is the template written by `quark init`.
const defaultQuarkfile = `# Quarkfile — space definition
# https://docs.quarkloop.io/reference/quarkfile
quark: "1.0"

# ── Meta ────────────────────────────────────────────────────────────────────
meta:
  name: my-space                   # used as the default instance name
  description: ""                  # human-readable description
  version: "0.1.0"                 # semver; shown in 'quark inspect'
  author: ""                       # your name or team name
  labels: {}                       # arbitrary key/value tags

# ── Model (optional) ─────────────────────────────────────────────────────────
# When absent the agent resolves from env vars or dynamic config.
# model:
#   provider: anthropic             # anthropic | openai | zhipu
#   name: claude-sonnet-4.6         # model name

# ── Model routing (optional) ─────────────────────────────────────────────────
# routing:
#   fallback:
#     - provider: openai
#       model: gpt-5.4
#   rules:
#     - match: "code_.*"
#       provider: anthropic
#       model: claude-sonnet-4.6

# ── Plugins ──────────────────────────────────────────────────────────────────
# Everything is a plugin: tools, agents, skills.
# Installed into .quark/plugins/{type}-{name}/ by 'quark plugin install'.
plugins:
  - ref: quark/tool-bash
  # - ref: quark/tool-read
  # - ref: quark/tool-write
  # - ref: quark/tool-web-search
  # - ref: quark/agent-researcher
  #   config:
  #     max_search_depth: 5
  # - ref: quark/skill-code-review

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
  approval_policy: auto         # required | auto

# ── Environment variables ─────────────────────────────────────────────────────
env:
  - ANTHROPIC_API_KEY
  # - OPENROUTER_API_KEY

# ── Model gateway (optional) ─────────────────────────────────────────────────
gateway:
  token_budget_per_hour: 100000
`

const defaultGitignore = `# Quark runtime artifacts
.quark/
`

func runInit(dir string) error {
	absDir, err := ensureAbs(dir)
	if err != nil {
		return err
	}

	// Create v2 directory structure.
	for _, d := range []string{
		".quark/sessions",
		".quark/config",
		".quark/activity",
		".quark/plans",
		".quark/kb",
		".quark/plugins",
	} {
		if err := os.MkdirAll(filepath.Join(absDir, d), 0755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}

	// Write files.
	files := map[string]string{
		"Quarkfile":  defaultQuarkfile,
		".gitignore": defaultGitignore,
	}
	for name, content := range files {
		path := filepath.Join(absDir, name)
		if _, err := os.Stat(path); err == nil {
			continue // never overwrite
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}

	return nil
}
