// Package agent is the thin orchestrator that manages sessions and routes
// requests to per-session contexts. All business logic lives in the
// chat, cycle, inference, and execution packages.
package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/quarkloop/agent/pkg/agentcore"
	"github.com/quarkloop/agent/pkg/chat"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/cycle"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/intervention"
	planpkg "github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/session"
	"github.com/quarkloop/agent/pkg/subagent"
)

// Agent drives the multi-agent loop inside a single space runtime process.
// It manages sessions (each with its own context) and routes requests.
type Agent struct {
	agentID     string
	res         *agentcore.Resources
	def         *agentcore.Definition
	subAgents   map[string]*agentcore.Definition
	sessions    map[string]*SessionState // session key → state
	sessStore   *session.Store
	subagentMgr *subagent.Manager
	mu          sync.RWMutex
}

// New constructs an Agent. Call Init before Run or Chat.
func New(
	agentID string,
	def *agentcore.Definition,
	res *agentcore.Resources,
	sessStore *session.Store,
	subAgents map[string]*agentcore.Definition,
) *Agent {
	if subAgents == nil {
		subAgents = map[string]*agentcore.Definition{}
	}
	return &Agent{
		res:       res,
		agentID:   agentID,
		def:       def,
		subAgents: subAgents,
		sessions:  map[string]*SessionState{},
		sessStore: sessStore,
	}
}

// Init loads or creates the main session and restores active chat sessions.
func (a *Agent) Init() error {
	// Initialize subagent manager.
	if a.res.EventBus != nil {
		maxWorkers := a.def.Capabilities.MaxWorkers
		if maxWorkers <= 0 {
			maxWorkers = 5
		}
		a.subagentMgr = subagent.NewManager(maxWorkers, a.res.EventBus)
	}

	// Build shared llmctx infrastructure.
	tc, err := llmctx.DefaultTokenComputer()
	if err != nil {
		return fmt.Errorf("token computer: %w", err)
	}
	a.res.TC = tc
	a.res.IDGen = llmctx.DefaultIDGenerator()
	a.res.AdapterReg = llmctx.NewAdapterRegistry()
	a.res.VisPolicy = llmctx.DefaultVisibilityPolicy()
	a.res.VisPolicy.Set(llmctx.ToolCallMessageType, llmctx.VisibleToLLMAndDev)
	a.res.VisPolicy.Set(llmctx.ToolResultMessageType, llmctx.VisibleToLLMAndDev)
	a.res.VisPolicy.Set(llmctx.ReasoningMessageType, llmctx.VisibleToDeveloperOnly)
	a.res.VisPolicy.Set(llmctx.PlanMessageType, llmctx.VisibleToDeveloperOnly)
	a.res.VisPolicy.Set(llmctx.MemoryMessageType, llmctx.VisibleToLLMAndDev)

	// Load or create main session.
	mainKey := session.MainKey(a.agentID)
	if a.sessStore.MainExists(a.agentID) {
		mainSess, err := a.sessStore.GetMain(a.agentID)
		if err != nil {
			return fmt.Errorf("load main session: %w", err)
		}

		// Try to restore context from snapshot.
		ac, err := LoadSessionSnapshot(a.res.KB, a.res.TC, a.res.IDGen, mainKey)
		if err != nil {
			log.Printf("agent: no snapshot for main session, creating fresh context")
			ac, err = a.buildFreshContext(agentcore.DefaultContextWindow, agentcore.ModeAuto)
			if err != nil {
				return fmt.Errorf("fresh context: %w", err)
			}
		} else {
			log.Printf("agent: restored main session context from snapshot")
		}

		mode := a.restoreMode()
		a.mu.Lock()
		a.sessions[mainKey] = &SessionState{
			Session:       mainSess,
			Context:       ac,
			Mode:          mode,
			PlanStore:     planpkg.NewStore(a.res.KB, agentcore.NSPlans, agentcore.KeyMasterPlan),
			MasterStore:   planpkg.NewMasterPlanStore(a.res.KB, agentcore.NSPlans, agentcore.KeyMasterPlanDoc),
			Interventions: intervention.New(10),
		}
		a.mu.Unlock()
	} else {
		now := time.Now()
		mainSess := &session.Session{
			Key:       mainKey,
			AgentID:   a.agentID,
			Type:      session.TypeMain,
			Status:    session.StatusActive,
			Title:     "Main",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := a.sessStore.Create(mainSess); err != nil {
			return fmt.Errorf("create main session: %w", err)
		}

		ac, err := a.buildFreshContext(agentcore.DefaultContextWindow, agentcore.ModeAuto)
		if err != nil {
			return fmt.Errorf("fresh context: %w", err)
		}

		a.mu.Lock()
		a.sessions[mainKey] = &SessionState{
			Session:       mainSess,
			Context:       ac,
			Mode:          agentcore.ModeAuto,
			PlanStore:     planpkg.NewStore(a.res.KB, agentcore.NSPlans, agentcore.KeyMasterPlan),
			MasterStore:   planpkg.NewMasterPlanStore(a.res.KB, agentcore.NSPlans, agentcore.KeyMasterPlanDoc),
			Interventions: intervention.New(10),
		}
		a.mu.Unlock()
	}

	// Restore active chat sessions.
	active, err := a.sessStore.ListActive(a.agentID)
	if err != nil {
		log.Printf("agent: failed to list active sessions: %v", err)
		return nil
	}
	for _, sess := range active {
		if sess.Type == session.TypeMain {
			continue // already loaded
		}
		ac, err := LoadSessionSnapshot(a.res.KB, a.res.TC, a.res.IDGen, sess.Key)
		if err != nil {
			ac, _ = a.buildFreshContext(agentcore.DefaultContextWindow, agentcore.ModeAuto)
		}
		a.mu.Lock()
		a.sessions[sess.Key] = &SessionState{
			Session:       sess,
			Context:       ac,
			Mode:          agentcore.ModeAuto,
			Interventions: intervention.New(10),
		}
		a.mu.Unlock()
	}

	return nil
}

// Chat routes a message to the session's context and calls chat.Process.
func (a *Agent) Chat(ctx context.Context, sessionKey string, req agentcore.ChatRequest) (*agentcore.ChatResponse, error) {
	if sessionKey == "" {
		sessionKey = session.MainKey(a.agentID)
	}

	a.mu.RLock()
	state, ok := a.sessions[sessionKey]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionKey)
	}

	// Resolve mode.
	mode := state.Mode
	if req.Mode != "" {
		if m, err := agentcore.ParseMode(req.Mode); err == nil {
			mode = m
			state.Mode = m
			a.res.KB.Set(agentcore.NSConfig, agentcore.KeyMode, []byte(string(m)))
		}
	}

	// Update system prompt for the resolved mode.
	a.updateSystemPrompt(state, mode)

	// Emit user message activity.
	a.emitMessage(sessionKey, agentcore.AuthorUser, req.Message, string(mode))

	deps := &chat.Deps{
		Def:           a.def,
		SubAgents:     a.subAgents,
		PlanStore:     state.PlanStore,
		MasterStore:   state.MasterStore,
		Interventions: state.Interventions,
	}
	if deps.PlanStore == nil {
		deps.PlanStore = planpkg.NewStore(a.res.KB, agentcore.NSPlans, agentcore.KeyMasterPlan)
	}
	if deps.MasterStore == nil {
		deps.MasterStore = planpkg.NewMasterPlanStore(a.res.KB, agentcore.NSPlans, agentcore.KeyMasterPlanDoc)
	}

	req.SessionKey = sessionKey
	resp, err := chat.Process(ctx, state.Context, a.res, deps, mode, req)

	// Emit agent reply activity.
	if err == nil && resp != nil && resp.Reply != "" {
		a.emitMessage(sessionKey, agentcore.AuthorAgent, resp.Reply, resp.Mode)
	}

	// Save checkpoint.
	a.saveCheckpoint(sessionKey)

	return resp, err
}

// Run starts the agent loop, blocking until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	log.Printf("agent: starting (name=%s model=%s/%s)",
		a.def.Name, a.res.GetGateway().Provider(), a.res.GetGateway().ModelName())

	mainKey := session.MainKey(a.agentID)
	a.emit(mainKey, eventbus.KindSessionStarted, map[string]string{
		"agent": a.def.Name,
	})

	for {
		select {
		case <-ctx.Done():
			a.saveCheckpoint(mainKey)
			a.emit(mainKey, eventbus.KindSessionEnded, map[string]string{"reason": "cancelled"})
			return ctx.Err()
		default:
		}

		interval := a.runOnce(ctx, mainKey)

		select {
		case <-ctx.Done():
			a.saveCheckpoint(mainKey)
			a.emit(mainKey, eventbus.KindSessionEnded, map[string]string{"reason": "cancelled"})
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (a *Agent) runOnce(ctx context.Context, mainKey string) time.Duration {
	a.mu.RLock()
	state, ok := a.sessions[mainKey]
	a.mu.RUnlock()
	if !ok {
		return 2 * time.Second
	}

	switch state.Mode {
	case agentcore.ModePlan:
		return a.runPlanCycle(ctx, state)
	case agentcore.ModeMasterPlan:
		return a.runMasterPlanCycle(ctx, state)
	default:
		return 2 * time.Second
	}
}

func (a *Agent) runPlanCycle(ctx context.Context, state *SessionState) time.Duration {
	p, err := state.PlanStore.Load()
	if err != nil || p == nil || p.Complete || p.Status != planpkg.PlanApproved {
		return 2 * time.Second
	}

	done, err := cycle.Supervisor(ctx, state.Context, a.res, state.PlanStore, a, a.subAgents, state.Interventions)
	if err != nil {
		log.Printf("agent: plan cycle error: %v", err)
		return 5 * time.Second
	}
	if done {
		log.Printf("agent: plan complete")
		a.saveCheckpoint(state.Session.Key)
	}
	return 2 * time.Second
}

func (a *Agent) runMasterPlanCycle(ctx context.Context, state *SessionState) time.Duration {
	mp, err := state.MasterStore.Load()
	if err != nil || mp == nil || mp.Complete || mp.Status != planpkg.PlanApproved {
		return 2 * time.Second
	}

	var phase *planpkg.Phase
	for i := range mp.Phases {
		p := &mp.Phases[i]
		if p.Status == planpkg.StepPending && planpkg.PhaseDepsComplete(mp, p) {
			phase = p
			break
		}
		if p.Status == planpkg.StepRunning {
			phase = p
			break
		}
	}
	if phase == nil {
		allDone := true
		for _, p := range mp.Phases {
			if p.Status != planpkg.StepComplete {
				allDone = false
				break
			}
		}
		if allDone {
			mp.Complete = true
			mp.Summary = "All phases completed."
			state.MasterStore.Save(mp)
			log.Printf("agent: masterplan complete")
			a.saveCheckpoint(state.Session.Key)
		}
		return 2 * time.Second
	}

	if phase.Status == planpkg.StepPending {
		phase.Status = planpkg.StepRunning
		state.MasterStore.Save(mp)
		a.emit(state.Session.Key, eventbus.KindPhaseStarted, map[string]string{"phase": phase.ID})
		log.Printf("agent: starting phase %s", phase.ID)
	}

	subPlanStore := state.MasterStore.SubPlanStore(phase.ID)
	done, err := cycle.Supervisor(ctx, state.Context, a.res, subPlanStore, a, a.subAgents, state.Interventions)

	if err != nil {
		phase.Status = planpkg.StepFailed
		state.MasterStore.Save(mp)
		a.emit(state.Session.Key, eventbus.KindPhaseFailed, map[string]string{"phase": phase.ID, "error": err.Error()})
		log.Printf("agent: phase %s failed: %v", phase.ID, err)
		return 5 * time.Second
	}
	if done {
		phase.Status = planpkg.StepComplete
		state.MasterStore.Save(mp)
		a.emit(state.Session.Key, eventbus.KindPhaseCompleted, map[string]string{"phase": phase.ID})
		log.Printf("agent: phase %s complete", phase.ID)
	}

	return 2 * time.Second
}

// ─── Session CRUD ───────────────────────────────────────────────────────────

// CreateSession creates a new session and initializes its context.
func (a *Agent) CreateSession(t session.Type, title string) (*session.Session, error) {
	id := uuid.New().String()[:8]
	var key string
	switch t {
	case session.TypeChat:
		key = session.ChatKey(a.agentID, id)
	case session.TypeSubAgent:
		key = session.SubAgentKey(a.agentID, id)
	default:
		return nil, fmt.Errorf("cannot create session of type %s", t)
	}

	now := time.Now()
	sess := &session.Session{
		Key:       key,
		AgentID:   a.agentID,
		Type:      t,
		Status:    session.StatusActive,
		Title:     title,
		ParentKey: session.MainKey(a.agentID),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := a.sessStore.Create(sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	ac, err := a.buildFreshContext(agentcore.DefaultContextWindow, agentcore.ModeAuto)
	if err != nil {
		return nil, fmt.Errorf("build context: %w", err)
	}

	a.mu.Lock()
	a.sessions[key] = &SessionState{
		Session:       sess,
		Context:       ac,
		Mode:          agentcore.ModeAuto,
		Interventions: intervention.New(10),
	}
	a.mu.Unlock()

	a.emit(key, eventbus.KindSessionStarted, map[string]string{"type": string(t), "title": title})
	return sess, nil
}

// DeleteSession marks a session as deleted and removes its state.
func (a *Agent) DeleteSession(sessionKey string) error {
	// Cannot delete main session.
	mainKey := session.MainKey(a.agentID)
	if sessionKey == mainKey {
		return fmt.Errorf("cannot delete main session")
	}

	a.mu.Lock()
	state, ok := a.sessions[sessionKey]
	if ok {
		// Save snapshot before deleting.
		if state.Context != nil {
			SaveSessionSnapshot(a.res.KB, a.res.IDGen, sessionKey, state.Context)
		}
		delete(a.sessions, sessionKey)
	}
	a.mu.Unlock()

	// Update store.
	sess, err := a.sessStore.Get(sessionKey)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	now := time.Now()
	sess.Status = session.StatusDeleted
	sess.EndedAt = &now
	sess.UpdatedAt = now
	if err := a.sessStore.Update(sess); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	a.emit(sessionKey, eventbus.KindSessionEnded, map[string]string{"reason": "deleted"})
	return nil
}

// Intervene pushes a user message into a session's intervention queue.
// The message will be consumed at the next safe interruption point.
func (a *Agent) Intervene(sessionKey, content string, priority int) (string, int, error) {
	a.mu.RLock()
	state, ok := a.sessions[sessionKey]
	a.mu.RUnlock()
	if !ok {
		return "", 0, fmt.Errorf("session %q not found", sessionKey)
	}
	if state.Interventions == nil {
		return "", 0, fmt.Errorf("session %q has no intervention queue", sessionKey)
	}
	msg := intervention.Message{
		SessionID: sessionKey,
		Content:   content,
		Priority:  priority,
	}
	if err := state.Interventions.Push(msg); err != nil {
		return "", state.Interventions.Pending(), err
	}
	return msg.ID, state.Interventions.Pending(), nil
}

// BroadcastIntervene pushes a message to all active session queues.
// Used for "stop everything" style interventions.
func (a *Agent) BroadcastIntervene(content string, priority int) int {
	a.mu.RLock()
	sessions := make([]*SessionState, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s)
	}
	a.mu.RUnlock()

	count := 0
	for _, s := range sessions {
		if s.Interventions == nil {
			continue
		}
		msg := intervention.Message{
			SessionID: s.Session.Key,
			Content:   content,
			Priority:  priority,
		}
		if s.Interventions.Push(msg) == nil {
			count++
		}
	}
	return count
}

// ListSessions returns all sessions for this agent.
func (a *Agent) ListSessions() ([]*session.Session, error) {
	return a.sessStore.ListByAgent(a.agentID)
}

// GetSession returns a specific session by key.
func (a *Agent) GetSession(sessionKey string) (*session.Session, error) {
	return a.sessStore.Get(sessionKey)
}

// ─── Accessors ──────────────────────────────────────────────────────────────

// Mode returns the agent's current working mode (from main session).
func (a *Agent) Mode() agentcore.Mode {
	mainKey := session.MainKey(a.agentID)
	a.mu.RLock()
	state, ok := a.sessions[mainKey]
	a.mu.RUnlock()
	if !ok {
		return agentcore.ModeAuto
	}
	return state.Mode
}

// SetMode updates the main session's working mode.
func (a *Agent) SetMode(m agentcore.Mode) {
	mainKey := session.MainKey(a.agentID)
	a.mu.RLock()
	state, ok := a.sessions[mainKey]
	a.mu.RUnlock()
	if ok {
		state.Mode = m
	}
	a.res.KB.Set(agentcore.NSConfig, agentcore.KeyMode, []byte(string(m)))
}

// Provider returns the configured LLM provider identifier.
func (a *Agent) Provider() string { return a.res.GetGateway().Provider() }

// ModelName returns the configured model identifier.
func (a *Agent) ModelName() string { return a.res.GetGateway().ModelName() }

// Tools returns the names of all registered tools.
func (a *Agent) Tools() []string { return a.res.GetDispatcher().List() }

// BudgetStatus returns the token budget status for the given session.
func (a *Agent) BudgetStatus(sessionKey string) (*llmctx.BudgetStatus, error) {
	if sessionKey == "" {
		sessionKey = session.MainKey(a.agentID)
	}
	a.mu.RLock()
	state, ok := a.sessions[sessionKey]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionKey)
	}
	status := state.Context.BudgetStatus()
	return &status, nil
}

// ContextStats returns context metrics for the main session.
func (a *Agent) ContextStats() *llmctx.ContextStats {
	mainKey := session.MainKey(a.agentID)
	a.mu.RLock()
	state, ok := a.sessions[mainKey]
	a.mu.RUnlock()
	if !ok || state.Context == nil {
		return nil
	}
	s := state.Context.Stats()
	return &s
}

// ApprovePlan sets the current plan's status to "approved".
func (a *Agent) ApprovePlan() (*planpkg.Plan, error) {
	mainKey := session.MainKey(a.agentID)
	a.mu.RLock()
	state, ok := a.sessions[mainKey]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no main session")
	}

	planStore := state.PlanStore
	if planStore == nil {
		planStore = planpkg.NewStore(a.res.KB, agentcore.NSPlans, agentcore.KeyMasterPlan)
	}

	p, err := planStore.Load()
	if err != nil || p == nil {
		return nil, fmt.Errorf("no plan to approve")
	}
	if p.Status == planpkg.PlanApproved {
		return p, nil
	}
	p.Status = planpkg.PlanApproved
	if err := planStore.Save(p); err != nil {
		return nil, fmt.Errorf("save approved plan: %w", err)
	}
	a.emit(mainKey, eventbus.KindPlanUpdated, map[string]string{
		"status": string(planpkg.PlanApproved),
		"goal":   p.Goal,
	})
	return p, nil
}

// RejectPlan sets the current plan's status to "draft".
func (a *Agent) RejectPlan() error {
	mainKey := session.MainKey(a.agentID)
	a.mu.RLock()
	state, ok := a.sessions[mainKey]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no main session")
	}

	planStore := state.PlanStore
	if planStore == nil {
		planStore = planpkg.NewStore(a.res.KB, agentcore.NSPlans, agentcore.KeyMasterPlan)
	}

	p, err := planStore.Load()
	if err != nil || p == nil {
		return fmt.Errorf("no plan to reject")
	}
	if p.Status == planpkg.PlanDraft {
		return nil
	}
	p.Status = planpkg.PlanDraft
	if err := planStore.Save(p); err != nil {
		return fmt.Errorf("save rejected plan: %w", err)
	}
	a.emit(mainKey, eventbus.KindPlanUpdated, map[string]string{
		"status": string(planpkg.PlanDraft),
		"goal":   p.Goal,
	})
	return nil
}

// ─── Internal helpers ───────────────────────────────────────────────────────

func (a *Agent) buildFreshContext(windowSize int, mode agentcore.Mode) (*llmctx.AgentContext, error) {
	if windowSize <= 0 {
		windowSize = agentcore.DefaultContextWindow
	}
	window, _ := llmctx.NewContextWindow(int32(windowSize))

	compactor, err := buildCompactor()
	if err != nil {
		return nil, err
	}

	sysPromptText := chat.SystemPromptForMode(a.def, a.res, mode)
	sysID, _ := a.res.IDGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(a.def.Name)
	sysMsg, err := llmctx.NewSystemPromptMessage(sysID, agentAuthor, sysPromptText, a.res.TC)
	if err != nil {
		return nil, fmt.Errorf("system prompt: %w", err)
	}

	return llmctx.NewAgentContextBuilder().
		WithSystemPrompt(sysMsg).
		WithContextWindow(window).
		WithCompactor(compactor).
		WithTokenComputer(a.res.TC).
		WithIDGenerator(a.res.IDGen).
		Build()
}

func (a *Agent) updateSystemPrompt(state *SessionState, mode agentcore.Mode) {
	text := chat.SystemPromptForMode(a.def, a.res, mode)
	id, _ := a.res.IDGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(a.def.Name)
	msg, err := llmctx.NewSystemPromptMessage(id, agentAuthor, text, a.res.TC)
	if err != nil {
		return
	}
	state.Context.SetSystemPrompt(msg)
}

func (a *Agent) restoreMode() agentcore.Mode {
	if data, err := a.res.KB.Get(agentcore.NSConfig, agentcore.KeyMode); err == nil {
		if m, err := agentcore.ParseMode(string(data)); err == nil {
			return m
		}
	}
	return agentcore.ModeAuto
}

func (a *Agent) emitMessage(sessionKey, author, content, mode string) {
	truncated := content
	if len(truncated) > 4096 {
		truncated = truncated[:4096]
	}
	data := map[string]string{
		"author":  author,
		"content": truncated,
	}
	if mode != "" {
		data["mode"] = mode
	}
	a.emit(sessionKey, eventbus.KindMessageAdded, data)
}

func (a *Agent) emit(sessionKey string, kind eventbus.EventKind, data interface{}) {
	if a.res.EventBus == nil {
		return
	}
	id := fmt.Sprintf("%s-%d", kind, time.Now().UnixNano())
	a.res.EventBus.Emit(eventbus.Event{
		ID:        id,
		Kind:      kind,
		SessionID: sessionKey,
		Timestamp: time.Now().UTC(),
		Data:      data,
	})
}
