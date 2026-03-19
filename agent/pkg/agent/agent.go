// Package agent implements the multi-agent execution engine.
//
// Agent coordinates the full ORIENT→PLAN→DISPATCH→MONITOR→ASSESS loop:
//
//   - The supervisor LLM call produces or updates the master Plan.
//   - Worker goroutines (one per ready step) call worker agents via the KB.
//   - Tool calls are dispatched through the skill.Invoker.
//   - The llmctx AgentContext manages the token window and compaction.
//
// Typical usage:
//
//	a := agent.NewAgent(def, kb, gw, disp, agent.WithSubAgents(agents))
//	a.InitContext(8192)
//	a.Run(ctx)
package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/quarkloop/agent/pkg/activity"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/model"
	planpkg "github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/session"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/core/pkg/kb"
)

// Agent drives the multi-agent loop inside a single space-runtime process.
// It owns the llmctx AgentContext and runs the
// ORIENT → PLAN → DISPATCH → MONITOR → ASSESS cycle, fanning out plan steps
// to worker goroutines. One Agent exists per running space.
//
// Usage: NewAgent → InitContext → Run.
type Agent struct {
	def             *Definition
	kb              kb.Store
	gateway         model.Gateway
	dispatcher      skill.Invoker
	subAgents       map[string]*Definition
	planStore       *planpkg.Store
	masterPlanStore *planpkg.MasterPlanStore
	mode            Mode
	mu              sync.Mutex
	activeSteps     map[string]struct{} // step IDs currently running

	// llmctx integration
	ctx        *llmctx.AgentContext
	tc         llmctx.TokenComputer
	idGen      llmctx.IDGenerator
	visPolicy  *llmctx.VisibilityPolicy
	adapterReg *llmctx.AdapterRegistry
	snapRepo   *KBSnapshotRepository

	// session & activity (optional)
	session      *session.Session
	sessionStore *session.Store
	activity     activity.Sink
}

// Option configures an Agent.
type Option func(*Agent)

// WithSubAgents registers worker agent definitions that the Agent can dispatch to.
func WithSubAgents(agents map[string]*Definition) Option {
	return func(a *Agent) {
		a.subAgents = agents
	}
}

// WithSession attaches a session and its store to the agent.
func WithSession(sess *session.Session, store *session.Store) Option {
	return func(a *Agent) {
		a.session = sess
		a.sessionStore = store
	}
}

// WithActivitySink attaches an activity sink for event streaming.
func WithActivitySink(sink activity.Sink) Option {
	return func(a *Agent) {
		a.activity = sink
	}
}

// WithMode sets the initial working mode for the agent.
func WithMode(m Mode) Option {
	return func(a *Agent) {
		a.mode = m
	}
}

// NewAgent constructs an Agent. InitContext must be called before Run.
func NewAgent(
	def *Definition,
	k kb.Store,
	gw model.Gateway,
	disp skill.Invoker,
	opts ...Option,
) *Agent {
	a := &Agent{
		def:             def,
		kb:              k,
		gateway:         gw,
		dispatcher:      disp,
		subAgents:       map[string]*Definition{},
		planStore:       planpkg.NewStore(k, NSPlans, KeyMasterPlan),
		masterPlanStore: planpkg.NewMasterPlanStore(k, NSPlans, KeyMasterPlanDoc),
		mode:            ModeAuto,
		activeSteps:     map[string]struct{}{},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// InitContext builds the llmctx AgentContext for the agent.
// It must be called exactly once before Run. Steps:
//
//  1. Create a CachedTokenComputer and IDGenerator.
//  2. Build the AdapterRegistry (all providers including noop).
//  3. Configure a VisibilityPolicy.
//  4. Restore a previous-session snapshot from the KB if available;
//     otherwise build a fresh context with the system prompt.
func (a *Agent) InitContext(windowSize int) error {
	tc, err := llmctx.DefaultTokenComputer()
	if err != nil {
		return fmt.Errorf("token computer: %w", err)
	}
	a.tc = tc
	a.idGen = llmctx.DefaultIDGenerator()

	a.adapterReg = llmctx.NewAdapterRegistry()

	a.visPolicy = llmctx.DefaultVisibilityPolicy()
	a.visPolicy.Set(llmctx.ToolCallMessageType, llmctx.VisibleToLLMAndDev)
	a.visPolicy.Set(llmctx.ToolResultMessageType, llmctx.VisibleToLLMAndDev)
	a.visPolicy.Set(llmctx.ReasoningMessageType, llmctx.VisibleToDeveloperOnly)
	a.visPolicy.Set(llmctx.PlanMessageType, llmctx.VisibleToDeveloperOnly)
	a.visPolicy.Set(llmctx.MemoryMessageType, llmctx.VisibleToLLMAndDev)

	a.snapRepo = NewKBSnapshotRepository(a.kb)
	a.restoreMode()

	// Try to restore context from a previous session.
	if snap, err := a.snapRepo.LoadLatestSnapshot(); err == nil {
		log.Printf("agent: restoring context from snapshot (%d messages)",
			len(snap.Messages))
		ac, err := llmctx.ContextFromSnapshot(snap, tc,
			llmctx.WithRestoreIDGenerator(a.idGen))
		if err == nil {
			a.ctx = ac
			return nil
		}
		log.Printf("agent: failed to restore snapshot, creating fresh: %v", err)
	}

	return a.buildFreshContext(windowSize)
}

// Mode returns the agent's current working mode.
func (a *Agent) Mode() Mode { return a.mode }

// SetMode updates the agent's working mode and persists it to the KB.
func (a *Agent) SetMode(m Mode) {
	a.mode = m
	a.kb.Set(NSConfig, KeyMode, []byte(string(m)))
}

// restoreMode loads the persisted mode from the KB. Falls back to ModeAuto.
func (a *Agent) restoreMode() {
	if data, err := a.kb.Get(NSConfig, KeyMode); err == nil {
		if m, err := ParseMode(string(data)); err == nil {
			a.mode = m
		}
	}
}

// buildFreshContext creates a new AgentContext.
func (a *Agent) buildFreshContext(windowSize int) error {
	window, _ := llmctx.NewContextWindow(int32(windowSize))

	pipeline, err := llmctx.NewPipelineCompactor(
		llmctx.NewWeightBasedCompactor(),
		llmctx.NewFIFOCompactor(),
	)
	if err != nil {
		return fmt.Errorf("pipeline compactor: %w", err)
	}
	compactor, err := llmctx.NewThresholdCompactor(pipeline, DefaultCompactionThreshold)
	if err != nil {
		return fmt.Errorf("threshold compactor: %w", err)
	}

	sysPromptText := a.systemPromptForMode(a.mode)
	sysID, _ := a.idGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(a.def.Name)
	sysMsg, err := llmctx.NewSystemPromptMessage(sysID, agentAuthor, sysPromptText, a.tc)
	if err != nil {
		return fmt.Errorf("system prompt: %w", err)
	}

	ac, err := llmctx.NewAgentContextBuilder().
		WithSystemPrompt(sysMsg).
		WithContextWindow(window).
		WithCompactor(compactor).
		WithTokenComputer(a.tc).
		WithIDGenerator(a.idGen).
		Build()
	if err != nil {
		return fmt.Errorf("agent context: %w", err)
	}
	a.ctx = ac
	return nil
}

// emit sends an activity event if a sink is configured.
func (a *Agent) emit(eventType activity.EventType, data interface{}) {
	if a.activity == nil {
		return
	}
	sessionID := ""
	if a.session != nil {
		sessionID = a.session.ID
	}
	id := fmt.Sprintf("%s-%d", eventType, time.Now().UnixNano())
	a.activity.Emit(activity.Event{
		ID:        id,
		SessionID: sessionID,
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Data:      data,
	})
}

// Run starts the agent loop, blocking until ctx is cancelled.
//
// Behaviour depends on the current working mode:
//   - Ask / Auto: idle — all interaction happens via Chat().
//   - Plan: when an approved plan exists, the LLM-driven supervisorCycle
//     (ORIENT→PLAN→DISPATCH→MONITOR→ASSESS) executes it. Otherwise idle.
//   - MasterPlan: same as Plan but orchestrates phases — each phase is
//     executed as a sub-plan via supervisorCycle.
//
// Must be called after InitContext.
func (a *Agent) Run(ctx context.Context) error {
	log.Printf("agent: starting (name=%s mode=%s model=%s/%s)",
		a.def.Name, a.mode, a.gateway.Provider(), a.gateway.ModelName())

	a.emit(activity.SessionStarted, map[string]string{
		"agent": a.def.Name,
		"mode":  string(a.mode),
	})

	for {
		select {
		case <-ctx.Done():
			a.saveCheckpoint()
			a.emit(activity.SessionEnded, map[string]string{"reason": "cancelled"})
			return ctx.Err()
		default:
		}

		interval := a.runOnce(ctx)

		select {
		case <-ctx.Done():
			a.saveCheckpoint()
			a.emit(activity.SessionEnded, map[string]string{"reason": "cancelled"})
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// runOnce executes one iteration of the Run loop. Returns the interval to
// wait before the next iteration.
func (a *Agent) runOnce(ctx context.Context) time.Duration {
	switch a.mode {
	case ModePlan:
		return a.runPlanCycle(ctx)
	case ModeMasterPlan:
		return a.runMasterPlanCycle(ctx)
	default:
		// Ask / Auto: idle, all work via Chat().
		return 2 * time.Second
	}
}

// runPlanCycle checks for an approved plan and runs one supervisorCycle if found.
func (a *Agent) runPlanCycle(ctx context.Context) time.Duration {
	p, err := a.planStore.Load()
	if err != nil || p == nil {
		return 2 * time.Second // no plan, idle
	}
	if p.Complete {
		return 2 * time.Second // done, idle
	}
	if p.Status != planpkg.PlanApproved {
		return 2 * time.Second // draft, waiting for approval
	}

	done, err := a.supervisorCycle(ctx)
	if err != nil {
		log.Printf("agent: plan cycle error: %v", err)
		return 5 * time.Second
	}
	if done {
		log.Printf("agent: plan complete")
		a.saveCheckpoint()
	}
	return 2 * time.Second
}

// runMasterPlanCycle checks for an approved masterplan and executes the next
// ready phase via supervisorCycle. Each phase's sub-plan is a standard Plan
// managed through a per-phase planStore.
func (a *Agent) runMasterPlanCycle(ctx context.Context) time.Duration {
	mp, err := a.masterPlanStore.Load()
	if err != nil || mp == nil {
		return 2 * time.Second
	}
	if mp.Complete {
		return 2 * time.Second
	}
	if mp.Status != planpkg.PlanApproved {
		return 2 * time.Second
	}

	// Find next ready phase.
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
		// Check if all done.
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
			a.masterPlanStore.Save(mp)
			log.Printf("agent: masterplan complete")
			a.saveCheckpoint()
		}
		return 2 * time.Second
	}

	// Mark phase running if still pending.
	if phase.Status == planpkg.StepPending {
		phase.Status = planpkg.StepRunning
		a.masterPlanStore.Save(mp)
		a.emit(activity.PhaseStarted, map[string]string{"phase": phase.ID})
		log.Printf("agent: starting phase %s", phase.ID)
	}

	// Swap planStore to this phase's sub-plan, run one supervisor cycle.
	origStore := a.planStore
	a.planStore = a.masterPlanStore.SubPlanStore(phase.ID)

	done, err := a.supervisorCycle(ctx)

	a.planStore = origStore

	if err != nil {
		phase.Status = planpkg.StepFailed
		a.masterPlanStore.Save(mp)
		a.emit(activity.PhaseFailed, map[string]string{"phase": phase.ID, "error": err.Error()})
		log.Printf("agent: phase %s failed: %v", phase.ID, err)
		return 5 * time.Second
	}
	if done {
		phase.Status = planpkg.StepComplete
		a.masterPlanStore.Save(mp)
		a.emit(activity.PhaseCompleted, map[string]string{"phase": phase.ID})
		log.Printf("agent: phase %s complete", phase.ID)
	}

	return 2 * time.Second
}

// ExecuteTask runs a single task through the tool loop without the supervisor
// cycle. Useful for one-shot operations.
func (a *Agent) ExecuteTask(ctx context.Context, task string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("agent not initialised: call InitContext first")
	}
	resp, err := a.inferWithContext(ctx, a.ctx, task)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ContextStats returns an immutable snapshot of the AgentContext metrics
// (token counts, pressure level, compaction history).
// Returns nil before InitContext has been called.
func (a *Agent) ContextStats() *llmctx.ContextStats {
	if a.ctx == nil {
		return nil
	}
	s := a.ctx.Stats()
	return &s
}
