package agentcore

import (
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
type Resources struct {
	KB            kb.Store
	ConfigStore   *config.Store
	EventBus      *eventbus.Bus
	Activity      *activity.Writer
	Hooks         *hooks.Registry
	SkillResolver *skill.Resolver
	PluginManager *plugin.Manager
	Gateway       model.Gateway
	Dispatcher    tool.Invoker
	AdapterReg    *llmctx.AdapterRegistry
	TC            llmctx.TokenComputer
	IDGen         llmctx.IDGenerator
	VisPolicy     *llmctx.VisibilityPolicy
	Permissions   Permissions
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
