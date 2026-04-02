// Package skill provides skill discovery, parsing, and cascade resolution.
//
// Skills are SKILL.md markdown files with YAML frontmatter that provide
// domain knowledge and advisory guidance to the agent. They are discovered
// from multiple roots (space > plugin > builtin) and resolved by priority.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Source identifies where a skill was loaded from.
type Source string

const (
	SourceSpace   Source = "space"   // priority 3 (highest)
	SourcePlugin  Source = "plugin"  // priority 2
	SourceBuiltin Source = "builtin" // priority 1 (lowest)
)

// priority returns the numeric priority for a source.
func (s Source) priority() int {
	switch s {
	case SourceSpace:
		return 3
	case SourcePlugin:
		return 2
	case SourceBuiltin:
		return 1
	default:
		return 0
	}
}

// Skill represents a parsed SKILL.md file.
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Version     string   `yaml:"version"`
	Tools       []string `yaml:"tools"`
	Triggers    []string `yaml:"triggers"`
	Content     string   // markdown body (after frontmatter)
	Source      Source   // where this skill was loaded from
	Priority    int      // computed from source
}

// Root is a directory to scan for skills.
type Root struct {
	Dir      string
	Source   Source
	Priority int
}

// Resolver discovers skills from multiple roots and resolves by priority.
type Resolver struct {
	roots  []Root
	skills map[string]*Skill // name → highest-priority skill
}

// NewResolver creates a resolver with the given roots.
// Roots should be ordered by priority (highest first).
func NewResolver(roots ...Root) *Resolver {
	r := &Resolver{
		roots:  roots,
		skills: make(map[string]*Skill),
	}
	// Pre-resolve so Resolve() is fast.
	r.Resolve()
	return r
}

// Resolve scans all roots and returns the deduplicated skill set.
// Same-named skills at higher priority shadow lower ones.
func (r *Resolver) Resolve() ([]*Skill, error) {
	r.skills = make(map[string]*Skill)

	for _, root := range r.roots {
		if root.Dir == "" {
			continue
		}
		entries, err := os.ReadDir(root.Dir)
		if err != nil {
			// Root doesn't exist or isn't readable — skip silently.
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillPath := filepath.Join(root.Dir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillPath); err != nil {
				continue
			}
			skill, err := Parse(skillPath)
			if err != nil {
				continue
			}
			skill.Source = root.Source
			skill.Priority = root.Priority
			if root.Priority == 0 {
				skill.Priority = root.Source.priority()
			}

			// Shadow: only replace if higher priority.
			if existing, ok := r.skills[skill.Name]; !ok || skill.Priority > existing.Priority {
				r.skills[skill.Name] = skill
			}
		}
	}

	result := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	return result, nil
}

// ResolveByName returns the highest-priority skill with the given name.
func (r *Resolver) ResolveByName(name string) (*Skill, error) {
	s, ok := r.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return s, nil
}

// ResolveByTrigger returns skills whose triggers match the given text.
// Simple case-insensitive substring matching.
func (r *Resolver) ResolveByTrigger(text string) []*Skill {
	lower := strings.ToLower(text)
	var matched []*Skill
	for _, s := range r.skills {
		for _, trigger := range s.Triggers {
			if strings.Contains(lower, strings.ToLower(trigger)) {
				matched = append(matched, s)
				break
			}
		}
	}
	return matched
}

// Parse reads a SKILL.md file and returns a Skill.
func Parse(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %s: %w", path, err)
	}

	content := string(data)
	// Parse YAML frontmatter between --- delimiters.
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("skill %s missing YAML frontmatter", path)
	}

	endIdx := strings.Index(content[3:], "---")
	if endIdx < 0 {
		return nil, fmt.Errorf("skill %s has unclosed frontmatter", path)
	}
	endIdx += 3 // adjust for the offset

	frontmatter := content[3:endIdx]
	body := strings.TrimSpace(content[endIdx+3:])

	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, fmt.Errorf("parse skill frontmatter %s: %w", path, err)
	}
	skill.Content = body

	if skill.Name == "" {
		// Derive name from directory.
		skill.Name = strings.ToLower(filepath.Base(filepath.Dir(path)))
	}

	return &skill, nil
}
