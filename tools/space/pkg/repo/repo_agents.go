package repo

import (
	"fmt"

	"github.com/quarkloop/tools/space/pkg/quarkfile"
)

func runAddAgent(dir, ref, name string) error {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return err
	}
	for _, a := range qf.Agents {
		if a.Name == name {
			return fmt.Errorf("agent %q already exists", name)
		}
	}
	qf.Agents = append(qf.Agents, quarkfile.Agent{Ref: ref, Name: name})
	return quarkfile.Save(dir, qf)
}

func runRemoveAgent(dir, name string) error {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return err
	}
	filtered := qf.Agents[:0]
	for _, a := range qf.Agents {
		if a.Name != name {
			filtered = append(filtered, a)
		}
	}
	qf.Agents = filtered
	return quarkfile.Save(dir, qf)
}

func runListAgents(dir string) ([]AgentEntry, error) {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]AgentEntry, len(qf.Agents))
	for i, a := range qf.Agents {
		entries[i] = AgentEntry{Name: a.Name, Ref: a.Ref}
	}
	return entries, nil
}
