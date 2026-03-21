package repo

import (
	"fmt"

	"github.com/quarkloop/tools/space/pkg/quarkfile"
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

	// Check that all tools in Quarkfile are present in lock file
	lockedTools := map[string]bool{}
	for _, t := range lf.Tools {
		lockedTools[t.Ref] = true
	}
	for _, t := range qf.Tools {
		if !lockedTools[t.Ref] {
			return fmt.Errorf("tool %q not in lock file — run 'quark lock'", t.Ref)
		}
	}

	return nil
}
