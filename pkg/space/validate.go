package space

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// ValidateQuarkfile performs semantic validation of qf.
func ValidateQuarkfile(qf *Quarkfile) error {
	if qf == nil {
		return fmt.Errorf("quarkfile is nil")
	}
	if qf.SchemaVersion() == "" {
		return fmt.Errorf("missing required field: quark")
	}
	if qf.Meta.Name == "" {
		return fmt.Errorf("missing required field: meta.name")
	}
	if err := ValidateName(qf.Meta.Name); err != nil {
		return fmt.Errorf("meta.name: %w", err)
	}
	if qf.Meta.Version == "" {
		return fmt.Errorf("missing required field: meta.version")
	}
	if len(qf.Plugins) == 0 {
		return fmt.Errorf("missing required field: plugins (must have at least one)")
	}
	for i, p := range qf.Plugins {
		if p.Ref == "" {
			return fmt.Errorf("plugins[%d]: missing ref", i)
		}
	}

	if err := validateModel(qf.Model); err != nil {
		return err
	}
	if err := validateEnvVars(qf.EnvironmentVariables()); err != nil {
		return err
	}

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

	if qf.Gateway.TokenBudgetPerHour < 0 {
		return fmt.Errorf("gateway.token_budget_per_hour must be >= 0, got %d", qf.Gateway.TokenBudgetPerHour)
	}

	return validatePermissions(qf)
}

func validateModel(model Model) error {
	if model.IsZero() {
		return nil
	}
	if model.Provider == "" {
		return fmt.Errorf("model section present but missing provider")
	}
	if model.Name == "" {
		return fmt.Errorf("model section present but missing name")
	}
	validProviders := map[string]bool{"anthropic": true, "openai": true, "openrouter": true, "zhipu": true, "noop": true}
	if !validProviders[model.Provider] {
		return fmt.Errorf("invalid model provider %q (supported: anthropic, openai, openrouter, zhipu, noop)", model.Provider)
	}
	return nil
}

func validateEnvVars(names []string) error {
	envName := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	for _, name := range names {
		if !envName.MatchString(name) {
			return fmt.Errorf("env: invalid environment variable name %q", name)
		}
		if strings.HasPrefix(name, "QUARK_") {
			return fmt.Errorf("env: %s is reserved for quark runtime variables", name)
		}
	}
	return nil
}

func validatePermissions(qf *Quarkfile) error {
	perms := qf.Permissions
	for _, entry := range perms.Network.Deny {
		if _, _, err := net.ParseCIDR(entry); err != nil {
			if net.ParseIP(entry) == nil && !isValidHostname(entry) {
				return fmt.Errorf("permissions.network.deny: invalid CIDR, IP, or hostname %q", entry)
			}
		}
	}
	if perms.Audit.RetentionDays < 0 {
		return fmt.Errorf("permissions.audit.retention_days must be >= 0, got %d", perms.Audit.RetentionDays)
	}
	return nil
}

func isValidHostname(s string) bool {
	s = strings.TrimSuffix(s, ".")
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	labelPattern := regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	for _, label := range strings.Split(s, ".") {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if !labelPattern.MatchString(label) {
			return false
		}
	}
	return true
}
