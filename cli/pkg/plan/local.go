package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	"github.com/quarkloop/cli/pkg/kb"
)

const namespace = "plans"

// LocalStore implements Service using the local KB filesystem store.
type LocalStore struct {
	kb kb.Store
}

// NewLocalService creates a plan service backed by the space's KB.
func NewLocalService(spaceDir string) (Service, error) {
	k, err := kb.Open(spaceDir)
	if err != nil {
		return nil, fmt.Errorf("open kb for plans: %w", err)
	}
	return &LocalStore{kb: k}, nil
}

func (s *LocalStore) Create(_ context.Context, goal string) (*agentapi.Plan, error) {
	now := time.Now()
	id := fmt.Sprintf("plan-%d", now.UnixNano())
	p := agentapi.Plan{
		Goal:      goal,
		Status:    agentapi.PlanDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal plan: %w", err)
	}
	if err := s.kb.Set(namespace, id, data); err != nil {
		return nil, fmt.Errorf("store plan: %w", err)
	}
	return &p, nil
}

func (s *LocalStore) Get(_ context.Context, planID string) (*agentapi.Plan, error) {
	data, err := s.kb.Get(namespace, planID)
	if err != nil {
		return nil, fmt.Errorf("plan %s not found", planID)
	}
	var p agentapi.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w", err)
	}
	return &p, nil
}

func (s *LocalStore) List(_ context.Context) ([]agentapi.Plan, error) {
	keys, err := s.kb.List(namespace)
	if err != nil {
		return nil, err
	}
	var out []agentapi.Plan
	for _, k := range keys {
		data, err := s.kb.Get(namespace, k)
		if err != nil {
			continue
		}
		var p agentapi.Plan
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *LocalStore) Approve(_ context.Context, planID string) (*agentapi.Plan, error) {
	return s.updateStatus(planID, agentapi.PlanApproved)
}

func (s *LocalStore) Reject(_ context.Context, planID string) error {
	_, err := s.updateStatus(planID, "rejected")
	return err
}

func (s *LocalStore) Update(_ context.Context, planID string, status string) error {
	_, err := s.updateStatus(planID, agentapi.PlanStatus(status))
	return err
}

func (s *LocalStore) Close() error {
	return s.kb.Close()
}

func (s *LocalStore) updateStatus(planID string, status agentapi.PlanStatus) (*agentapi.Plan, error) {
	data, err := s.kb.Get(namespace, planID)
	if err != nil {
		return nil, fmt.Errorf("plan %s not found", planID)
	}
	var p agentapi.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w", err)
	}
	p.Status = status
	p.UpdatedAt = time.Now()
	data, err = json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal plan: %w", err)
	}
	if err := s.kb.Set(namespace, planID, data); err != nil {
		return nil, fmt.Errorf("update plan: %w", err)
	}
	return &p, nil
}
