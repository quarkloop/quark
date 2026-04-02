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
	"github.com/quarkloop/agent/pkg/agentcore"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/resolver"
	"github.com/quarkloop/agent/pkg/session"
	"github.com/quarkloop/agent/pkg/tool"
	"github.com/quarkloop/core/pkg/kb"
	"github.com/quarkloop/tools/space/pkg/quarkfile"
)

type spaceConfig struct {
	provider   string
	modelName  string
	routing    *quarkfile.RoutingSection
	supervisor *agentcore.Definition
	subAgents  map[string]*agentcore.Definition
	registry   *tool.Registry
	toolDefs   map[string]*tool.Definition
	qf         *quarkfile.Quarkfile // raw parsed Quarkfile, retained for diffing
}

// ConfigDiff describes what changed between two Quarkfile versions.
type ConfigDiff struct {
	ModelChanged       bool
	RoutingChanged     bool
	ToolsAdded         []string
	ToolsRemoved       []string
	PermissionsChanged bool
	PromptsChanged     bool
}

// IsEmpty reports whether the diff contains no changes.
func (d ConfigDiff) IsEmpty() bool {
	return !d.ModelChanged && !d.RoutingChanged &&
		len(d.ToolsAdded) == 0 && len(d.ToolsRemoved) == 0 &&
		!d.PermissionsChanged && !d.PromptsChanged
}

// DiffQuarkfiles returns the set of changes between two Quarkfile versions.
// Returns an empty diff if either argument is nil.
func DiffQuarkfiles(oldQF, newQF *quarkfile.Quarkfile) ConfigDiff {
	if oldQF == nil || newQF == nil {
		return ConfigDiff{}
	}
	var d ConfigDiff

	if oldQF.Model.Provider != newQF.Model.Provider || oldQF.Model.Name != newQF.Model.Name {
		d.ModelChanged = true
	}

	if !routingSectionsEqual(oldQF.Routing, newQF.Routing) {
		d.RoutingChanged = true
	}

	oldTools := make(map[string]bool, len(oldQF.Tools))
	for _, t := range oldQF.Tools {
		oldTools[t.Name] = true
	}
	newTools := make(map[string]bool, len(newQF.Tools))
	for _, t := range newQF.Tools {
		newTools[t.Name] = true
	}
	for name := range newTools {
		if !oldTools[name] {
			d.ToolsAdded = append(d.ToolsAdded, name)
		}
	}
	for name := range oldTools {
		if !newTools[name] {
			d.ToolsRemoved = append(d.ToolsRemoved, name)
		}
	}

	if !permissionsEqual(oldQF.Permissions, newQF.Permissions) {
		d.PermissionsChanged = true
	}

	if oldQF.Supervisor.Prompt != newQF.Supervisor.Prompt {
		d.PromptsChanged = true
	}

	return d
}

func routingSectionsEqual(a, b quarkfile.RoutingSection) bool {
	if len(a.Rules) != len(b.Rules) || len(a.Fallback) != len(b.Fallback) {
		return false
	}
	for i := range a.Rules {
		if a.Rules[i] != b.Rules[i] {
			return false
		}
	}
	for i := range a.Fallback {
		if a.Fallback[i] != b.Fallback[i] {
			return false
		}
	}
	return true
}

func permissionsEqual(a, b quarkfile.Permissions) bool {
	return slicesEqual(a.Filesystem.AllowedPaths, b.Filesystem.AllowedPaths) &&
		slicesEqual(a.Filesystem.ReadOnly, b.Filesystem.ReadOnly) &&
		slicesEqual(a.Network.AllowedHosts, b.Network.AllowedHosts) &&
		slicesEqual(a.Network.Deny, b.Network.Deny) &&
		slicesEqual(a.Tools.Allowed, b.Tools.Allowed) &&
		slicesEqual(a.Tools.Denied, b.Tools.Denied)
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func loadSpaceConfig(dir string, store kb.Store) (*spaceConfig, error) {
	qf, err := quarkfile.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("load Quarkfile: %w", err)
	}

	supervisor := buildSupervisorDef(dir, qf)
	if policy := os.Getenv("QUARK_APPROVAL_POLICY"); policy != "" {
		supervisor.Config.ApprovalPolicy = agentcore.ApprovalPolicy(policy)
	}

	subAgents := buildSubAgentDefs(dir, qf)

	registry, toolDefs, err := buildToolRegistry(qf)
	if err != nil {
		return nil, err
	}

	if err := seedKBConfig(store, qf); err != nil {
		return nil, err
	}

	log.Printf("runtime: loaded space %q tools=%v subagents=%d approval=%s",
		qf.Meta.Name, registry.List(), len(subAgents), supervisor.Config.ApprovalPolicy)

	var routing *quarkfile.RoutingSection
	if len(qf.Routing.Rules) > 0 || len(qf.Routing.Fallback) > 0 {
		r := qf.Routing
		routing = &r
	}

	return &spaceConfig{
		provider:   qf.Model.Provider,
		modelName:  qf.Model.Name,
		routing:    routing,
		supervisor: supervisor,
		subAgents:  subAgents,
		registry:   registry,
		toolDefs:   toolDefs,
		qf:         qf,
	}, nil
}

func buildSupervisorDef(dir string, qf *quarkfile.Quarkfile) *agentcore.Definition {
	def := &agentcore.Definition{
		Name: "supervisor",
		Capabilities: agentcore.Capabilities{
			SpawnAgents: qf.Capabilities.SpawnAgents,
			MaxWorkers:  qf.Capabilities.MaxWorkers,
			CreatePlans: qf.Capabilities.CreatePlans,
		},
	}
	if def.Capabilities.MaxWorkers == 0 {
		def.Capabilities.MaxWorkers = 10
	}
	if qf.Capabilities.ApprovalPolicy != "" {
		def.Config.ApprovalPolicy = agentcore.ApprovalPolicy(qf.Capabilities.ApprovalPolicy)
	}
	if qf.Supervisor.Prompt != "" {
		if prompt, err := loadPromptText(dir, qf.Supervisor.Prompt); err == nil {
			def.SystemPrompt = prompt
		} else {
			log.Printf("runtime: failed to load supervisor prompt %s: %v", qf.Supervisor.Prompt, err)
		}
	}
	return def
}

func buildSubAgentDefs(dir string, qf *quarkfile.Quarkfile) map[string]*agentcore.Definition {
	subAgents := map[string]*agentcore.Definition{}
	for _, entry := range qf.Agents {
		def := &agentcore.Definition{
			Name: entry.Name,
		}
		if entry.Prompt != "" {
			if prompt, err := loadPromptText(dir, entry.Prompt); err == nil {
				def.SystemPrompt = prompt
			} else {
				log.Printf("runtime: failed to load agent prompt %s: %v", entry.Prompt, err)
			}
		}
		subAgents[entry.Name] = def
	}
	return subAgents
}

// buildToolRegistry creates the tool registry from Quarkfile tool entries.
// Each tool must have a name and an endpoint (via config or direct field).
// Returns the registry and a map of tool definitions for the orchestrator.
func buildToolRegistry(qf *quarkfile.Quarkfile) (*tool.Registry, map[string]*tool.Definition, error) {
	registry := tool.NewRegistry()
	defs := make(map[string]*tool.Definition)
	for _, entry := range qf.Tools {
		def := &tool.Definition{
			Ref:    entry.Ref,
			Name:   entry.Name,
			Config: make(map[string]string),
		}
		for key, value := range entry.Config {
			def.Config[key] = value
		}
		if endpoint := def.Config["endpoint"]; endpoint != "" {
			def.Endpoint = endpoint
		}
		if def.Endpoint == "" {
			return nil, nil, fmt.Errorf("tool %s has no endpoint", entry.Name)
		}

		registry.Register(entry.Name, def)
		defs[entry.Name] = def
		log.Printf("runtime: registered tool %s endpoint=%s", entry.Name, def.Endpoint)
	}
	return registry, defs, nil
}

func seedKBConfig(store kb.Store, qf *quarkfile.Quarkfile) error {
	for _, entry := range qf.KB.Env {
		if entry.Key == "" || entry.From == "" {
			continue
		}
		if value := os.Getenv(entry.From); value != "" {
			if err := store.Set(agentcore.NSConfig, entry.Key, []byte(value)); err != nil {
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

type agentService struct {
	dir          string
	kb           kb.Store
	registry     *agent.Registry
	resolver     resolver.Resolver
	bus          *eventbus.Bus
	actWriter    *activity.Writer
	sessionStore *session.Store
}

func newAgentService(dir string, store kb.Store, registry *agent.Registry, res resolver.Resolver, bus *eventbus.Bus, actWriter *activity.Writer, sessStore *session.Store) *agentService {
	return &agentService{dir: dir, kb: store, registry: registry, resolver: res, bus: bus, actWriter: actWriter, sessionStore: sessStore}
}

func (s *agentService) resolveAgent(ctx context.Context, sessionKey string) (*agent.Agent, error) {
	msg := resolver.InboundMessage{
		SessionKey: sessionKey,
		Channel:    "web",
	}
	agentID, err := s.resolver.Resolve(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("resolve agent: %w", err)
	}
	a, ok := s.registry.Get(agentID)
	if !ok {
		return nil, fmt.Errorf("agent %q not found", agentID)
	}
	return a, nil
}

func (s *agentService) Health(ctx context.Context, r *http.Request) (*agentapi.HealthResponse, error) {
	return &agentapi.HealthResponse{AgentID: "supervisor", Status: "running"}, nil
}

func (s *agentService) Info(ctx context.Context, r *http.Request) (*agentapi.InfoResponse, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	return &agentapi.InfoResponse{
		AgentID:  "supervisor",
		Provider: a.Provider(),
		Model:    a.ModelName(),
		Mode:     string(a.Mode()),
		Tools:    a.Tools(),
	}, nil
}

func (s *agentService) Mode(ctx context.Context, r *http.Request) (*agentapi.ModeResponse, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	return &agentapi.ModeResponse{Mode: string(a.Mode())}, nil
}

func (s *agentService) Stats(ctx context.Context, r *http.Request) (agentapi.StatsResponse, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	resp := agentapi.StatsResponse{
		"agent_id":    "supervisor",
		"agent_count": len(s.registry.List()),
		"mode":        string(a.Mode()),
	}
	if cs := a.ContextStats(); cs != nil {
		var contextStats map[string]interface{}
		raw, err := json.Marshal(cs)
		if err == nil && json.Unmarshal(raw, &contextStats) == nil {
			resp["context"] = contextStats
		}
	}
	return resp, nil
}

func (s *agentService) Chat(ctx context.Context, r *http.Request, req agentapi.ChatRequest) (*agentapi.ChatResponse, error) {
	a, err := s.resolveAgent(ctx, req.SessionKey)
	if err != nil {
		return nil, err
	}

	agentReq := agentcore.ChatRequest{
		Message:    req.Message,
		SessionKey: req.SessionKey,
		Stream:     req.Stream,
		Mode:       req.Mode,
	}

	// Save uploaded files to the workspace and pass metadata.
	if len(req.Files) > 0 {
		for i, f := range req.Files {
			saved, err := s.saveUploadedFile(f)
			if err != nil {
				return nil, agentapi.Error(http.StatusBadRequest,
					fmt.Sprintf("save file %s: %v", f.Name, err), err)
			}
			agentReq.Files = append(agentReq.Files, agentcore.FileAttachment{
				Name:     saved.Name,
				MimeType: saved.MimeType,
				Size:     saved.Size,
				Path:     saved.Path,
			})
			req.Files[i].Path = saved.Path
		}
	}

	resp, err := a.Chat(ctx, req.SessionKey, agentReq)
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

// saveUploadedFile writes a file attachment to the uploads directory in the
// agent workspace and returns the attachment with its Path set.
func (s *agentService) saveUploadedFile(f agentapi.FileAttachment) (agentapi.FileAttachment, error) {
	if len(f.Content) == 0 {
		return f, nil
	}
	uploadsDir := filepath.Join(s.dir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return f, fmt.Errorf("create uploads dir: %w", err)
	}
	dest := filepath.Join(uploadsDir, f.Name)
	if err := os.WriteFile(dest, f.Content, 0o644); err != nil {
		return f, fmt.Errorf("write file: %w", err)
	}
	f.Path = dest
	return f, nil
}

func (s *agentService) Stop(ctx context.Context, r *http.Request) error {
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

func (s *agentService) Plan(ctx context.Context, r *http.Request) (*agentapi.Plan, error) {
	currentPlan, err := plan.NewStore(s.kb, agentcore.NSPlans, agentcore.KeyMasterPlan).Load()
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "plan not found", err)
	}
	return convertPlan(currentPlan)
}

func (s *agentService) Activity(ctx context.Context, r *http.Request, limit int) ([]agentapi.ActivityRecord, error) {
	events := s.actWriter.Recent(limit)
	out := make([]agentapi.ActivityRecord, 0, len(events))
	for _, event := range events {
		out = append(out, convertActivity(event))
	}
	return out, nil
}

func (s *agentService) StreamActivity(ctx context.Context, r *http.Request, emit func(agentapi.ActivityRecord) error) error {
	for _, event := range s.actWriter.Recent(64) {
		if err := emit(convertActivity(event)); err != nil {
			return err
		}
	}

	ch := s.bus.Subscribe(256)
	defer s.bus.Unsubscribe(ch)

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
		Type:      string(event.Kind),
		Timestamp: event.Timestamp,
		Data:      raw,
	}
}

func (s *agentService) Sessions(ctx context.Context, r *http.Request) ([]agentapi.SessionRecord, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	sessions, err := a.ListSessions()
	if err != nil {
		return nil, err
	}
	out := make([]agentapi.SessionRecord, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, convertSession(sess))
	}
	return out, nil
}

func (s *agentService) Session(ctx context.Context, r *http.Request, sessionKey string) (*agentapi.SessionRecord, error) {
	a, err := s.resolveAgent(ctx, sessionKey)
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "session not found", err)
	}
	sess, err := a.GetSession(sessionKey)
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "session not found", err)
	}
	rec := convertSession(sess)
	return &rec, nil
}

func (s *agentService) SessionActivity(ctx context.Context, r *http.Request, sessionKey string, limit int) ([]agentapi.ActivityRecord, error) {
	events, err := s.actWriter.History(sessionKey)
	if err != nil {
		return nil, err
	}
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	out := make([]agentapi.ActivityRecord, 0, len(events))
	for _, ev := range events {
		out = append(out, convertActivity(ev))
	}
	return out, nil
}

func (s *agentService) CreateSession(ctx context.Context, r *http.Request, req agentapi.CreateSessionRequest) (*agentapi.CreateSessionResponse, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	sess, err := a.CreateSession(session.Type(req.Type), req.Title)
	if err != nil {
		return nil, agentapi.Error(http.StatusBadRequest, err.Error(), err)
	}
	return &agentapi.CreateSessionResponse{
		Session: convertSession(sess),
	}, nil
}

func (s *agentService) DeleteSession(ctx context.Context, r *http.Request, sessionKey string) error {
	a, err := s.resolveAgent(ctx, sessionKey)
	if err != nil {
		return err
	}
	if err := a.DeleteSession(sessionKey); err != nil {
		return agentapi.Error(http.StatusBadRequest, err.Error(), err)
	}
	return nil
}

func convertSession(sess *session.Session) agentapi.SessionRecord {
	return agentapi.SessionRecord{
		Key:       sess.Key,
		AgentID:   sess.AgentID,
		Type:      agentapi.SessionType(sess.Type),
		Status:    string(sess.Status),
		Title:     sess.Title,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
		EndedAt:   sess.EndedAt,
	}
}

func (s *agentService) ApprovePlan(ctx context.Context, r *http.Request) (*agentapi.Plan, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	p, err := a.ApprovePlan()
	if err != nil {
		return nil, agentapi.Error(http.StatusBadRequest, err.Error(), err)
	}
	return convertPlan(p)
}

func (s *agentService) RejectPlan(ctx context.Context, r *http.Request) error {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return err
	}
	if err := a.RejectPlan(); err != nil {
		return agentapi.Error(http.StatusBadRequest, err.Error(), err)
	}
	return nil
}

func (s *agentService) SessionBudget(ctx context.Context, r *http.Request, sessionKey string) (*agentapi.BudgetResponse, error) {
	a, err := s.resolveAgent(ctx, sessionKey)
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "agent not found", err)
	}
	status, err := a.BudgetStatus(sessionKey)
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "session not found", err)
	}
	return &agentapi.BudgetResponse{
		TotalBudget:      status.TotalBudget,
		UsedTokens:       status.UsedTokens,
		AvailableTokens:  status.AvailableTokens,
		UsagePct:         status.UsagePct,
		AtSoftLimit:      status.CompactionNeeded,
		AtHardLimit:      status.AtHardLimit,
		CompactionNeeded: status.CompactionNeeded,
	}, nil
}
