package quarkfile

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Validate performs semantic validation of qf:
//   - required fields: quark, meta.name, supervisor.agent
//   - model.provider (if present) must be one of: anthropic, openai, zhipu, noop
//   - restart policy must be one of: on-failure, always, never (or empty string)
//   - every declared prompt file must exist on disk relative to dir
//   - every agent entry must have both a ref and a name
//   - every tool entry must have both a ref and a name
//   - permissions.network.deny (if present) must be valid CIDR or hostname
//   - permissions.audit.retention_days (if set) must be > 0
//
// Returns the first error encountered; nil when qf is fully valid.
func Validate(dir string, qf *Quarkfile) error {
	if qf.Quark == "" {
		return fmt.Errorf("missing required field: quark version")
	}
	if qf.Meta.Name == "" {
		return fmt.Errorf("missing required field: meta.name")
	}
	if qf.Supervisor.Agent == "" {
		return fmt.Errorf("missing required field: supervisor.agent")
	}

	// Model is optional — if present, validate it.
	if qf.Model.Provider != "" || qf.Model.Name != "" {
		if qf.Model.Provider == "" {
			return fmt.Errorf("model section present but missing provider")
		}
		if qf.Model.Name == "" {
			return fmt.Errorf("model section present but missing name")
		}
		validProviders := map[string]bool{"anthropic": true, "openai": true, "zhipu": true, "noop": true}
		if !validProviders[qf.Model.Provider] {
			return fmt.Errorf("invalid model provider %q (supported: anthropic, openai, zhipu, noop)", qf.Model.Provider)
		}
	}

	validRestart := map[string]bool{"on-failure": true, "always": true, "never": true, "": true}
	if !validRestart[qf.Restart] {
		return fmt.Errorf("invalid restart policy %q (supported: on-failure, always, never)", qf.Restart)
	}

	if qf.Supervisor.Prompt != "" {
		if err := fileExists(dir, qf.Supervisor.Prompt); err != nil {
			return fmt.Errorf("supervisor prompt: %w", err)
		}
	}
	for _, a := range qf.Agents {
		if a.Ref == "" {
			return fmt.Errorf("agent %q missing ref", a.Name)
		}
		if a.Name == "" {
			return fmt.Errorf("agent missing name (ref: %s)", a.Ref)
		}
		if a.Prompt != "" {
			if err := fileExists(dir, a.Prompt); err != nil {
				return fmt.Errorf("agent %q prompt: %w", a.Name, err)
			}
		}
	}
	for _, t := range qf.Tools {
		if t.Ref == "" {
			return fmt.Errorf("tool %q missing ref", t.Name)
		}
		if t.Name == "" {
			return fmt.Errorf("tool missing name (ref: %s)", t.Ref)
		}
	}

	// Validate permissions
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
			// Not CIDR — check if it's a valid hostname or IP
			if net.ParseIP(entry) == nil && !isValidHostname(entry) {
				return fmt.Errorf("permissions.network.deny: invalid CIDR, IP, or hostname %q", entry)
			}
		}
	}

	// Audit retention days must be > 0 if set
	if perms.Audit.RetentionDays < 0 {
		return fmt.Errorf("permissions.audit.retention_days must be positive, got %d", perms.Audit.RetentionDays)
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

// fileExists returns an error when the file at filepath.Join(dir, rel) does not exist.
func fileExists(dir, rel string) error {
	if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
		return fmt.Errorf("%s not found", rel)
	}
	return nil
}
