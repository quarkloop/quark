// Package agent implements the multi-agent execution engine.
//
// Executor coordinates the full ORIENT→PLAN→DISPATCH→MONITOR→ASSESS loop:
//
//   - The supervisor LLM call produces or updates the master Plan.
//   - Worker goroutines (one per ready step) call worker agents via the KB.
//   - Tool calls are dispatched through the skill.Invoker.
//   - The llmctx AgentContext manages the token window and compaction.
//
// Typical usage (done by space/builder.go):
//
//	exec := agent.NewExecutor(kb, gw, disp, engine, supervisor, agents)
//	exec.InitContext(8192)
//	exec.Run(ctx)
package agent

import (
	"context"
	"fmt"
	"log"
	"sync"

	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/kb/pkg/kb"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/skill"
)

// Executor coordinates the supervisor and worker agent lifecycle inside a
// single space-runtime process. It is not safe for concurrent use from
// multiple goroutines.
// Executor drives the multi-agent loop inside a single space-runtime process.
// It owns the supervisor's llmctx AgentContext and runs the
// ORIENT → PLAN → DISPATCH → MONITOR → ASSESS cycle, fanning out plan steps
// to worker goroutines. One Executor exists per running space.
//
// Usage: NewExecutor → InitContext → Run.
type Executor struct {
	kb          kb.Store
	gateway     model.Gateway
	dispatcher  skill.Invoker
	engine      Engine
	supervisor  *Definition
	agents      map[string]*Definition
	mu          sync.Mutex
	activeSteps map[string]struct{} // step IDs currently running

	// llmctx integration
	supervisorCtx *llmctx.AgentContext
	tc            llmctx.TokenComputer
	idGen         llmctx.IDGenerator
	visPolicy     *llmctx.VisibilityPolicy
	adapterReg    *llmctx.AdapterRegistry
	snapRepo      *KBSnapshotRepository
}

// NewExecutor constructs an Executor. InitContext must be called before Run;
// the Executor is unusable until then.
// NewExecutor creates an Executor wired to the given dependencies.
// Call InitContext before Run.
func NewExecutor(
	k kb.Store,
	gw model.Gateway,
	disp skill.Invoker,
	engine Engine,
	supervisor *Definition,
	agents map[string]*Definition,
) *Executor {
	return &Executor{
		kb:          k,
		gateway:     gw,
		dispatcher:  disp,
		engine:      engine,
		supervisor:  supervisor,
		agents:      agents,
		activeSteps: map[string]struct{}{},
	}
}

// InitContext builds the llmctx AgentContext for the supervisor agent.
// It must be called exactly once before Run. Steps:
//
//  1. Create a CachedTokenComputer and IDGenerator.
//  2. Build the AdapterRegistry (all providers including noop).
//  3. Configure a VisibilityPolicy (tool calls + memory visible to LLM;
//     plan + reasoning visible to developer only).
//  4. Restore a previous-session snapshot from the KB if available;
//     otherwise build a fresh context with the supervisor system prompt.
func (e *Executor) InitContext(windowSize int) error {
	tc, err := llmctx.DefaultTokenComputer()
	if err != nil {
		return fmt.Errorf("token computer: %w", err)
	}
	e.tc = tc
	e.idGen = llmctx.DefaultIDGenerator()

	e.adapterReg = llmctx.NewAdapterRegistry()

	e.visPolicy = llmctx.DefaultVisibilityPolicy()
	e.visPolicy.Set(llmctx.ToolCallMessageType, llmctx.VisibleToLLMAndDev)
	e.visPolicy.Set(llmctx.ToolResultMessageType, llmctx.VisibleToLLMAndDev)
	e.visPolicy.Set(llmctx.ReasoningMessageType, llmctx.VisibleToDeveloperOnly)
	e.visPolicy.Set(llmctx.PlanMessageType, llmctx.VisibleToDeveloperOnly)
	e.visPolicy.Set(llmctx.MemoryMessageType, llmctx.VisibleToLLMAndDev)

	e.snapRepo = NewKBSnapshotRepository(e.kb)

	// Try to restore supervisor context from a previous session.
	if snap, err := e.snapRepo.LoadLatestSnapshot(); err == nil {
		log.Printf("executor: restoring supervisor context from snapshot (%d messages)",
			len(snap.Messages))
		ac, err := llmctx.ContextFromSnapshot(snap, tc,
			llmctx.WithRestoreIDGenerator(e.idGen))
		if err == nil {
			e.supervisorCtx = ac
			return nil
		}
		log.Printf("executor: failed to restore snapshot, creating fresh: %v", err)
	}

	return e.buildFreshSupervisorContext(windowSize)
}

// buildFreshSupervisorContext creates a new AgentContext for the supervisor.
func (e *Executor) buildFreshSupervisorContext(windowSize int) error {
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

	sysPromptText := e.buildSupervisorSystemPrompt(nil)
	sysID, _ := e.idGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(AuthorSupervisor)
	sysMsg, err := llmctx.NewSystemPromptMessage(sysID, agentAuthor, sysPromptText, e.tc)
	if err != nil {
		return fmt.Errorf("system prompt: %w", err)
	}

	ac, err := llmctx.NewAgentContextBuilder().
		WithSystemPrompt(sysMsg).
		WithContextWindow(window).
		WithCompactor(compactor).
		WithTokenComputer(e.tc).
		WithIDGenerator(e.idGen).
		Build()
	if err != nil {
		return fmt.Errorf("agent context: %w", err)
	}
	e.supervisorCtx = ac
	return nil
}

// Run starts the supervisor engine loop, blocking until the goal is complete
// or ctx is cancelled. Must be called after InitContext.
func (e *Executor) Run(ctx context.Context) error {
	log.Printf("executor: starting engine (supervisor=%s model=%s/%s)",
		e.supervisor.Name, e.gateway.Provider(), e.gateway.ModelName())

	if e.engine == nil {
		return fmt.Errorf("executor: no engine configured")
	}
	return e.engine.Run(ctx, e)
}

// ContextStats returns an immutable snapshot of the supervisor AgentContext
// metrics (token counts, pressure level, compaction history).
// Returns nil before InitContext has been called.
func (e *Executor) ContextStats() *llmctx.ContextStats {
	if e.supervisorCtx == nil {
		return nil
	}
	s := e.supervisorCtx.Stats()
	return &s
}
