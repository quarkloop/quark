package quarkfile

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"time"
)

// Validate performs semantic validation of qf against the v2 schema:
//   - required fields: quark, meta.name
//   - plugins list must be non-empty
//   - each plugin ref must be non-empty
//   - model.provider (if present) must be one of the supported set
//   - routing.rules[].match (if present) must be valid regex
//   - gateway.token_budget_per_hour (if set) must be >= 0
//   - permissions.network.deny entries must be valid CIDR or hostname
//   - permissions.audit.retention_days (if set) must be >= 0
func Validate(dir string, qf *Quarkfile) error {
	_ = dir
	if qf.Quark == "" {
		return fmt.Errorf("missing required field: quark version")
	}
	if qf.Meta.Name == "" {
		return fmt.Errorf("missing required field: meta.name")
	}
	if len(qf.Plugins) == 0 {
		return fmt.Errorf("missing required field: plugins (must have at least one)")
	}
	for i, p := range qf.Plugins {
		if p.Ref == "" {
			return fmt.Errorf("plugins[%d]: missing ref", i)
		}
	}

	// Model is optional — if present, validate both fields.
	if qf.Model.Provider != "" || qf.Model.Name != "" {
		if qf.Model.Provider == "" {
			return fmt.Errorf("model section present but missing provider")
		}
		if qf.Model.Name == "" {
			return fmt.Errorf("model section present but missing name")
		}
		validProviders := map[string]bool{"anthropic": true, "openai": true, "openrouter": true, "zhipu": true, "noop": true}
		if !validProviders[qf.Model.Provider] {
			return fmt.Errorf("invalid model provider %q (supported: anthropic, openai, zhipu, noop)", qf.Model.Provider)
		}
	}

	// Routing rules: validate regex patterns.
	for i, rule := range qf.Routing.Rules {
		if rule.Match == "" {
			return fmt.Errorf("routing.rules[%d]: missing match pattern", i)
		}
		if _, err := regexp.Compile(rule.Match); err != nil {
			return fmt.Errorf("routing.rules[%d]: invalid regex %q: %w", i, rule.Match, err)
		}
		if rule.Provider == "" || rule.Model == "" {
			return fmt.Errorf("routing.rules[%d]: missing provider or model", i)
		}
	}

	// Gateway: token budget must be non-negative.
	if qf.Gateway.TokenBudgetPerHour < 0 {
		return fmt.Errorf("gateway.token_budget_per_hour must be >= 0, got %d", qf.Gateway.TokenBudgetPerHour)
	}

	// Permissions
	if err := validatePermissions(qf); err != nil {
		return err
	}

	// Execution
	if err := validateExecution(qf); err != nil {
		return err
	}

	return nil
}

func validatePermissions(qf *Quarkfile) error {
	perms := qf.Permissions

	// Network deny entries must be valid CIDR or hostname
	for _, entry := range perms.Network.Deny {
		if _, _, err := net.ParseCIDR(entry); err != nil {
			if net.ParseIP(entry) == nil && !isValidHostname(entry) {
				return fmt.Errorf("permissions.network.deny: invalid CIDR, IP, or hostname %q", entry)
			}
		}
	}

	// Audit retention days must be >= 0 if set
	if perms.Audit.RetentionDays < 0 {
		return fmt.Errorf("permissions.audit.retention_days must be >= 0, got %d", perms.Audit.RetentionDays)
	}

	return nil
}

func isValidHostname(s string) bool {
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	for _, label := range filepath.SplitList(s) {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
	}
	return true
}

// validateExecution validates the execution configuration.
func validateExecution(qf *Quarkfile) error {
	exec := qf.Execution

	// Mode must be one of the valid values (empty defaults to autonomous)
	if exec.Mode != "" {
		validModes := map[string]bool{"autonomous": true, "assistive": true, "workflow": true}
		if !validModes[exec.Mode] {
			return fmt.Errorf("execution.mode must be one of: autonomous, assistive, workflow; got %q", exec.Mode)
		}
	}

	// ApprovalTimeout must be a valid duration if set
	if exec.ApprovalTimeout != "" {
		if _, err := time.ParseDuration(exec.ApprovalTimeout); err != nil {
			return fmt.Errorf("execution.approval_timeout is not a valid duration: %w", err)
		}
	}

	// DAG validation
	if len(exec.DAG) > 0 {
		if exec.Mode != "" && exec.Mode != "workflow" {
			return fmt.Errorf("execution.dag is only valid when mode is 'workflow', got mode=%q", exec.Mode)
		}

		// Validate DAG steps
		stepIDs := make(map[string]bool)
		for i, step := range exec.DAG {
			if step.ID == "" {
				return fmt.Errorf("execution.dag[%d]: missing required field 'id'", i)
			}
			if stepIDs[step.ID] {
				return fmt.Errorf("execution.dag[%d]: duplicate step id %q", i, step.ID)
			}
			stepIDs[step.ID] = true

			if step.Name == "" {
				return fmt.Errorf("execution.dag[%d]: missing required field 'name'", i)
			}
			if step.Action == "" {
				return fmt.Errorf("execution.dag[%d]: missing required field 'action'", i)
			}

			// Validate timeout if set
			if step.Timeout != "" {
				if _, err := time.ParseDuration(step.Timeout); err != nil {
					return fmt.Errorf("execution.dag[%d].timeout is not a valid duration: %w", i, err)
				}
			}

			// RetryCount must be non-negative
			if step.RetryCount < 0 {
				return fmt.Errorf("execution.dag[%d].retry_count must be >= 0, got %d", i, step.RetryCount)
			}
		}

		// Validate dependencies reference existing steps
		for i, step := range exec.DAG {
			for _, dep := range step.DependsOn {
				if !stepIDs[dep] {
					return fmt.Errorf("execution.dag[%d]: depends_on references unknown step %q", i, dep)
				}
				if dep == step.ID {
					return fmt.Errorf("execution.dag[%d]: step cannot depend on itself", i)
				}
			}
		}

		// Detect cycles in DAG
		if err := detectCycles(exec.DAG); err != nil {
			return err
		}
	}

	return nil
}

// detectCycles checks for circular dependencies in the DAG using DFS.
func detectCycles(steps []DAGStep) error {
	// Build adjacency list
	adj := make(map[string][]string)
	for _, step := range steps {
		adj[step.ID] = step.DependsOn
	}

	// Track visit state: 0=unvisited, 1=visiting, 2=visited
	state := make(map[string]int)

	var visit func(id string, path []string) error
	visit = func(id string, path []string) error {
		if state[id] == 2 {
			return nil // already fully processed
		}
		if state[id] == 1 {
			// Found cycle — build error message
			cycleStart := -1
			for i, p := range path {
				if p == id {
					cycleStart = i
					break
				}
			}
			cycle := append(path[cycleStart:], id)
			return fmt.Errorf("execution.dag: circular dependency detected: %v", cycle)
		}

		state[id] = 1
		path = append(path, id)
		for _, dep := range adj[id] {
			if err := visit(dep, path); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}

	for _, step := range steps {
		if state[step.ID] == 0 {
			if err := visit(step.ID, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
