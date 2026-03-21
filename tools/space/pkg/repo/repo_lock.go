package repo

import (
	"fmt"
	"time"

	"github.com/quarkloop/tools/space/pkg/quarkfile"
)

func runLock(dir string) error {
	absDir, err := ensureAbs(dir)
	if err != nil {
		return err
	}

	qf, err := quarkfile.Load(absDir)
	if err != nil {
		return err
	}

	now := time.Now()
	lf := &quarkfile.LockFile{
		Quark:      qf.Quark,
		ResolvedAt: &now,
	}

	// Record supervisor agent.
	lf.Agents = append(lf.Agents, quarkfile.LockedAgent{
		Ref:      qf.Supervisor.Agent,
		Resolved: qf.Supervisor.Agent,
	})

	// Record worker agents.
	for _, a := range qf.Agents {
		lf.Agents = append(lf.Agents, quarkfile.LockedAgent{
			Ref:      a.Ref,
			Resolved: a.Ref,
		})
	}

	// Record tools.
	for _, t := range qf.Tools {
		lf.Tools = append(lf.Tools, quarkfile.LockedTool{
			Ref:      t.Ref,
			Resolved: t.Ref,
		})
	}

	if err := quarkfile.SaveLock(absDir, lf); err != nil {
		return fmt.Errorf("writing lock file: %w", err)
	}

	fmt.Println("Lock file written \u2192 .quark/lock.yaml")
	return nil
}
