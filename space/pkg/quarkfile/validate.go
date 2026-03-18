package quarkfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Validate runs structural checks on qf:
//
//   - Required fields: quark, meta.name, model.provider, model.name, supervisor.agent.
//   - model.provider allowlist: anthropic, openai, zhipu, noop.
//   - restart policy allowlist: on-failure, always, never (empty = default).
//   - Prompt file references must exist on disk relative to dir.
//   - Every agent/skill entry must have both ref and name populated.
// Validate performs semantic validation of qf:
//   - required fields: quark, meta.name, model.provider, model.name, supervisor.agent
//   - model.provider must be one of: anthropic, openai, zhipu, noop
//   - restart policy must be one of: on-failure, always, never (or empty string)
//   - every declared prompt file must exist on disk relative to dir
//   - every agent entry must have both a ref and a name
//   - every skill entry must have both a ref and a name
//
// Returns the first error encountered; nil when qf is fully valid.
func Validate(dir string, qf *Quarkfile) error {
	if qf.Quark == "" {
		return fmt.Errorf("missing required field: quark version")
	}
	if qf.Meta.Name == "" {
		return fmt.Errorf("missing required field: meta.name")
	}
	if qf.Model.Provider == "" || qf.Model.Name == "" {
		return fmt.Errorf("missing required fields: model.provider and model.name")
	}
	validProviders := map[string]bool{"anthropic": true, "openai": true, "zhipu": true, "noop": true}
	if !validProviders[qf.Model.Provider] {
		return fmt.Errorf("invalid model provider %q (supported: anthropic, openai, zhipu, noop)", qf.Model.Provider)
	}

	validRestart := map[string]bool{"on-failure": true, "always": true, "never": true, "": true}
	if !validRestart[qf.Restart] {
		return fmt.Errorf("invalid restart policy %q (supported: on-failure, always, never)", qf.Restart)
	}

	if qf.Supervisor.Agent == "" {
		return fmt.Errorf("missing required field: supervisor.agent")
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
	for _, s := range qf.Skills {
		if s.Ref == "" {
			return fmt.Errorf("skill %q missing ref", s.Name)
		}
		if s.Name == "" {
			return fmt.Errorf("skill missing name (ref: %s)", s.Ref)
		}
	}
	return nil
}

// fileExists returns an error when the file at filepath.Join(dir, rel) does not exist.
func fileExists(dir, rel string) error {
	if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
		return fmt.Errorf("%s not found", rel)
	}
	return nil
}
