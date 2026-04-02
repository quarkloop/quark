package agentcore

import (
	"sync"

	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/config"
	llmctx "github.com/quarkloop/agent/pkg/context"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/hooks"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/plugin"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/agent/pkg/tool"
	"github.com/quarkloop/core/pkg/kb"
)

// Resources holds the shared dependencies that all agent sub-packages need.
// It is constructed once by the runtime and passed to every package that
// participates in agent processing.
//
// The Gateway, Dispatcher, and Permissions fields support live hot-swap via
// SwapGateway/SwapDispatcher/SwapPermissions. All concurrent callers must
// access these three fields through their Get* accessor methods.
type Resources struct {
	KB            kb.Store
	ConfigStore   *config.Store
	EventBus      *eventbus.Bus
	Activity      *activity.Writer
	Hooks         *hooks.Registry
	SkillResolver *skill.Resolver
	PluginManager *plugin.Manager
	AdapterReg    *llmctx.AdapterRegistry
	TC            llmctx.TokenComputer
	IDGen         llmctx.IDGenerator
	VisPolicy     *llmctx.VisibilityPolicy

	// hot-swappable fields — access via GetGateway/GetDispatcher/GetPermissions.
	mu          sync.RWMutex
	Gateway     model.Gateway // initialize directly; use GetGateway() for concurrent reads
	Dispatcher  tool.Invoker  // initialize directly; use GetDispatcher() for concurrent reads
	Permissions Permissions   // initialize directly; use GetPermissions() for concurrent reads
}

// GetGateway returns the current LLM gateway. Safe for concurrent use.
func (r *Resources) GetGateway() model.Gateway {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Gateway
}

// SwapGateway atomically replaces the LLM gateway. Used during hot-reload.
func (r *Resources) SwapGateway(gw model.Gateway) {
	r.mu.Lock()
	r.Gateway = gw
	r.mu.Unlock()
}

// GetDispatcher returns the current tool dispatcher. Safe for concurrent use.
func (r *Resources) GetDispatcher() tool.Invoker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Dispatcher
}

// SwapDispatcher atomically replaces the tool dispatcher. Used during hot-reload.
func (r *Resources) SwapDispatcher(d tool.Invoker) {
	r.mu.Lock()
	r.Dispatcher = d
	r.mu.Unlock()
}

// GetPermissions returns the current permissions snapshot. Safe for concurrent use.
func (r *Resources) GetPermissions() Permissions {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Permissions
}

// SwapPermissions atomically replaces the permissions. Used during hot-reload.
func (r *Resources) SwapPermissions(p Permissions) {
	r.mu.Lock()
	r.Permissions = p
	r.mu.Unlock()
}

// Permissions mirrors the Quarkfile permissions block at runtime.
// This is the single source of truth for permission checks across all
// agent sub-packages.
type Permissions struct {
	Filesystem FilesystemPermissions
	Network    NetworkPermissions
	Tools      ToolPermissions
	Plugins    PluginPermissions
	Audit      AuditPermissions
}

// FilesystemPermissions controls which paths the agent can access.
type FilesystemPermissions struct {
	AllowedPaths []string
	ReadOnly     []string
}

// NetworkPermissions controls which hosts the agent can reach.
type NetworkPermissions struct {
	AllowedHosts []string
	Deny         []string
}

// ToolPermissions controls which tools the agent can invoke.
type ToolPermissions struct {
	Allowed []string
	Denied  []string
}

// PluginPermissions controls which plugins the agent can load.
type PluginPermissions struct {
	Allowed     []string
	AutoInstall bool
}

// AuditPermissions controls logging and retention policies.
type AuditPermissions struct {
	LogToolCalls    bool
	LogLLMResponses bool
	RetentionDays   int
}
