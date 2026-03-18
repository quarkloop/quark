// Package plan provides the execution plan data model and KB persistence
// for the multi-agent supervisor loop.
package plan

import "time"

// StepStatus tracks the lifecycle of a single plan step.
type StepStatus string

const (
	StepPending  StepStatus = "pending"
	StepRunning  StepStatus = "running"
	StepComplete StepStatus = "complete"
	StepFailed   StepStatus = "failed"
)

// Step is a single unit of work in the master plan.
type Step struct {
	ID          string     `json:"id"`
	Agent       string     `json:"agent"`       // agent name from Quarkfile
	Description string     `json:"description"` // natural-language task
	DependsOn   []string   `json:"depends_on"`  // step IDs that must be complete first
	Status      StepStatus `json:"status"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// Plan is the master execution plan produced and updated by the supervisor.
type Plan struct {
	Goal      string    `json:"goal"`
	Steps     []Step    `json:"steps"`
	Complete  bool      `json:"complete"`
	Summary   string    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
