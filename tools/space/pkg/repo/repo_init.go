package repo

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultQuarkfile is the template written by `quark init`.
// Every field is present with an inline comment so developers understand
// the schema without needing to consult external docs.
const defaultQuarkfile = `# Quarkfile — space definition
# https://docs.quarkloop.io/reference/quarkfile
quark: "1.0"

# ── Meta ────────────────────────────────────────────────────────────────────
meta:
  name: my-space                   # used as the default instance name in quark run
  description: ""                  # human-readable description
  version: "0.1.0"                 # semver; shown in 'quark inspect'
  author: ""                       # your name or team name
  labels: {}                       # arbitrary key/value tags

# ── Default model ────────────────────────────────────────────────────────────
# All agents inherit this unless they declare their own model.
model:
  provider: anthropic               # anthropic | openai | zhipu
  name: claude-opus-4-6             # model name as accepted by the provider
  # fallback:                       # optional fallback if the primary is unavailable
  #   provider: openai
  #   name: gpt-4o

# ── Supervisor agent ─────────────────────────────────────────────────────────
# Every space has exactly one supervisor.  It receives the master goal, creates
# the plan, and assigns sub-tasks to worker agents.
supervisor:
  agent: quark/supervisor@latest    # registry ref: <namespace>/<name>@<version>
  prompt: ./prompts/supervisor.txt  # system prompt file (relative to this file)

# ── Worker agents ────────────────────────────────────────────────────────────
# Optional list of specialist agents the supervisor can delegate to.
# Remove or leave empty for single-agent spaces.
agents: []
# agents:
#   - ref: quark/researcher@latest  # registry ref
#     name: researcher              # local alias used in supervisor plans
#     prompt: ./prompts/researcher.txt  # optional per-agent system prompt override
#
#   - ref: quark/coder@latest
#     name: coder

# ── Tools ────────────────────────────────────────────────────────────────────
# Tools are HTTP-dispatched capabilities that agents can invoke (shell
# execution, file I/O, web search, etc.). Each tool needs a name and an
# endpoint URL provided via config.
tools: []
# tools:
#   - ref: quark/bash
#     name: bash
#     config:
#       endpoint: "http://127.0.0.1:8091/run"
#
#   - ref: quark/web-search
#     name: web_search
#     config:
#       endpoint: "http://127.0.0.1:8090/search"

# ── Environment variables ─────────────────────────────────────────────────────
# Names of environment variables that will be read from the shell at 'quark run'
# time and forwarded to the space-runtime process.
# Values are never stored in this file; they must exist in your shell or .env file.
env:
  - ANTHROPIC_API_KEY              # required for the default Anthropic model

# ── Knowledge Base ────────────────────────────────────────────────────────────
# KB entries can be seeded from environment variables at startup.
kb:
  env: []
# kb:
#   env:
#     - key: notion_token           # KB key
#       from: NOTION_TOKEN          # environment variable to read the value from

# ── Model gateway ────────────────────────────────────────────────────────────
# Rate-limit guard: cap total tokens consumed by all agents in this space.
model_gateway:
  token_budget_per_hour: 100000    # 0 = unlimited

# ── Network ──────────────────────────────────────────────────────────────────
# Ports the space-runtime exposes (e.g. for an agent that runs an HTTP server).
network:
  expose: []
# network:
#   expose:
#     - port: 8080
#       protocol: tcp
#       description: "agent web UI"

# ── Restart policy ───────────────────────────────────────────────────────────
# Controls what the api-server does when the space-runtime process exits.
#   on-failure  restart only on non-zero exit (default)
#   always      restart on any exit, including clean shutdown
#   never       do not restart; leave the space in stopped/failed state
restart: on-failure
`

const defaultSupervisorPrompt = `You are the supervisor agent for this space.

Your responsibilities:
1. Understand the master goal provided by the user.
2. Break it down into a concrete plan with clearly scoped sub-tasks.
3. Assign each sub-task to the most suitable worker agent.
4. Synthesise results and report progress back to the user.

Be concise in your plans. Prefer parallel sub-tasks where possible.
`

const defaultGitignore = `# Quark runtime artifacts
.quark/
`

const defaultLockStub = `quark: "1.0"
agents: []
tools: []
`

func runInit(dir string) error {
	absDir, err := ensureAbs(dir)
	if err != nil {
		return err
	}
	for _, d := range []string{
		"prompts", "agents",
		"kb/plans", "kb/memory", "kb/documents",
		"kb/config", "kb/notes", "kb/artifacts",
		".quark",
	} {
		if err := os.MkdirAll(filepath.Join(absDir, d), 0755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	files := map[string]string{
		"Quarkfile":              defaultQuarkfile,
		"prompts/supervisor.txt": defaultSupervisorPrompt,
		".gitignore":             defaultGitignore,
		".quark/lock.yaml":       defaultLockStub,
	}
	for name, content := range files {
		path := filepath.Join(absDir, name)
		if _, err := os.Stat(path); err == nil {
			continue // never overwrite existing files
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
}
