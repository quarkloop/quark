package quarkfile

// Merge deep-merges a parent Quarkfile into a child.
// Child fields take precedence. Agents and skills are additive.
func Merge(parent, child *Quarkfile) *Quarkfile {
	result := *parent

	if child.Meta.Name != "" {
		result.Meta = child.Meta
	}
	if child.Model.Name != "" {
		result.Model = child.Model
	}
	if child.Supervisor.Agent != "" {
		result.Supervisor = child.Supervisor
	}

	// Agents are additive — child entries with new names are appended
	seenAgents := map[string]bool{}
	for _, a := range parent.Agents {
		seenAgents[a.Name] = true
	}
	result.Agents = append([]Agent{}, parent.Agents...)
	for _, a := range child.Agents {
		if !seenAgents[a.Name] {
			result.Agents = append(result.Agents, a)
		}
	}

	// Skills are additive — child entries with new names are appended
	seenSkills := map[string]bool{}
	for _, s := range parent.Skills {
		seenSkills[s.Name] = true
	}
	result.Skills = append([]Skill{}, parent.Skills...)
	for _, s := range child.Skills {
		if !seenSkills[s.Name] {
			result.Skills = append(result.Skills, s)
		}
	}

	if len(child.Env) > 0 {
		result.Env = child.Env
	}
	if len(child.KB.Env) > 0 {
		result.KB = child.KB
	}
	if child.ModelGateway.TokenBudgetPerHour > 0 {
		result.ModelGateway = child.ModelGateway
	}
	if child.Restart != "" {
		result.Restart = child.Restart
	}
	return &result
}
