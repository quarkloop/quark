package repo

import (
	"fmt"
	"time"

	"github.com/quarkloop/tools/space/pkg/registry"
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

	// Resolve supervisor agent
	sup, err := registry.LockAgent(qf.Supervisor.Agent)
	if err != nil {
		return fmt.Errorf("resolving supervisor: %w", err)
	}
	lf.Agents = append(lf.Agents, *sup)

	// Resolve worker agents
	for _, a := range qf.Agents {
		locked, err := registry.LockAgent(a.Ref)
		if err != nil {
			return fmt.Errorf("resolving agent %s: %w", a.Name, err)
		}
		lf.Agents = append(lf.Agents, *locked)
	}

	// Resolve skills
	for _, s := range qf.Skills {
		locked, err := registry.LockSkill(s.Ref)
		if err != nil {
			return fmt.Errorf("resolving skill %s: %w", s.Name, err)
		}
		lf.Skills = append(lf.Skills, *locked)
	}

	return quarkfile.SaveLock(absDir, lf)
}
