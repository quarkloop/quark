// Package plan provides the execution plan data model and KB persistence
// for the multi-agent supervisor loop.
package plan

import "time"

// PlanStatus tracks the approval lifecycle of a plan or masterplan.
type PlanStatus string

const (
	// PlanDraft means the plan has been created but not yet approved by the user.
	PlanDraft PlanStatus = "draft"
	// PlanApproved means the user has approved the plan for execution.
	PlanApproved PlanStatus = "approved"
	// PlanRejected means the plan was explicitly rejected.
	PlanRejected PlanStatus = "rejected"
	// PlanExecuting means the plan is currently running.
	PlanExecuting PlanStatus = "executing"
	// PlanSucceeded means the plan completed successfully.
	PlanSucceeded PlanStatus = "succeeded"
	// PlanFailed means the plan terminated with errors.
	PlanFailed PlanStatus = "failed"
)

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
	Goal      string     `json:"goal"`
	Status    PlanStatus `json:"status"`
	Steps     []Step     `json:"steps"`
	Complete  bool       `json:"complete"`
	Summary   string     `json:"summary,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
