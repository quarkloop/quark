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

	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/plan"
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
	def        *Definition
	kb         kb.Store
	gateway    model.Gateway
	dispatcher skill.Invoker
	subAgents  map[string]*Definition
	planStore  *plan.Store
	mu         sync.Mutex
	activeSteps map[string]struct{} // step IDs currently running

	// llmctx integration
	ctx       *llmctx.AgentContext
	tc        llmctx.TokenComputer
	idGen     llmctx.IDGenerator
	visPolicy *llmctx.VisibilityPolicy
	adapterReg *llmctx.AdapterRegistry
	snapRepo  *KBSnapshotRepository
}

// Option configures an Agent.
type Option func(*Agent)

// WithSubAgents registers worker agent definitions that the Agent can dispatch to.
func WithSubAgents(agents map[string]*Definition) Option {
	return func(a *Agent) {
		a.subAgents = agents
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
		def:         def,
		kb:          k,
		gateway:     gw,
		dispatcher:  disp,
		subAgents:   map[string]*Definition{},
		planStore:   plan.NewStore(k, NSPlans, KeyMasterPlan),
		activeSteps: map[string]struct{}{},
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

	sysPromptText := a.buildSupervisorSystemPrompt(nil)
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

// Run starts the supervisor cycle loop, blocking until the goal is complete
// or ctx is cancelled. Must be called after InitContext.
func (a *Agent) Run(ctx context.Context) error {
	log.Printf("agent: starting (name=%s model=%s/%s)",
		a.def.Name, a.gateway.Provider(), a.gateway.ModelName())

	for {
		select {
		case <-ctx.Done():
			a.saveCheckpoint()
			return ctx.Err()
		default:
		}
		done, err := a.supervisorCycle(ctx)
		if err != nil {
			log.Printf("agent: cycle error: %v", err)
			select {
			case <-ctx.Done():
				a.saveCheckpoint()
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
			continue
		}
		if done {
			log.Printf("agent: goal complete")
			a.saveCheckpoint()
			return nil
		}
		a.saveCheckpoint()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
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
