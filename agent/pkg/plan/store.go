package plan

import (
	"encoding/json"
	"time"

	"github.com/quarkloop/core/pkg/kb"
)

// Store provides load/save/query operations for the master execution plan
// backed by a KB namespace.
type Store struct {
	kb        kb.Store
	namespace string
	key       string
}

// NewStore creates a plan store that persists plans under the given
// KB namespace and key.
func NewStore(kb kb.Store, namespace, key string) *Store {
	return &Store{kb: kb, namespace: namespace, key: key}
}

// Load reads the master plan from the KB. Returns nil if no plan exists.
func (s *Store) Load() (*Plan, error) {
	data, err := s.kb.Get(s.namespace, s.key)
	if err != nil {
		return nil, err
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

// Save persists the master plan to the KB, updating the timestamp.
func (s *Store) Save(plan *Plan) error {
	plan.UpdatedAt = time.Now()
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	return s.kb.Set(s.namespace, s.key, data)
}

// DepsComplete returns true if every dependency of step is in status "complete".
func DepsComplete(plan *Plan, step *Step) bool {
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
