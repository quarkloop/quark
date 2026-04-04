package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	"github.com/quarkloop/cli/pkg/kb"
)

const namespace = "sessions"

// LocalStore implements Service using the local KB filesystem store.
type LocalStore struct {
	kb kb.Store
}

// NewLocalService creates a session service backed by the space's KB.
func NewLocalService(spaceDir string) (Service, error) {
	k, err := kb.Open(spaceDir)
	if err != nil {
		return nil, fmt.Errorf("open kb for sessions: %w", err)
	}
	return &LocalStore{kb: k}, nil
}

func (s *LocalStore) Create(_ context.Context, req agentapi.CreateSessionRequest) (*agentapi.CreateSessionResponse, error) {
	now := time.Now()
	key := fmt.Sprintf("session:%s:%d", req.Type, now.UnixNano())
	rec := agentapi.SessionRecord{
		Key:       key,
		Type:      req.Type,
		Status:    "active",
		Title:     req.Title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("marshal session: %w", err)
	}
	if err := s.kb.Set(namespace, key, data); err != nil {
		return nil, fmt.Errorf("store session: %w", err)
	}
	return &agentapi.CreateSessionResponse{Session: rec}, nil
}

func (s *LocalStore) Get(_ context.Context, sessionKey string) (*agentapi.SessionRecord, error) {
	data, err := s.kb.Get(namespace, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("session %s not found", sessionKey)
	}
	var rec agentapi.SessionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &rec, nil
}

func (s *LocalStore) Delete(_ context.Context, sessionKey string) error {
	return s.kb.Delete(namespace, sessionKey)
}

func (s *LocalStore) List(_ context.Context) ([]agentapi.SessionRecord, error) {
	keys, err := s.kb.List(namespace)
	if err != nil {
		return nil, err
	}
	var out []agentapi.SessionRecord
	for _, k := range keys {
		data, err := s.kb.Get(namespace, k)
		if err != nil {
			continue
		}
		var rec agentapi.SessionRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (s *LocalStore) Close() error {
	return s.kb.Close()
}
