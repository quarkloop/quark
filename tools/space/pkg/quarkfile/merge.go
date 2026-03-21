package quarkfile

// Merge deep-merges a parent Quarkfile into a child.
// Child fields take precedence. Agents and tools are additive.
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

	// Tools are additive — child entries with new names are appended
	seenTools := map[string]bool{}
	for _, t := range parent.Tools {
		seenTools[t.Name] = true
	}
	result.Tools = append([]Tool{}, parent.Tools...)
	for _, t := range child.Tools {
		if !seenTools[t.Name] {
			result.Tools = append(result.Tools, t)
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
