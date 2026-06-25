// Package query — node lifecycle transitions
// (POST /api/v1/namespaces/:ns/systems/:sys/nodes/:name/{pause|resume|drain|archive|recover}).
//
// In the Java server, LifecycleService delegates to LifecycleManager
// which mutates the in-memory RuntimeContext. The Go server has no
// RuntimeContext (it doesn't execute systems) — so it just persists
// the state change to the Catalog and lets the runtime pick it up on
// the next deploy/undeploy cycle.
//
// This is a known limitation of the v6 architecture: lifecycle
// transitions on running nodes require a side-channel to the runtime
// (e.g. a NATS subject like quark.control.<runtimeId>.lifecycle).
// For now, the endpoint is provided for CLI compatibility but only
// updates the persisted state.
package query

import (
	"context"
	"errors"

	"github.com/quarkloop/quark/server/internal/domain"
	"github.com/quarkloop/quark/server/internal/store"
)

// LifecycleService backs the POST .../nodes/:name/{pause|resume|...}
// endpoints.
type LifecycleService struct {
	nodeRepo store.NodeRepository
}

// NewLifecycleService constructs a LifecycleService.
func NewLifecycleService(nodeRepo store.NodeRepository) *LifecycleService {
	return &LifecycleService{nodeRepo: nodeRepo}
}

// Transition applies a lifecycle transition to a node.
//
// op is one of: pause, resume, drain, archive, recover.
//
// Returns ErrNotFound if the node doesn't exist; ErrInvalidTransition
// if the transition isn't valid for the node's current state.
func (s *LifecycleService) Transition(ctx context.Context, namespace, system, name, op string) error {
	rec, err := s.nodeRepo.FindNode(ctx, namespace, system, name)
	if err != nil {
		return ErrNotFound
	}

	var newState string
	switch op {
	case "pause":
		newState = domain.NodeStatePaused
	case "resume":
		newState = domain.NodeStateActive
	case "drain":
		newState = domain.NodeStateDraining
	case "archive":
		newState = domain.NodeStateArchived
	case "recover":
		newState = domain.NodeStateActive
	default:
		return errors.New("unknown lifecycle op: " + op)
	}

	// Validate transition. Same rules as the Java LifecycleManager.
	if !isValidTransition(rec.State, newState) {
		return ErrInvalidTransition
	}

	return s.nodeRepo.UpdateNodeState(ctx, namespace, system, name, newState, rec.Health, rec.Version+1, "")
}

// ErrInvalidTransition is returned when a lifecycle transition isn't
// valid for the node's current state.
var ErrInvalidTransition = errors.New("invalid lifecycle transition")

// isValidTransition checks the lifecycle state machine rules.
// Mirrors the Java LifecycleStateMachine. Most transitions are
// allowed; the few that aren't:
//   - ARCHIVED → ACTIVE (must go through RECOVERING, but for simplicity
//     we just block it here; the runtime handles the proper sequence)
//   - DELETED → anything (terminal state)
func isValidTransition(from, to string) bool {
	if from == domain.NodeStateDeleted {
		return false
	}
	if from == domain.NodeStateArchived && to == domain.NodeStateActive {
		return false
	}
	return true
}
