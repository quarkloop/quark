package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/core/pkg/kb"
	"github.com/quarkloop/tools/space/pkg/quarkfile"
	"github.com/quarkloop/tools/space/pkg/registry"
)

type spaceConfig struct {
	provider   string
	modelName  string
	supervisor *agent.Definition
	subAgents  map[string]*agent.Definition
	dispatcher *skill.HTTPDispatcher
}

func loadSpaceConfig(dir string, store kb.Store) (*spaceConfig, error) {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("load Quarkfile: %w", err)
	}
	lf, err := quarkfile.LoadLock(dir)
	if err != nil {
		return nil, fmt.Errorf("load lock file: %w", err)
	}

	resolver := registry.NewLocalClient()
	supervisor, err := resolveSupervisor(dir, qf, lf, resolver)
	if err != nil {
		return nil, err
	}
	if supervisor.Config.ContextWindow == 0 {
		supervisor.Config.ContextWindow = agent.DefaultContextWindow
	}
	if policy := os.Getenv("QUARK_APPROVAL_POLICY"); policy != "" {
		supervisor.Config.ApprovalPolicy = agent.ApprovalPolicy(policy)
	}

	subAgents, err := resolveSubAgents(dir, qf, lf, resolver)
	if err != nil {
		return nil, err
	}

	dispatcher, err := resolveSkills(qf, lf, resolver)
	if err != nil {
		return nil, err
	}

	if err := seedKBConfig(store, qf); err != nil {
		return nil, err
	}

	log.Printf("runtime: loaded space %q provider=%s model=%s tools=%v workers=%d approval=%s",
		qf.Meta.Name, qf.Model.Provider, qf.Model.Name, dispatcher.List(), len(subAgents), supervisor.Config.ApprovalPolicy)

	return &spaceConfig{
		provider:   qf.Model.Provider,
		modelName:  qf.Model.Name,
		supervisor: supervisor,
		subAgents:  subAgents,
		dispatcher: dispatcher,
	}, nil
}

func resolveSupervisor(dir string, qf *quarkfile.Quarkfile, lf *quarkfile.LockFile, resolver registry.Resolver) (*agent.Definition, error) {
	locked, err := lockedAgentForRef(lf, qf.Supervisor.Agent)
	if err != nil {
		return nil, err
	}
	def, err := resolver.ResolveAgent(qf.Supervisor.Agent, locked.Digest)
	if err != nil {
		return nil, fmt.Errorf("resolve supervisor %s: %w", qf.Supervisor.Agent, err)
	}
	clone := *def
	if qf.Supervisor.Prompt != "" {
		prompt, err := loadPromptText(dir, qf.Supervisor.Prompt)
		if err != nil {
			return nil, err
		}
		clone.SystemPrompt = prompt
	}
	return &clone, nil
}

func resolveSubAgents(dir string, qf *quarkfile.Quarkfile, lf *quarkfile.LockFile, resolver registry.Resolver) (map[string]*agent.Definition, error) {
	subAgents := map[string]*agent.Definition{}
	for _, entry := range qf.Agents {
		locked, err := lockedAgentForRef(lf, entry.Ref)
		if err != nil {
			return nil, err
		}
		def, err := resolver.ResolveAgent(entry.Ref, locked.Digest)
		if err != nil {
			return nil, fmt.Errorf("resolve agent %s: %w", entry.Name, err)
		}

		clone := *def
		if entry.Name != "" {
			clone.Name = entry.Name
		}
		if entry.Prompt != "" {
			prompt, err := loadPromptText(dir, entry.Prompt)
			if err != nil {
				return nil, err
			}
			clone.SystemPrompt = prompt
		}

		key := clone.Name
		if key == "" {
			key = entry.Name
		}
		subAgents[key] = &clone
	}
	return subAgents, nil
}

func resolveSkills(qf *quarkfile.Quarkfile, lf *quarkfile.LockFile, resolver registry.Resolver) (*skill.HTTPDispatcher, error) {
	dispatcher := skill.NewHTTPDispatcher()
	for _, entry := range qf.Skills {
		locked, err := lockedSkillForRef(lf, entry.Ref)
		if err != nil {
			return nil, err
		}
		def, err := resolver.ResolveSkill(entry.Ref, locked.Digest)
		if err != nil {
			return nil, fmt.Errorf("resolve tool %s: %w", entry.Name, err)
		}

		clone := *def
		clone.Name = entry.Name
		clone.Config = cloneStringMap(def.Config)
		for key, value := range entry.Config {
			clone.Config[key] = value
		}
		if endpoint := clone.Config["endpoint"]; endpoint != "" {
			clone.Endpoint = endpoint
		}
		if clone.Endpoint == "" {
			return nil, fmt.Errorf("tool %s has no endpoint", entry.Name)
		}

		dispatcher.Register(entry.Name, &clone)
		log.Printf("runtime: registered tool %s endpoint=%s ref=%s", entry.Name, clone.Endpoint, entry.Ref)
	}
	return dispatcher, nil
}

func seedKBConfig(store kb.Store, qf *quarkfile.Quarkfile) error {
	for _, entry := range qf.KB.Env {
		if entry.Key == "" || entry.From == "" {
			continue
		}
		if value := os.Getenv(entry.From); value != "" {
			if err := store.Set(agent.NSConfig, entry.Key, []byte(value)); err != nil {
				return fmt.Errorf("seed kb config %s: %w", entry.Key, err)
			}
		}
	}
	return nil
}

func loadPromptText(dir, promptPath string) (string, error) {
	fullPath := filepath.Join(dir, promptPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read prompt %s: %w", promptPath, err)
	}
	return string(data), nil
}

func lockedAgentForRef(lf *quarkfile.LockFile, ref string) (*quarkfile.LockedAgent, error) {
	for _, entry := range lf.Agents {
		if entry.Ref == ref || entry.Resolved == ref {
			entryCopy := entry
			return &entryCopy, nil
		}
	}
	return nil, fmt.Errorf("agent %s not found in lock file", ref)
}

func lockedSkillForRef(lf *quarkfile.LockFile, ref string) (*quarkfile.LockedSkill, error) {
	for _, entry := range lf.Skills {
		if entry.Ref == ref || entry.Resolved == ref {
			entryCopy := entry
			return &entryCopy, nil
		}
	}
	return nil, fmt.Errorf("tool %s not found in lock file", ref)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

type agentService struct {
	agentID string
	kb      kb.Store
	agent   *agent.Agent
	feed    *activity.Feed
}

func newAgentService(agentID string, store kb.Store, a *agent.Agent, feed *activity.Feed) *agentService {
	return &agentService{agentID: agentID, kb: store, agent: a, feed: feed}
}

func (s *agentService) Health(ctx context.Context, r *http.Request) (*agentapi.HealthResponse, error) {
	return &agentapi.HealthResponse{AgentID: s.agentID, Status: "running"}, nil
}

func (s *agentService) Mode(ctx context.Context, r *http.Request) (*agentapi.ModeResponse, error) {
	return &agentapi.ModeResponse{Mode: string(s.agent.Mode())}, nil
}

func (s *agentService) Stats(ctx context.Context, r *http.Request) (agentapi.StatsResponse, error) {
	resp := agentapi.StatsResponse{
		"agent_id":    s.agentID,
		"agent_count": 1,
		"mode":        string(s.agent.Mode()),
	}
	if cs := s.agent.ContextStats(); cs != nil {
		var contextStats map[string]interface{}
		raw, err := json.Marshal(cs)
		if err == nil && json.Unmarshal(raw, &contextStats) == nil {
			resp["context"] = contextStats
		}
	}
	return resp, nil
}

func (s *agentService) Chat(ctx context.Context, r *http.Request, req agentapi.ChatRequest) (*agentapi.ChatResponse, error) {
	resp, err := s.agent.Chat(ctx, agent.ChatRequest{
		Message: req.Message,
		Stream:  req.Stream,
		Mode:    req.Mode,
	})
	if err != nil {
		return nil, err
	}
	return &agentapi.ChatResponse{
		Reply:        resp.Reply,
		Mode:         resp.Mode,
		Warning:      resp.Warning,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}, nil
}

func (s *agentService) Stop(ctx context.Context, r *http.Request) error {
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

func (s *agentService) Plan(ctx context.Context, r *http.Request) (*agentapi.Plan, error) {
	currentPlan, err := plan.NewStore(s.kb, agent.NSPlans, agent.KeyMasterPlan).Load()
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "plan not found", err)
	}
	return convertPlan(currentPlan)
}

func (s *agentService) Activity(ctx context.Context, r *http.Request, limit int) ([]agentapi.ActivityRecord, error) {
	events := s.feed.Recent(limit)
	out := make([]agentapi.ActivityRecord, 0, len(events))
	for _, event := range events {
		out = append(out, convertActivity(event))
	}
	return out, nil
}

func (s *agentService) StreamActivity(ctx context.Context, r *http.Request, emit func(agentapi.ActivityRecord) error) error {
	for _, event := range s.feed.Recent(64) {
		if err := emit(convertActivity(event)); err != nil {
			return err
		}
	}

	ch := s.feed.Subscribe()
	defer s.feed.Unsubscribe(ch)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if err := emit(convertActivity(event)); err != nil {
				return err
			}
		}
	}
}

func convertPlan(currentPlan *plan.Plan) (*agentapi.Plan, error) {
	raw, err := json.Marshal(currentPlan)
	if err != nil {
		return nil, err
	}
	var converted agentapi.Plan
	if err := json.Unmarshal(raw, &converted); err != nil {
		return nil, err
	}
	return &converted, nil
}

func convertActivity(event activity.Event) agentapi.ActivityRecord {
	var raw json.RawMessage
	if event.Data != nil {
		if encoded, err := json.Marshal(event.Data); err == nil {
			raw = encoded
		}
	}
	return agentapi.ActivityRecord{
		ID:        event.ID,
		SessionID: event.SessionID,
		Type:      string(event.Type),
		Timestamp: event.Timestamp,
		Data:      raw,
	}
}
