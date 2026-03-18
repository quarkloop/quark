package repo

import (
	"fmt"

	"github.com/quarkloop/space/pkg/quarkfile"
)

func runValidate(dir string) error {
	absDir, err := ensureAbs(dir)
	if err != nil {
		return err
	}

	qf, err := quarkfile.Load(absDir)
	if err != nil {
		return err
	}
	if err := quarkfile.Validate(absDir, qf); err != nil {
		return fmt.Errorf("Quarkfile invalid: %w", err)
	}

	if !quarkfile.LockExists(absDir) {
		return fmt.Errorf("lock file missing — run 'quark lock'")
	}
	lf, err := quarkfile.LoadLock(absDir)
	if err != nil {
		return err
	}

	// Check that all agents in Quarkfile are present in lock file
	lockedAgents := map[string]bool{}
	for _, a := range lf.Agents {
		lockedAgents[a.Ref] = true
	}
	if !lockedAgents[qf.Supervisor.Agent] {
		return fmt.Errorf("supervisor agent %q not in lock file — run 'quark lock'", qf.Supervisor.Agent)
	}
	for _, a := range qf.Agents {
		if !lockedAgents[a.Ref] {
			return fmt.Errorf("agent %q not in lock file — run 'quark lock'", a.Ref)
		}
	}

	// Check that all skills in Quarkfile are present in lock file
	lockedSkills := map[string]bool{}
	for _, s := range lf.Skills {
		lockedSkills[s.Ref] = true
	}
	for _, s := range qf.Skills {
		if !lockedSkills[s.Ref] {
			return fmt.Errorf("skill %q not in lock file — run 'quark lock'", s.Ref)
		}
	}

	return nil
}
