package plan

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/quarkloop/cli/pkg/kb"
)

// MasterPlanStore provides load/save operations for a MasterPlan backed by the KB.
type MasterPlanStore struct {
	kb        kb.Store
	namespace string
	key       string
}

// NewMasterPlanStore creates a master plan store that persists under the given
// KB namespace and key.
func NewMasterPlanStore(kb kb.Store, namespace, key string) *MasterPlanStore {
	return &MasterPlanStore{kb: kb, namespace: namespace, key: key}
}

// Load reads the master plan from the KB. Returns nil, error if none exists.
func (s *MasterPlanStore) Load() (*MasterPlan, error) {
	data, err := s.kb.Get(s.namespace, s.key)
	if err != nil {
		return nil, err
	}
	var mp MasterPlan
	if err := json.Unmarshal(data, &mp); err != nil {
		return nil, fmt.Errorf("unmarshal masterplan: %w", err)
	}
	return &mp, nil
}

// Save persists the master plan to the KB, updating the timestamp.
func (s *MasterPlanStore) Save(mp *MasterPlan) error {
	mp.UpdatedAt = time.Now()
	data, err := json.Marshal(mp)
	if err != nil {
		return fmt.Errorf("marshal masterplan: %w", err)
	}
	return s.kb.Set(s.namespace, s.key, data)
}

// SubPlanStore returns a standard Store keyed to "phase-<phaseID>" for
// persisting the sub-plan associated with a given phase.
func (s *MasterPlanStore) SubPlanStore(phaseID string) *Store {
	return NewStore(s.kb, s.namespace, fmt.Sprintf("phase-%s", phaseID))
}
