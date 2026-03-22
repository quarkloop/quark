package agentcore

import (
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/tool"
	"github.com/quarkloop/core/pkg/kb"
)

// Resources holds the shared dependencies that all agent sub-packages need.
// It is constructed once by the runtime and passed to every package that
// participates in agent processing.
type Resources struct {
	KB         kb.Store
	Gateway    model.Gateway
	Dispatcher tool.Invoker
	AdapterReg *llmctx.AdapterRegistry
	TC         llmctx.TokenComputer
	IDGen      llmctx.IDGenerator
	VisPolicy  *llmctx.VisibilityPolicy
	Activity   activity.Sink
}
