package repo

import (
	"fmt"

	"github.com/quarkloop/tools/space/pkg/quarkfile"
)

func runAddTool(dir, ref, name string) error {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return err
	}
	for _, t := range qf.Tools {
		if t.Name == name {
			return fmt.Errorf("tool %q already exists", name)
		}
	}
	qf.Tools = append(qf.Tools, quarkfile.Tool{Ref: ref, Name: name})
	return quarkfile.Save(dir, qf)
}

func runRemoveTool(dir, name string) error {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return err
	}
	filtered := qf.Tools[:0]
	for _, t := range qf.Tools {
		if t.Name != name {
			filtered = append(filtered, t)
		}
	}
	qf.Tools = filtered
	return quarkfile.Save(dir, qf)
}

func runListTools(dir string) ([]ToolEntry, error) {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]ToolEntry, len(qf.Tools))
	for i, t := range qf.Tools {
		entries[i] = ToolEntry{Name: t.Name, Ref: t.Ref}
	}
	return entries, nil
}
