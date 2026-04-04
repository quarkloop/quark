package runtime

import (
	"log"

	"github.com/quarkloop/agent/pkg/agentcore"
	"github.com/quarkloop/agent/pkg/tool"
	cliplugin "github.com/quarkloop/cli/pkg/plugin"
	"github.com/quarkloop/cli/pkg/quarkfile"
)

type spaceConfig struct {
	provider   string
	modelName  string
	routing    *quarkfile.RoutingSection
	registry   *tool.Registry
	toolDefs   map[string]*tool.Definition
	skillPaths []string
	agentDefs  map[string]*cliplugin.Manifest
	qf         *quarkfile.Quarkfile // raw parsed Quarkfile, retained for diffing
}

// ConfigDiff describes what changed between two Quarkfile versions.
type ConfigDiff struct {
	ModelChanged       bool
	RoutingChanged     bool
	PluginsAdded       []string
	PluginsRemoved     []string
	PermissionsChanged bool
}

// IsEmpty reports whether the diff contains no changes.
func (d ConfigDiff) IsEmpty() bool {
	return !d.ModelChanged && !d.RoutingChanged &&
		len(d.PluginsAdded) == 0 && len(d.PluginsRemoved) == 0 &&
		!d.PermissionsChanged
}

// DiffQuarkfiles returns the set of changes between two Quarkfile versions.
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

	// Compare plugin refs.
	oldPlugins := make(map[string]bool, len(oldQF.Plugins))
	for _, p := range oldQF.Plugins {
		oldPlugins[p.Ref] = true
	}
	newPlugins := make(map[string]bool, len(newQF.Plugins))
	for _, p := range newQF.Plugins {
		newPlugins[p.Ref] = true
	}
	for ref := range newPlugins {
		if !oldPlugins[ref] {
			d.PluginsAdded = append(d.PluginsAdded, ref)
		}
	}
	for ref := range oldPlugins {
		if !newPlugins[ref] {
			d.PluginsRemoved = append(d.PluginsRemoved, ref)
		}
	}

	if !permissionsEqual(oldQF.Permissions, newQF.Permissions) {
		d.PermissionsChanged = true
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
		slicesEqual(a.Tools.Denied, b.Tools.Denied) &&
		a.Audit.LogToolCalls == b.Audit.LogToolCalls &&
		a.Audit.LogLLMResponses == b.Audit.LogLLMResponses &&
		a.Audit.RetentionDays == b.Audit.RetentionDays
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

func loadSpaceConfig(dir string, plugins []cliplugin.Plugin, qf *quarkfile.Quarkfile) (*spaceConfig, error) {
	_ = dir
	registry, toolDefs, skillPaths, agentDefs := buildFromPlugins(plugins)

	var routing *quarkfile.RoutingSection
	if len(qf.Routing.Rules) > 0 || len(qf.Routing.Fallback) > 0 {
		r := qf.Routing
		routing = &r
	}

	sc := &spaceConfig{
		provider:   qf.Model.Provider,
		modelName:  qf.Model.Name,
		routing:    routing,
		registry:   registry,
		toolDefs:   toolDefs,
		skillPaths: skillPaths,
		agentDefs:  agentDefs,
		qf:         qf,
	}

	toolNames := registry.List()
	agentCount := len(agentDefs)
	log.Printf("runtime: loaded space %q tools=%v skills=%d agents=%d",
		qf.Meta.Name, toolNames, len(skillPaths), agentCount)

	return sc, nil
}

// buildFromPlugins creates the tool registry, skill resolver roots, and
// agent definitions from a list of installed plugins.
func buildFromPlugins(plugins []cliplugin.Plugin) (*tool.Registry, map[string]*tool.Definition, []string, map[string]*cliplugin.Manifest) {
	registry := tool.NewRegistry()
	toolDefs := make(map[string]*tool.Definition)
	var skillPaths []string
	agentDefs := make(map[string]*cliplugin.Manifest)

	for _, p := range plugins {
		man := p.Manifest
		switch man.Type {
		case cliplugin.TypeTool:
			buildToolPlugin(p, registry, toolDefs)
		case cliplugin.TypeAgent:
			agentDefs[man.Name] = man
			if man.Prompt != "" {
				skillPaths = append(skillPaths, man.Prompt)
			}
		case cliplugin.TypeSkill:
			skillDir := p.Dir
			skillPaths = append(skillPaths, skillDir)
		}
	}

	return registry, toolDefs, skillPaths, agentDefs
}

// buildToolPlugin registers a tool plugin in the tool registry.
func buildToolPlugin(p cliplugin.Plugin, registry *tool.Registry, toolDefs map[string]*tool.Definition) {
	def := &tool.Definition{
		Ref:    p.Manifest.Repository,
		Name:   p.Manifest.Name,
		Config: make(map[string]string),
	}

	if len(p.Manifest.Interface.Mode) > 0 {
		for _, mode := range p.Manifest.Interface.Mode {
			if mode == "http" {
				def.Config["mode"] = "http"
			}
		}
		if p.Manifest.Interface.Endpoint != "" {
			def.Config["endpoint_path"] = p.Manifest.Interface.Endpoint
		}
		if len(p.Manifest.Interface.Commands) > 0 {
			def.Config["commands"] = p.Manifest.Interface.Commands[0]
		}
	}

	registry.Register(def.Name, def)
	toolDefs[def.Name] = def
	log.Printf("runtime: registered tool plugin %s (ref: %s)", def.Name, def.Ref)
}

// buildSupervisorDefFromPlugins builds the supervisor definition from
// agent plugins. The first agent plugin is used as supervisor.
func buildSupervisorDefFromPlugins(agentDefs map[string]*cliplugin.Manifest, qf *quarkfile.Quarkfile) *agentcore.Definition {
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
	for _, man := range agentDefs {
		if man.Prompt != "" {
			def.SystemPrompt = man.Prompt
			break
		}
	}
	return def
}
