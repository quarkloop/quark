package agent

import (
	"encoding/json"
	"time"
)

// ─── Plan data structures ─────────────────────────────────────────────────────

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

// ─── Plan KB helpers ──────────────────────────────────────────────────────────

// loadPlan reads the master plan from the KB.
func (e *Executor) loadPlan() (*Plan, error) {
	data, err := e.kb.Get(NSPlans, KeyMasterPlan)
	if err != nil {
		return nil, err
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

// savePlan persists the master plan to the KB.
func (e *Executor) savePlan(plan *Plan) error {
	plan.UpdatedAt = time.Now()
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	return e.kb.Set(NSPlans, KeyMasterPlan, data)
}

// depsComplete returns true if every dependency of step is in status "complete".
func (e *Executor) depsComplete(plan *Plan, step *Step) bool {
	statusOf := map[string]StepStatus{}
	for _, s := range plan.Steps {
		statusOf[s.ID] = s.Status
	}
	for _, dep := range step.DependsOn {
		if statusOf[dep] != StepComplete {
			return false
		}
	}
	return true
}
