// Package session defines the session service interface and a local
// KB-backed implementation. The interface is transport-agnostic — both
// local filesystem and HTTP modes satisfy it.
package session

import (
	"context"

	agentapi "github.com/quarkloop/agent-api"
)

// Service defines operations for managing agent sessions.
type Service interface {
	Create(ctx context.Context, req agentapi.CreateSessionRequest) (*agentapi.CreateSessionResponse, error)
	Get(ctx context.Context, sessionKey string) (*agentapi.SessionRecord, error)
	Delete(ctx context.Context, sessionKey string) error
	List(ctx context.Context) ([]agentapi.SessionRecord, error)
	Close() error
}
