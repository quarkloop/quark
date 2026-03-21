// Package quarkfile loads, validates, and saves the two project manifest files
// that every quark space needs:
//
//   - Quarkfile       — human-authored project definition (meta, model, agents, tools).
//   - .quark/lock.yaml — machine-generated pin file produced by `quark lock`.
//
// Typical call sequence:
//
//	qf, err := quarkfile.Load(dir)
//	if err := quarkfile.Validate(dir, qf); err != nil { … }
//	lf, err := quarkfile.LoadLock(dir)
package quarkfile

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// QuarkfileFilename is the name of the project configuration file present at
// the root of every quark project.
const QuarkfileFilename = "Quarkfile"

// LockfileFilename is the path of the dependency lock file, relative to the
// project root. The lock file pins every agent and tool ref to an exact
// SHA-256 digest so builds are reproducible.
const LockfileFilename = ".quark/lock.yaml"

// Quarkfile is the parsed representation of the "Quarkfile" YAML.
// It defines space identity, model provider, supervisor + worker agents,
// tools, environment forwarding, KB configuration, and restart policy.
// Quarkfile is the parsed, in-memory representation of the Quarkfile on disk.
//
// It declares the space name, LLM model/provider, supervisor agent, optional
// worker agents, tools, environment variable forwarding, restart policy, and
// network port exposure. Obtain one via Load; validate it with Validate.
type Quarkfile struct {
	Quark        string       `yaml:"quark"`
	From         string       `yaml:"from,omitempty"`
	Meta         Meta         `yaml:"meta"`
	Model        Model        `yaml:"model"`
	Supervisor   Supervisor   `yaml:"supervisor"`
	Agents       []Agent      `yaml:"agents,omitempty"`
	Tools        []Tool       `yaml:"tools,omitempty"`
	Env          []string     `yaml:"env,omitempty"`
	KB           KBConfig     `yaml:"kb,omitempty"`
	ModelGateway ModelGateway `yaml:"model_gateway,omitempty"`
	Network      Network      `yaml:"network,omitempty"`
	Restart      string       `yaml:"restart,omitempty"`
}

// Meta holds human-readable identity fields for the space.
// Meta holds human-readable metadata about the space, used for display and
// tagging purposes only.
type Meta struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Version     string            `yaml:"version,omitempty"`
	Author      string            `yaml:"author,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// Model specifies the LLM provider and model name for the space.
// Supported providers: "anthropic", "openai", "zhipu", "noop".
// Model declares the LLM provider and model name used by the supervisor.
// Fallback is an optional chain entry used when the primary model is
// unavailable (not yet implemented in the gateway).
type Model struct {
	Provider string `yaml:"provider"`
	Name     string `yaml:"name"`
	Fallback *Model `yaml:"fallback,omitempty"`
}

// Supervisor configures the orchestrating agent that drives the
// ORIENT → PLAN → DISPATCH → MONITOR → ASSESS cycle.
// Supervisor describes the supervisor agent: the registry ref to resolve and
// an optional path to an override system-prompt file.
type Supervisor struct {
	Agent  string `yaml:"agent"`
	Prompt string `yaml:"prompt,omitempty"`
}

// Agent declares a worker agent the supervisor can dispatch plan steps to.
// Agent is a single entry in the agents list. It binds a registry ref to a
// local name and optionally overrides the agent's default system prompt.
type Agent struct {
	Ref    string `yaml:"ref"`
	Name   string `yaml:"name"`
	Prompt string `yaml:"prompt,omitempty"`
}

// Tool declares an HTTP-dispatched capability (e.g. web-search, code-exec)
// that agents may invoke as a tool call.
// Tool is a single entry in the tools list. It binds a registry ref to a
// local name and provides static config values passed to the HTTP dispatcher.
type Tool struct {
	Ref    string            `yaml:"ref"`
	Name   string            `yaml:"name"`
	Config map[string]string `yaml:"config,omitempty"`
}

// KBConfig configures knowledge-base initialisation from environment variables
// so secrets are injected at runtime without being stored in the Quarkfile.
// KBConfig describes environment-variable injection into the knowledge base
// at startup. Each entry reads an env var (From) and stores its value under
// the given KB key (Key).
type KBConfig struct {
	Env []KBEnvEntry `yaml:"env,omitempty"`
}

type KBEnvEntry struct {
	Key  string `yaml:"key"`
	From string `yaml:"from"`
}

// ModelGateway configures token-budget throttling for the model gateway.
// ModelGateway contains optional resource-limit settings applied to the
// model gateway (e.g. token budget per hour).
type ModelGateway struct {
	TokenBudgetPerHour int `yaml:"token_budget_per_hour,omitempty"`
}

// Network declares which ports the space exposes to the host.
// Network describes which ports the space exposes to the host machine.
type Network struct {
	Expose []PortExpose `yaml:"expose,omitempty"`
}

// PortExpose describes a single port the space makes accessible.
// PortExpose declares a single port forwarded from the space process to the
// host, useful for spaces that expose a web API or UI.
type PortExpose struct {
	Port        int    `yaml:"port"`
	Protocol    string `yaml:"protocol"`
	Description string `yaml:"description,omitempty"`
}

// Load reads and YAML-parses the Quarkfile inside dir.
// Returns an error when the file is absent or contains invalid YAML.
// Load reads and YAML-parses the Quarkfile in dir.
// Returns an error when the file is missing, unreadable, or malformed.
func Load(dir string) (*Quarkfile, error) {
	path := filepath.Join(dir, QuarkfileFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading Quarkfile: %w", err)
	}
	var qf Quarkfile
	if err := yaml.Unmarshal(data, &qf); err != nil {
		return nil, fmt.Errorf("parsing Quarkfile: %w", err)
	}
	return &qf, nil
}

// Save serialises qf to the Quarkfile inside dir, creating it if absent.
// Save marshals qf to YAML and writes it to <dir>/Quarkfile,
// creating or overwriting the file.
func Save(dir string, qf *Quarkfile) error {
	path := filepath.Join(dir, QuarkfileFilename)
	data, err := yaml.Marshal(qf)
	if err != nil {
		return fmt.Errorf("marshaling Quarkfile: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Exists reports whether a Quarkfile exists inside dir.
// Exists reports whether a Quarkfile is present in dir.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, QuarkfileFilename))
	return err == nil
}
