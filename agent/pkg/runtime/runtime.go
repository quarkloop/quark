// Package runtime implements the agent runtime process.
//
// A runtime is a long-lived process that:
//   - Opens the space knowledge base.
//   - Loads the Quarkfile and lock file.
//   - Resolves agent, tool, and model configuration.
//   - Runs the Agent loop.
//   - Serves the standardized agent HTTP API.
//   - Optionally reports health to the api-server.
package runtime

import (
	"bytes"
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
	"github.com/quarkloop/agent/pkg/channel"
	"github.com/quarkloop/agent/pkg/config"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/hooks"
	"github.com/quarkloop/agent/pkg/infra/httpserver"
	"github.com/quarkloop/agent/pkg/infra/watcher"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/plugin"
	"github.com/quarkloop/agent/pkg/resolver"
	"github.com/quarkloop/agent/pkg/session"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/agent/pkg/tool"
	"github.com/quarkloop/core/pkg/kb"
	"github.com/quarkloop/tools/space/pkg/quarkfile"
)

type Config struct {
	AgentID   string
	Dir       string
	Port      int
	APIServer string
	BinDir    string // directory containing tool binaries; auto-detected if empty
	AutoTools bool   // if true, start tool processes automatically
}

type Runtime struct {
	cfg          *Config
	kb           kb.Store
	registry     *agent.Registry
	resolver     resolver.Resolver
	chManager    *channel.Manager
	bus          *eventbus.Bus
	actWriter    *activity.Writer
	httpSrv      *httpserver.Server
	orchestrator *ToolOrchestrator
	res          *agentcore.Resources    // retained for hot-swap
	currentQF    *quarkfile.Quarkfile    // last successfully loaded Quarkfile
	cfgStore     *config.Store           // retained for reload
}

func New(cfg *Config) (*Runtime, error) {
	absDir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve dir: %w", err)
	}
	cfg.Dir = absDir

	k, err := kb.Open(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("open kb: %w", err)
	}

	spaceCfg, err := loadSpaceConfig(cfg.Dir, k)
	if err != nil {
		k.Close()
		return nil, err
	}

	// Auto-start tool processes if configured.
	var orchestrator *ToolOrchestrator
	if cfg.AutoTools {
		binDir := cfg.BinDir
		if binDir == "" {
			binDir = detectBinDir()
		}
		if binDir != "" {
			orchestrator = NewToolOrchestrator(binDir, cfg.Dir)
			startToolProcesses(orchestrator, spaceCfg)
		}
	}

	cfgStore := config.New(k)

	gw, err := buildGateway(spaceCfg, cfgStore)
	if err != nil {
		if orchestrator != nil {
			orchestrator.Shutdown()
		}
		k.Close()
		return nil, fmt.Errorf("build gateway: %w", err)
	}

	bus := eventbus.New()
	actWriter := activity.NewWriter(bus, k, 1024)
	actWriter.Start()
	hookReg := hooks.New()
	sessStore := session.NewStore(k)

	// Auto-configure model if not set via env vars or Quarkfile.
	if spaceCfg.provider == "" || spaceCfg.modelName == "" {
		provider, modelName := resolveModelFromEnv()
		if provider != "" && modelName != "" {
			if err := cfgStore.SetByAgent("model_provider", provider); err == nil {
				cfgStore.SetByAgent("model_name", modelName)
			}
			spaceCfg.provider = provider
			spaceCfg.modelName = modelName
		}
	}

	// Write auto-detected settings to config store.
	if err := autoConfigure(cfgStore, spaceCfg); err != nil {
		log.Printf("runtime: auto-configure warning: %v", err)
	}

	// Build skill resolver with space and builtin roots.
	spaceSkillsDir := filepath.Join(cfg.Dir, ".quark", "skills")
	skillResolver := skill.NewResolver(
		skill.Root{Dir: spaceSkillsDir, Source: skill.SourceSpace, Priority: 3},
	)

	// Create channel manager and register the Web UI channel.
	chManager := channel.NewManager(bus)
	webCh := channel.NewWebChannel(nil) // no allowlist for web
	if err := chManager.Register(webCh); err != nil {
		log.Printf("runtime: failed to register web channel: %v", err)
	}

	// Create plugin manager.
	pluginMgr := plugin.NewManager()

	res := &agentcore.Resources{
		KB:            k,
		ConfigStore:   cfgStore,
		EventBus:      bus,
		Activity:      actWriter,
		Hooks:         hookReg,
		SkillResolver: skillResolver,
		PluginManager: pluginMgr,
		Gateway:       gw,
		Dispatcher:    spaceCfg.registry,
	}

	// Register built-in security and audit hooks.
	hooks.RegisterBuiltInHooks(hookReg, hooks.ToolPermissions{
		Allowed: nil,
		Denied:  nil,
	}, hooks.FilesystemPermissions{
		AllowedPaths: nil,
		ReadOnly:     nil,
	}, bus)

	// Create agent registry and register the supervisor agent.
	registry := agent.NewRegistry()
	a := agent.New(cfg.AgentID, spaceCfg.supervisor, res, sessStore, spaceCfg.subAgents)
	if err := a.Init(); err != nil {
		if orchestrator != nil {
			orchestrator.Shutdown()
		}
		k.Close()
		return nil, fmt.Errorf("initialise agent: %w", err)
	}
	registry.Register(cfg.AgentID, a)

	// Build the resolver chain: session affinity → fallback to supervisor.
	chain := resolver.NewChainResolver(
		&resolver.SessionAffinityResolver{},
		resolver.NewFallbackResolver(cfg.AgentID),
	)

	mux := http.NewServeMux()
	agentapi.NewHandler(
		newAgentService(cfg.Dir, k, registry, chain, bus, actWriter, sessStore),
		agentapi.WithBasePath(agentapi.DefaultBasePath),
		agentapi.WithAliasBasePath(""),
	).RegisterRoutes(mux)
	srv := httpserver.New("127.0.0.1", cfg.Port, mux)

	return &Runtime{
		cfg:          cfg,
		kb:           k,
		registry:     registry,
		resolver:     chain,
		chManager:    chManager,
		bus:          bus,
		actWriter:    actWriter,
		httpSrv:      srv,
		orchestrator: orchestrator,
		res:          res,
		currentQF:    spaceCfg.qf,
		cfgStore:     cfgStore,
	}, nil
}

func (r *Runtime) Run(ctx context.Context) error {
	go func() {
		if err := r.httpSrv.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("runtime %s: http server: %v", r.cfg.AgentID, err)
		}
	}()

	healthURL := fmt.Sprintf("http://127.0.0.1:%d%s", r.cfg.Port, agentapi.JoinPath(agentapi.DefaultBasePath, agentapi.PathHealth))
	if err := waitHTTP(healthURL, 10*time.Second); err != nil {
		log.Printf("runtime %s: http not ready: %v", r.cfg.AgentID, err)
	}
	if err := r.reportHealth(); err != nil {
		log.Printf("runtime %s: health report failed: %v", r.cfg.AgentID, err)
	}
	go r.heartbeat(ctx)

	log.Printf("runtime %s ready on port %d", r.cfg.AgentID, r.cfg.Port)

	// Start all registered channels.
	if err := r.chManager.StartAll(ctx); err != nil {
		log.Printf("runtime %s: channel start error: %v", r.cfg.AgentID, err)
	}

	// Watch Quarkfile for hot-reload.
	qfPath := filepath.Join(r.cfg.Dir, quarkfile.QuarkfileFilename)
	w := watcher.New(func(path string) {
		if err := r.reload(ctx); err != nil {
			log.Printf("runtime %s: hot-reload failed: %v", r.cfg.AgentID, err)
		}
	}, 0, qfPath)
	go w.Start(ctx)

	// Run all registered agents.
	for _, agentID := range r.registry.List() {
		a, _ := r.registry.Get(agentID)
		go func(id string, ag *agent.Agent) {
			if err := ag.Run(ctx); err != nil {
				log.Printf("runtime %s: agent loop error: %v", id, err)
			}
		}(agentID, a)
	}

	// Block until context is cancelled.
	<-ctx.Done()
	return ctx.Err()
}

func (r *Runtime) Close() {
	if r.chManager != nil {
		r.chManager.StopAll(context.Background())
	}
	_ = r.httpSrv.Shutdown(context.Background())
	if r.actWriter != nil {
		r.actWriter.Stop()
	}
	if r.orchestrator != nil {
		r.orchestrator.Shutdown()
	}
	_ = r.kb.Close()
}

func (r *Runtime) reportHealth() error {
	if r.cfg.APIServer == "" {
		return nil
	}

	type healthReport struct {
		AgentID string `json:"agent_id"`
		PID     int    `json:"pid"`
		Port    int    `json:"port"`
	}
	body, err := json.Marshal(healthReport{
		AgentID: r.cfg.AgentID,
		PID:     os.Getpid(),
		Port:    r.cfg.Port,
	})
	if err != nil {
		return fmt.Errorf("marshal health report: %w", err)
	}
	url := fmt.Sprintf("%s/api/v1/agents/%s/health", r.cfg.APIServer, r.cfg.AgentID)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (r *Runtime) heartbeat(ctx context.Context) {
	if r.cfg.APIServer == "" {
		return
	}

	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := r.reportHealth(); err != nil {
				log.Printf("runtime %s: heartbeat failed: %v", r.cfg.AgentID, err)
			}
		}
	}
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", url)
}

func buildGateway(spaceCfg *spaceConfig, cfgStore *config.Store) (model.Gateway, error) {
	if os.Getenv("QUARK_DRY_RUN") == "1" {
		log.Printf("runtime: QUARK_DRY_RUN=1, using noop model gateway")
		return model.New(model.GatewayConfig{Provider: "noop", Model: "noop"})
	}

	// Build primary gateway.
	provider := stringsFirstNonEmpty(os.Getenv("QUARK_MODEL_PROVIDER"), spaceCfg.provider)
	modelName := stringsFirstNonEmpty(os.Getenv("QUARK_MODEL_NAME"), spaceCfg.modelName)
	if provider == "" {
		provider = "noop"
		modelName = "noop"
	}

	primaryGw, err := buildSingleGateway(provider, modelName)
	if err != nil {
		return nil, err
	}

	// Wrap primary with fallback chain.
	fallbackGws := buildFallbackChain(cfgStore, spaceCfg)
	var defaultGw model.Gateway = primaryGw
	if len(fallbackGws) > 0 {
		defaultGw = model.NewFallbackGateway(primaryGw, fallbackGws...)
	}

	// Wrap with routing gateway if routing rules are configured.
	rules := buildRoutingRules(cfgStore, spaceCfg)
	if len(rules) > 0 {
		return model.NewRoutingGateway(rules, defaultGw), nil
	}

	return defaultGw, nil
}

func buildSingleGateway(provider, modelName string) (model.Gateway, error) {
	var apiKey string
	switch provider {
	case "anthropic":
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	case "openai":
		apiKey = os.Getenv("OPENAI_API_KEY")
	case "zhipu":
		apiKey = os.Getenv("ZHIPU_API_KEY")
	case "openrouter":
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}

	return model.New(model.GatewayConfig{
		Provider: provider,
		Model:    modelName,
		APIKey:   apiKey,
	})
}

// buildFallbackChain creates the ordered list of fallback gateways.
// DynamicConfig takes precedence over Quarkfile routing config.
func buildFallbackChain(cfgStore *config.Store, spaceCfg *spaceConfig) []model.Gateway {
	// Prefer dynamic config (owner-set) fallback chain.
	if cfgStore != nil {
		cfg, err := cfgStore.Load()
		if err == nil && len(cfg.Routing.Fallback) > 0 {
			return buildGatewaysFromModelConfigs(cfg.Routing.Fallback)
		}
	}

	// Fall back to Quarkfile routing section.
	if spaceCfg != nil && spaceCfg.routing != nil {
		var refs []config.ModelConfig
		for _, fb := range spaceCfg.routing.Fallback {
			refs = append(refs, config.ModelConfig{Provider: fb.Provider, Name: fb.Model})
		}
		return buildGatewaysFromModelConfigs(refs)
	}

	return nil
}

// buildRoutingRules creates model routing rules from dynamic config or Quarkfile.
// DynamicConfig takes precedence over Quarkfile routing config.
func buildRoutingRules(cfgStore *config.Store, spaceCfg *spaceConfig) []model.RoutingRule {
	// Prefer dynamic config (owner-set) routing rules.
	if cfgStore != nil {
		cfg, err := cfgStore.Load()
		if err == nil && len(cfg.Routing.Rules) > 0 {
			return buildRulesFromConfigs(cfg.Routing.Rules)
		}
	}

	// Fall back to Quarkfile routing section.
	if spaceCfg != nil && spaceCfg.routing != nil {
		var ruleConfigs []config.RoutingRuleConfig
		for _, r := range spaceCfg.routing.Rules {
			ruleConfigs = append(ruleConfigs, config.RoutingRuleConfig{
				Match:    r.Match,
				Provider: r.Provider,
				Name:     r.Model,
			})
		}
		return buildRulesFromConfigs(ruleConfigs)
	}

	return nil
}

func buildGatewaysFromModelConfigs(refs []config.ModelConfig) []model.Gateway {
	var gws []model.Gateway
	for _, ref := range refs {
		gw, err := buildSingleGateway(ref.Provider, ref.Name)
		if err != nil {
			log.Printf("runtime: skipping fallback %s/%s: %v", ref.Provider, ref.Name, err)
			continue
		}
		gws = append(gws, gw)
	}
	return gws
}

func buildRulesFromConfigs(ruleConfigs []config.RoutingRuleConfig) []model.RoutingRule {
	var rules []model.RoutingRule
	for _, rc := range ruleConfigs {
		gw, err := buildSingleGateway(rc.Provider, rc.Name)
		if err != nil {
			log.Printf("runtime: skipping routing rule %q→%s/%s: %v", rc.Match, rc.Provider, rc.Name, err)
			continue
		}
		rules = append(rules, model.RoutingRule{
			Match:   rc.Match,
			Gateway: gw,
		})
	}
	return rules
}

func stringsFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// detectBinDir finds the directory containing tool binaries by checking:
// 1. QUARK_BIN_DIR env var
// 2. The directory containing the current executable
func detectBinDir() string {
	if dir := os.Getenv("QUARK_BIN_DIR"); dir != "" {
		return dir
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// startToolProcesses starts tool processes for each tool in the space config.
// Failures are logged but non-fatal — the tool may already be running externally.
func startToolProcesses(orch *ToolOrchestrator, spaceCfg *spaceConfig) {
	for _, name := range spaceCfg.registry.List() {
		def := spaceCfg.toolDefs[name]
		if def == nil || def.Endpoint == "" {
			continue
		}
		port, err := portFromEndpoint(def.Endpoint)
		if err != nil {
			log.Printf("orchestrator: skipping %s: %v", name, err)
			continue
		}
		ref := def.Ref
		if ref == "" {
			ref = name
		}
		if _, err := orch.Start(name, ref, port); err != nil {
			log.Printf("orchestrator: skipping %s (may already be running): %v", name, err)
		}
	}
}

var _ tool.Invoker = (*tool.Registry)(nil)

// resolveModelFromEnv checks which API keys are available and returns the best
// provider/model pair. Resolution order: ANTHROPIC → OPENAI → OPENROUTER → ZHIPU.
func resolveModelFromEnv() (provider, modelName string) {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "anthropic", "claude-sonnet-4-20250514"
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return "openai", "gpt-4o"
	}
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		return "openrouter", "anthropic/claude-sonnet-4"
	}
	if os.Getenv("ZHIPU_API_KEY") != "" {
		return "zhipu", "GLM-4"
	}
	return "", ""
}

// reload reloads the Quarkfile and applies any changes in-place.
// On validation failure the current config is preserved and an error is returned.
func (r *Runtime) reload(ctx context.Context) error {
	newQF, err := quarkfile.Load(r.cfg.Dir)
	if err != nil {
		log.Printf("hot-reload: invalid Quarkfile — keeping current config: %v", err)
		return err
	}
	if err := quarkfile.Validate(r.cfg.Dir, newQF); err != nil {
		log.Printf("hot-reload: validation failed — keeping current config: %v", err)
		return err
	}

	diff := DiffQuarkfiles(r.currentQF, newQF)
	if diff.IsEmpty() {
		return nil
	}

	if diff.ModelChanged || diff.RoutingChanged {
		if err := r.reloadModel(newQF); err != nil {
			log.Printf("hot-reload: model reload failed: %v", err)
		}
	}

	if len(diff.ToolsAdded) > 0 || len(diff.ToolsRemoved) > 0 {
		r.reloadTools(newQF, diff)
	}

	if diff.PermissionsChanged {
		r.reloadPermissions(newQF)
	}

	if diff.PromptsChanged {
		r.reloadPrompts(newQF)
	}

	r.currentQF = newQF
	r.bus.Emit(eventbus.Event{
		Kind: eventbus.KindConfigChanged,
		Data: map[string]any{
			"model":        diff.ModelChanged,
			"tools_added":  diff.ToolsAdded,
			"tools_removed": diff.ToolsRemoved,
			"permissions":  diff.PermissionsChanged,
			"prompts":      diff.PromptsChanged,
		},
	})
	log.Printf("hot-reload: config applied (model=%v tools_added=%v tools_removed=%v permissions=%v prompts=%v)",
		diff.ModelChanged, diff.ToolsAdded, diff.ToolsRemoved, diff.PermissionsChanged, diff.PromptsChanged)
	return nil
}

// reloadModel rebuilds and hot-swaps the gateway when model or routing config changes.
func (r *Runtime) reloadModel(newQF *quarkfile.Quarkfile) error {
	sc := &spaceConfig{
		provider:  newQF.Model.Provider,
		modelName: newQF.Model.Name,
	}
	if len(newQF.Routing.Rules) > 0 || len(newQF.Routing.Fallback) > 0 {
		routing := newQF.Routing
		sc.routing = &routing
	}
	newGW, err := buildGateway(sc, r.cfgStore)
	if err != nil {
		return fmt.Errorf("build gateway: %w", err)
	}
	r.res.SwapGateway(newGW)
	log.Printf("hot-reload: gateway swapped to %s/%s", newQF.Model.Provider, newQF.Model.Name)
	return nil
}

// reloadTools adds and removes tools from the dispatcher without restarting.
func (r *Runtime) reloadTools(newQF *quarkfile.Quarkfile, diff ConfigDiff) {
	reg, ok := r.res.GetDispatcher().(*tool.Registry)
	if !ok {
		log.Printf("hot-reload: dispatcher is not a *tool.Registry — skipping tool reload")
		return
	}

	// Build new tool map from updated Quarkfile.
	newToolMap := make(map[string]*tool.Definition, len(newQF.Tools))
	for _, entry := range newQF.Tools {
		def := &tool.Definition{
			Ref:    entry.Ref,
			Name:   entry.Name,
			Config: make(map[string]string),
		}
		for k, v := range entry.Config {
			def.Config[k] = v
		}
		if ep := def.Config["endpoint"]; ep != "" {
			def.Endpoint = ep
		}
		newToolMap[entry.Name] = def
	}

	for _, name := range diff.ToolsAdded {
		def, ok := newToolMap[name]
		if !ok || def.Endpoint == "" {
			log.Printf("hot-reload: skipping add of tool %s (no endpoint)", name)
			continue
		}
		reg.Register(name, def)
		log.Printf("hot-reload: registered tool %s endpoint=%s", name, def.Endpoint)
		if r.orchestrator != nil {
			port, err := portFromEndpoint(def.Endpoint)
			if err == nil {
				ref := def.Ref
				if ref == "" {
					ref = name
				}
				if _, err := r.orchestrator.Start(name, ref, port); err != nil {
					log.Printf("hot-reload: could not start tool process %s: %v", name, err)
				}
			}
		}
	}

	for _, name := range diff.ToolsRemoved {
		reg.Deregister(name)
		log.Printf("hot-reload: deregistered tool %s", name)
		if r.orchestrator != nil {
			r.orchestrator.Stop(name)
		}
	}
}

// reloadPermissions hot-swaps permission constraints from the updated Quarkfile.
func (r *Runtime) reloadPermissions(newQF *quarkfile.Quarkfile) {
	p := agentcore.Permissions{
		Filesystem: agentcore.FilesystemPermissions{
			AllowedPaths: newQF.Permissions.Filesystem.AllowedPaths,
			ReadOnly:     newQF.Permissions.Filesystem.ReadOnly,
		},
		Network: agentcore.NetworkPermissions{
			AllowedHosts: newQF.Permissions.Network.AllowedHosts,
			Deny:         newQF.Permissions.Network.Deny,
		},
		Tools: agentcore.ToolPermissions{
			Allowed: newQF.Permissions.Tools.Allowed,
			Denied:  newQF.Permissions.Tools.Denied,
		},
		Plugins: agentcore.PluginPermissions{
			Allowed:     newQF.Permissions.Plugins.Allowed,
			AutoInstall: newQF.Permissions.Plugins.AutoInstall,
		},
		Audit: agentcore.AuditPermissions{
			LogToolCalls:    newQF.Permissions.Audit.LogToolCalls,
			LogLLMResponses: newQF.Permissions.Audit.LogLLMResponses,
			RetentionDays:   newQF.Permissions.Audit.RetentionDays,
		},
	}
	r.res.SwapPermissions(p)
	log.Printf("hot-reload: permissions updated")
}

// reloadPrompts updates the supervisor system prompt in the KB so the next
// chat call picks up the new content (updateSystemPrompt rebuilds from KB).
func (r *Runtime) reloadPrompts(newQF *quarkfile.Quarkfile) {
	if newQF.Supervisor.Prompt == "" {
		return
	}
	text, err := loadPromptText(r.cfg.Dir, newQF.Supervisor.Prompt)
	if err != nil {
		log.Printf("hot-reload: failed to read supervisor prompt: %v", err)
		return
	}
	if err := r.kb.Set(agentcore.NSConfig, "supervisor-prompt", []byte(text)); err != nil {
		log.Printf("hot-reload: failed to update supervisor prompt in KB: %v", err)
		return
	}
	log.Printf("hot-reload: supervisor prompt updated")
}

// autoConfigure writes auto-detected operational settings to the config store.
// These are agent self-configuration values that the owner can override.
func autoConfigure(store *config.Store, sc *spaceConfig) error {
	if sc.provider != "" {
		if err := store.SetByAgent("model_provider", sc.provider); err != nil {
			return fmt.Errorf("set model_provider: %w", err)
		}
	}
	if sc.modelName != "" {
		if err := store.SetByAgent("model_name", sc.modelName); err != nil {
			return fmt.Errorf("set model_name: %w", err)
		}
	}
	return nil
}
