package space

import (
	"context"
	"fmt"
	"net/http"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"
)

type agentProxyService struct {
	store Store
}

func newAgentProxyService(store Store) *agentProxyService {
	return &agentProxyService{store: store}
}

func (s *agentProxyService) Health(ctx context.Context, r *http.Request) (*agentapi.HealthResponse, error) {
	client, err := s.clientForRequest(r)
	if err != nil {
		return nil, err
	}
	return client.Health(ctx)
}

func (s *agentProxyService) Info(ctx context.Context, r *http.Request) (*agentapi.InfoResponse, error) {
	client, err := s.clientForRequest(r)
	if err != nil {
		return nil, err
	}
	return client.Info(ctx)
}

func (s *agentProxyService) Mode(ctx context.Context, r *http.Request) (*agentapi.ModeResponse, error) {
	client, err := s.clientForRequest(r)
	if err != nil {
		return nil, err
	}
	return client.Mode(ctx)
}

func (s *agentProxyService) Stats(ctx context.Context, r *http.Request) (agentapi.StatsResponse, error) {
	client, err := s.clientForRequest(r)
	if err != nil {
		return nil, err
	}
	return client.Stats(ctx)
}

func (s *agentProxyService) Chat(ctx context.Context, r *http.Request, req agentapi.ChatRequest) (*agentapi.ChatResponse, error) {
	client, err := s.clientForRequest(r)
	if err != nil {
		return nil, err
	}
	return client.Chat(ctx, req)
}

func (s *agentProxyService) Stop(ctx context.Context, r *http.Request) error {
	client, err := s.clientForRequest(r)
	if err != nil {
		return err
	}
	return client.Stop(ctx)
}

func (s *agentProxyService) Plan(ctx context.Context, r *http.Request) (*agentapi.Plan, error) {
	client, err := s.clientForRequest(r)
	if err != nil {
		return nil, err
	}
	return client.Plan(ctx)
}

func (s *agentProxyService) Activity(ctx context.Context, r *http.Request, limit int) ([]agentapi.ActivityRecord, error) {
	client, err := s.clientForRequest(r)
	if err != nil {
		return nil, err
	}
	return client.Activity(ctx, limit)
}

func (s *agentProxyService) StreamActivity(ctx context.Context, r *http.Request, emit func(agentapi.ActivityRecord) error) error {
	client, err := s.clientForRequest(r)
	if err != nil {
		return err
	}
	var streamErr error
	err = client.StreamActivity(ctx, func(record agentapi.ActivityRecord) {
		if streamErr != nil {
			return
		}
		streamErr = emit(record)
	})
	if streamErr != nil {
		return streamErr
	}
	return err
}

func (s *agentProxyService) clientForRequest(r *http.Request) (*agentclient.Client, error) {
	agentID := r.PathValue("id")
	if agentID == "" {
		return nil, agentapi.Error(http.StatusBadRequest, "agent id is required", nil)
	}
	sp, err := s.store.Get(agentID)
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, err.Error(), err)
	}
	if sp.Status != StatusRunning {
		return nil, agentapi.Error(http.StatusConflict, fmt.Sprintf("agent is %s, not running", sp.Status), nil)
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", sp.Port)
	return agentclient.New(baseURL), nil
}
