// Package quarkfile loads and saves the Quarkfile: the space declaration
// containing identity, model selection, plugin list, permissions, and
// capability boundaries.
//
// Usage:
//
//	qf, err := quarkfile.Load(dir)
//	if err := quarkfile.Validate(dir, qf); err != nil { … }
package quarkfile

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// QuarkfileFilename is the name of the space manifest file.
const QuarkfileFilename = "Quarkfile"

// Quarkfile is the parsed, in-memory representation of the Quarkfile on disk.
type Quarkfile struct {
	Quark        string         `yaml:"quark"`
	Meta         Meta           `yaml:"meta"`
	Model        Model          `yaml:"model,omitempty"`
	Routing      RoutingSection `yaml:"routing,omitempty"`
	Plugins      []PluginRef    `yaml:"plugins"`
	Env          []string       `yaml:"env,omitempty"`
	Permissions  Permissions    `yaml:"permissions,omitempty"`
	Capabilities Capabilities   `yaml:"capabilities,omitempty"`
	Gateway      Gateway        `yaml:"gateway,omitempty"`
	Execution    Execution      `yaml:"execution,omitempty"`
}

// Execution configures the agent's execution mode and workflow definition.
type Execution struct {
	// Mode specifies how the agent operates: autonomous, assistive, or workflow.
	// - autonomous: Agent executes tools without human approval (default).
	// - assistive: Agent requires human approval before tool execution (HITL).
	// - workflow: Agent executes a predefined DAG of steps.
	Mode string `yaml:"mode,omitempty"`

	// DAG defines the workflow steps for workflow mode.
	// Each step can depend on other steps, enabling parallel execution.
	DAG []DAGStep `yaml:"dag,omitempty"`

	// ApprovalTimeout is the maximum time to wait for human approval in assistive mode.
	// Format: Go duration string (e.g., "5m", "1h"). Default: "24h".
	ApprovalTimeout string `yaml:"approval_timeout,omitempty"`
}

// DAGStep is a single step in a workflow DAG.
type DAGStep struct {
	// ID is the unique identifier for this step.
	ID string `yaml:"id"`

	// Name is a human-readable description of the step.
	Name string `yaml:"name"`

	// Action is the prompt or command to execute for this step.
	Action string `yaml:"action"`

	// DependsOn lists the IDs of steps that must complete before this step runs.
	DependsOn []string `yaml:"depends_on,omitempty"`

	// Timeout is the maximum duration for this step. Format: Go duration string.
	Timeout string `yaml:"timeout,omitempty"`

	// RetryCount is the number of times to retry on failure. Default: 0.
	RetryCount int `yaml:"retry_count,omitempty"`
}

// Meta holds human-readable identity for the space.
type Meta struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Version     string            `yaml:"version,omitempty"`
	Author      string            `yaml:"author,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// Model specifies the LLM provider and model name.
type Model struct {
	Provider string `yaml:"provider"`
	Name     string `yaml:"name"`
}

// RoutingSection configures model routing rules and fallback chain.
type RoutingSection struct {
	Rules    []RoutingRuleEntry `yaml:"rules,omitempty"`
	Fallback []ModelRef         `yaml:"fallback,omitempty"`
}

// RoutingRuleEntry maps a regex pattern to a target model.
type RoutingRuleEntry struct {
	Match    string `yaml:"match"`
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// ModelRef specifies a provider and model name.
type ModelRef struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// PluginRef declares a plugin dependency with optional per-plugin config.
type PluginRef struct {
	Ref    string         `yaml:"ref"`
	Config map[string]any `yaml:"config,omitempty"`
}

// Permissions defines policy constraints enforced by the agent runtime.
type Permissions struct {
	Filesystem FilesystemPermissions `yaml:"filesystem,omitempty"`
	Network    NetworkPermissions    `yaml:"network,omitempty"`
	Tools      ToolPermissions       `yaml:"tools,omitempty"`
	Audit      AuditPermissions      `yaml:"audit,omitempty"`
}

// FilesystemPermissions controls which paths the agent can access.
type FilesystemPermissions struct {
	AllowedPaths []string `yaml:"allowed_paths,omitempty"`
	ReadOnly     []string `yaml:"read_only,omitempty"`
}

// NetworkPermissions controls which hosts the agent can reach.
type NetworkPermissions struct {
	AllowedHosts []string `yaml:"allowed_hosts,omitempty"`
	Deny         []string `yaml:"deny,omitempty"`
}

// ToolPermissions controls which tools the agent can invoke.
type ToolPermissions struct {
	Allowed []string `yaml:"allowed,omitempty"`
	Denied  []string `yaml:"denied,omitempty"`
}

// AuditPermissions controls logging and retention policies.
type AuditPermissions struct {
	LogToolCalls    bool `yaml:"log_tool_calls"`
	LogLLMResponses bool `yaml:"log_llm_responses"`
	RetentionDays   int  `yaml:"retention_days,omitempty"`
}

// Capabilities declares what agents in this space are allowed to do.
type Capabilities struct {
	SpawnAgents    bool   `yaml:"spawn_agents"`
	MaxWorkers     int    `yaml:"max_workers,omitempty"`
	CreatePlans    bool   `yaml:"create_plans"`
	ApprovalPolicy string `yaml:"approval_policy,omitempty"`
}

// Gateway configures model gateway resource limits.
type Gateway struct {
	TokenBudgetPerHour int `yaml:"token_budget_per_hour,omitempty"`
}

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

// Save marshals qf to YAML and writes it to <dir>/Quarkfile.
func Save(dir string, qf *Quarkfile) error {
	path := filepath.Join(dir, QuarkfileFilename)
	data, err := yaml.Marshal(qf)
	if err != nil {
		return fmt.Errorf("marshaling Quarkfile: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Exists reports whether a Quarkfile is present in dir.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, QuarkfileFilename))
	return err == nil
}
