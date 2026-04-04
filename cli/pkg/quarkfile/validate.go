package quarkfile

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
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
