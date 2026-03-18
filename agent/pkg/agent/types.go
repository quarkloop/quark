package agent

// Definition is the resolved agent specification fetched from the registry.
type Definition struct {
	Ref            string       `json:"ref"`
	Name           string       `json:"name"`
	Version        string       `json:"version"`
	Digest         string       `json:"digest"`
	SystemPrompt   string       `json:"system_prompt"`
	Config         Config       `json:"config"`
	RequiredSkills []string     `json:"required_skills"`
	Capabilities   Capabilities `json:"capabilities"`
}

type Config struct {
	ContextWindow int    `json:"context_window"`
	Compaction    string `json:"compaction"`
	MemoryPolicy  string `json:"memory_policy"`
}

type Capabilities struct {
	SpawnAgents bool `json:"spawn_agents"`
	MaxWorkers  int  `json:"max_workers"`
	CreatePlans bool `json:"create_plans"`
}
