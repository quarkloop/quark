package agent

import (
	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/intervention"
	"github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/session"
)

// SessionState holds the per-session state: context window, plan stores, mode.
// It is a data struct with no business logic methods.
type SessionState struct {
	Session       *session.Session
	Context       *llmctx.AgentContext
	Mode          agentcore.Mode
	PlanStore     *plan.Store
	MasterStore   *plan.MasterPlanStore
	Interventions *intervention.Queue
}
