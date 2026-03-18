package repo

import (
	"fmt"

	"github.com/quarkloop/space/pkg/quarkfile"
)

func runAddSkill(dir, ref, name string) error {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return err
	}
	for _, s := range qf.Skills {
		if s.Name == name {
			return fmt.Errorf("skill %q already exists", name)
		}
	}
	qf.Skills = append(qf.Skills, quarkfile.Skill{Ref: ref, Name: name})
	return quarkfile.Save(dir, qf)
}

func runRemoveSkill(dir, name string) error {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return err
	}
	filtered := qf.Skills[:0]
	for _, s := range qf.Skills {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}
	qf.Skills = filtered
	return quarkfile.Save(dir, qf)
}

func runListSkills(dir string) ([]SkillEntry, error) {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]SkillEntry, len(qf.Skills))
	for i, s := range qf.Skills {
		entries[i] = SkillEntry{Name: s.Name, Ref: s.Ref}
	}
	return entries, nil
}
