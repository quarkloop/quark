package plan

import "time"

// MasterPlan is a plan with a broader view. It describes a high-level vision
// broken into sequential phases, where each phase maps to its own Plan with
// concrete steps. MasterPlan is used for large tasks that require multiple
// plans to complete.
type MasterPlan struct {
	Goal      string     `json:"goal"`
	Vision    string     `json:"vision"`
	Status    PlanStatus `json:"status"`
	Phases    []Phase    `json:"phases"`
	Complete  bool       `json:"complete"`
	Summary   string     `json:"summary,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Phase is a high-level unit of work in a master plan. Each phase maps 1:1
// to a Plan stored in the KB under the key given by PlanKey.
type Phase struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	PlanKey     string     `json:"plan_key"`   // KB key for this phase's sub-plan
	DependsOn   []string   `json:"depends_on"` // phase IDs that must complete first
	Status      StepStatus `json:"status"`
}

// PhaseDepsComplete returns true if every dependency of phase is complete.
func PhaseDepsComplete(mp *MasterPlan, phase *Phase) bool {
	statusOf := map[string]StepStatus{}
	for _, p := range mp.Phases {
		statusOf[p.ID] = p.Status
	}
	for _, dep := range phase.DependsOn {
		if statusOf[dep] != StepComplete {
			return false
		}
	}
	return true
}
