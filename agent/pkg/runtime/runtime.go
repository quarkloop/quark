// Package runtime implements the agent runtime process.
//
// A runtime is a long-lived process that:
//   - Opens the space knowledge base.
//   - Loads the Quarkfile and installed plugins.
//   - Resolves agent, tool, and model configuration.
//   - Runs the Agent loop.
//   - Serves the standardized agent HTTP API.
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	"github.com/quarkloop/agent/pkg/activity"
	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/agentcore"
	"github.com/quarkloop/agent/pkg/channel"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/hooks"
	"github.com/quarkloop/agent/pkg/infra/httpserver"
	"github.com/quarkloop/agent/pkg/infra/watcher"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/resolver"
	"github.com/quarkloop/agent/pkg/session"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/agent/pkg/tool"
	"github.com/quarkloop/cli/pkg/kb"
	cliplugin "github.com/quarkloop/cli/pkg/plugin"
	"github.com/quarkloop/cli/pkg/quarkfile"
)

type Config struct {
	AgentID   string
	Dir       string
	Port      int
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
	res          *agentcore.Resources
	currentQF    *quarkfile.Quarkfile
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

	qf, err := quarkfile.Load(cfg.Dir)
	if err != nil {
		k.Close()
		return nil, fmt.Errorf("load Quarkfile: %w", err)
	}
	if err := quarkfile.Validate(cfg.Dir, qf); err != nil {
		k.Close()
		return nil, fmt.Errorf("validate Quarkfile: %w", err)
	}

	// Discover installed plugins from .quark/plugins/.
	pluginsDir := filepath.Join(cfg.Dir, ".quark", "plugins")
	plugins, err := cliplugin.DiscoverInstalled(pluginsDir)
	if err != nil {
		k.Close()
		return nil, fmt.Errorf("discover plugins: %w", err)
	}

	spaceCfg, err := loadSpaceConfig(cfg.Dir, plugins, qf)
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

	gw, err := buildGateway(spaceCfg, nil)
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

	// Auto-configure model if not set via env vars or Quarkfile.
	provider := spaceCfg.provider
	modelName := spaceCfg.modelName
	if provider == "" || modelName == "" {
		envProvider, envModel := resolveModelFromEnv()
		if envProvider != "" && envModel != "" {
			provider = envProvider
			modelName = envModel
		}
	}

	autoConfigureSimple(gw, provider, modelName)

	// Build skill resolver with loaded skill plugin paths.
	var skillRoots []skill.Root
	for _, p := range plugins {
		if p.Manifest.Type == cliplugin.TypeSkill {
			skillRoots = append(skillRoots, skill.Root{Dir: p.Dir, Source: skill.SourcePlugin, Priority: 2})
		}
	}
	skillResolver := skill.NewResolver(skillRoots...)

	// Create channel manager and register the Web UI channel.
	chManager := channel.NewManager(bus)
	webCh := channel.NewWebChannel(nil)
	if err := chManager.Register(webCh); err != nil {
		log.Printf("runtime: failed to register web channel: %v", err)
	}

	res := &agentcore.Resources{
		KB:            k,
		EventBus:      bus,
		Activity:      actWriter,
		Hooks:         hookReg,
		SkillResolver: skillResolver,
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

	// Build supervisor definition from agent plugins.
	supervisorDef := buildSupervisorDefFromPlugins(spaceCfg.agentDefs, qf)
	if err := loadSupervisorPrompt(cfg.Dir, qf, supervisorDef); err != nil {
		log.Printf("runtime: supervisor prompt warning: %v", err)
	}

	// Build worker agent definitions from agent plugins.
	workerAgents := make(map[string]*agentcore.Definition)
	for _, man := range spaceCfg.agentDefs {
		if man.Name != "supervisor" {
			def := &agentcore.Definition{
				Name:         man.Name,
				SystemPrompt: man.Prompt,
			}
			workerAgents[man.Name] = def
		}
	}

	// Create agent registry and register the supervisor agent.
	registry := agent.NewRegistry()
	sessStore := session.NewStore(k)
	a := agent.New(cfg.AgentID, supervisorDef, res, sessStore, workerAgents)
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

	svc := newAgentService(cfg.Dir, k, registry, chain, bus, actWriter, sessStore)

	mux := http.NewServeMux()
	agentapi.NewHandler(
		svc,
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
		currentQF:    qf,
	}, nil
}

// loadSupervisorPrompt loads the supervisor prompt from the first agent plugin's prompt file.
func loadSupervisorPrompt(dir string, qf *quarkfile.Quarkfile, def *agentcore.Definition) error {
	if def == nil || def.SystemPrompt == "" {
		return nil
	}
	// The system prompt is already set by buildSupervisorDefFromPlugins.
	_ = dir
	_ = qf
	return nil
}

func autoConfigureSimple(gw model.Gateway, provider, modelName string) {
	if provider != "" && modelName != "" {
		log.Printf("runtime: model %s/%s", provider, modelName)
	}
}

func NewWithService(cfg *Config, svc agentapi.Service) (*Runtime, error) {
	absDir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve dir: %w", err)
	}
	cfg.Dir = absDir

	k, err := kb.Open(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("open kb: %w", err)
	}

	qf, err := quarkfile.Load(cfg.Dir)
	if err != nil {
		k.Close()
		return nil, fmt.Errorf("load Quarkfile: %w", err)
	}

	bus := eventbus.New()
	actWriter := activity.NewWriter(bus, k, 1024)
	actWriter.Start()

	registry := agent.NewRegistry()
	chain := resolver.NewChainResolver(
		&resolver.SessionAffinityResolver{},
		resolver.NewFallbackResolver(cfg.AgentID),
	)

	mux := http.NewServeMux()
	agentapi.NewHandler(
		svc,
		agentapi.WithBasePath(agentapi.DefaultBasePath),
		agentapi.WithAliasBasePath(""),
	).RegisterRoutes(mux)
	srv := httpserver.New("127.0.0.1", cfg.Port, mux)

	return &Runtime{
		cfg:       cfg,
		kb:        k,
		registry:  registry,
		resolver:  chain,
		bus:       bus,
		actWriter: actWriter,
		httpSrv:   srv,
		currentQF: qf,
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
		return fmt.Errorf("runtime %s: health check timed out: %w", r.cfg.AgentID, err)
	}

	log.Printf("runtime %s ready on port %d", r.cfg.AgentID, r.cfg.Port)

	// Start all registered channels.
	if r.chManager != nil {
		if err := r.chManager.StartAll(ctx); err != nil {
			log.Printf("runtime %s: channel start error: %v", r.cfg.AgentID, err)
		}
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

func buildGateway(spaceCfg *spaceConfig, cfgStore any) (model.Gateway, error) {
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

	// Wrap with routing gateway if routing rules are configured.
	rules := buildRoutingRulesFromQuarkfile(spaceCfg)
	if len(rules) > 0 {
		return model.NewRoutingGateway(rules, primaryGw), nil
	}

	return primaryGw, nil
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

func buildRoutingRulesFromQuarkfile(spaceCfg *spaceConfig) []model.RoutingRule {
	if spaceCfg == nil || spaceCfg.routing == nil {
		return nil
	}
	var rules []model.RoutingRule
	for _, r := range spaceCfg.routing.Rules {
		gw, err := buildSingleGateway(r.Provider, r.Model)
		if err != nil {
			log.Printf("runtime: skipping routing rule %q→%s/%s: %v", r.Match, r.Provider, r.Model, err)
			continue
		}
		rules = append(rules, model.RoutingRule{
			Match:   r.Match,
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

// startToolProcesses starts tool processes for each tool in the space config,
// assigning unique ports starting from the default base port.
func startToolProcesses(orch *ToolOrchestrator, spaceCfg *spaceConfig) {
	nextPort := 8091
	for _, name := range spaceCfg.registry.List() {
		def := spaceCfg.toolDefs[name]
		if def == nil || def.Endpoint == "" {
			// Assign a unique port for each tool without a configured endpoint.
			port := nextPort
			nextPort++
			ref := def.Ref
			if ref == "" {
				ref = name
			}
			if _, err := orch.Start(name, ref, port); err != nil {
				log.Printf("orchestrator: skipping %s (may already be running): %v", name, err)
			}
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

// resolveModelFromEnv checks which API keys are available and returns the
// best provider/model pair. Resolution order: ANTHROPIC → OPENAI → OPENROUTER → ZHIPU.
// The model name is read from the corresponding <PROVIDER>_MODEL env var or left
// empty for the gateway to use its default.
func resolveModelFromEnv() (provider, modelName string) {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "anthropic", os.Getenv("ANTHROPIC_MODEL")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return "openai", os.Getenv("OPENAI_MODEL")
	}
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		return "openrouter", os.Getenv("OPENROUTER_MODEL")
	}
	if os.Getenv("ZHIPU_API_KEY") != "" {
		return "zhipu", os.Getenv("ZHIPU_MODEL")
	}
	return "", ""
}

// reload reloads the Quarkfile and applies any changes in-place.
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

	if len(diff.PluginsAdded) > 0 || len(diff.PluginsRemoved) > 0 {
		r.reloadPlugins(diff)
	}

	if diff.PermissionsChanged {
		r.reloadPermissions(newQF)
	}

	r.currentQF = newQF
	r.bus.Emit(eventbus.Event{
		Kind: eventbus.KindConfigChanged,
		Data: map[string]any{
			"model":           diff.ModelChanged,
			"plugins_added":   diff.PluginsAdded,
			"plugins_removed": diff.PluginsRemoved,
			"permissions":     diff.PermissionsChanged,
		},
	})
	log.Printf("hot-reload: config applied (model=%v plugins_added=%v plugins_removed=%v permissions=%v)",
		diff.ModelChanged, diff.PluginsAdded, diff.PluginsRemoved, diff.PermissionsChanged)
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
	newGW, err := buildGateway(sc, nil)
	if err != nil {
		return fmt.Errorf("build gateway: %w", err)
	}
	r.res.SwapGateway(newGW)
	log.Printf("hot-reload: gateway swapped to %s/%s", newQF.Model.Provider, newQF.Model.Name)
	return nil
}

// reloadPlugins discovers new tool plugins and deregisters removed ones.
func (r *Runtime) reloadPlugins(diff ConfigDiff) {
	_ = diff
	pluginsDir := filepath.Join(r.cfg.Dir, ".quark", "plugins")
	plugins, err := cliplugin.DiscoverInstalled(pluginsDir)
	if err != nil {
		log.Printf("hot-reload: could not re-discover plugins: %v", err)
		return
	}

	reg, ok := r.res.GetDispatcher().(*tool.Registry)
	if !ok {
		return
	}

	// Build set of discovered tool names.
	newTools := make(map[string]bool)
	for _, p := range plugins {
		if p.Manifest.Type == cliplugin.TypeTool {
			newTools[p.Manifest.Name] = true
			if !toolKnownInRegistry(reg, p.Manifest.Name) {
				def := &tool.Definition{
					Ref:    p.Manifest.Repository,
					Name:   p.Manifest.Name,
					Config: make(map[string]string),
				}
				reg.Register(def.Name, def)
				log.Printf("hot-reload: registered tool plugin %s", def.Name)
			}
		}
	}

	// Deregister tools that no longer exist on disk.
	for _, name := range reg.ListAll() {
		if !newTools[name] {
			reg.Deregister(name)
			log.Printf("hot-reload: deregistered tool %s", name)
		}
	}

	// Shut down orchestrator processes for removed tools.
	if r.orchestrator != nil {
		for _, name := range reg.ListAll() {
			if !newTools[name] {
				r.orchestrator.Stop(name)
			}
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
		Audit: agentcore.AuditPermissions{
			LogToolCalls:    newQF.Permissions.Audit.LogToolCalls,
			LogLLMResponses: newQF.Permissions.Audit.LogLLMResponses,
			RetentionDays:   newQF.Permissions.Audit.RetentionDays,
		},
	}
	r.res.SwapPermissions(p)
	log.Printf("hot-reload: permissions updated")
}

// toolKnownInRegistry checks if a tool is already registered (no IsRegistered method on Registry).
func toolKnownInRegistry(reg *tool.Registry, name string) bool {
	return slices.Contains(reg.List(), name)
}

// ---------- agentService ----------

type agentService struct {
	dir          string
	kb           kb.Store
	registry     *agent.Registry
	resolver     resolver.Resolver
	bus          *eventbus.Bus
	actWriter    *activity.Writer
	sessionStore *session.Store
}

func newAgentService(dir string, k kb.Store, registry *agent.Registry, res resolver.Resolver, bus *eventbus.Bus, actWriter *activity.Writer, sessStore *session.Store) *agentService {
	return &agentService{dir: dir, kb: k, registry: registry, resolver: res, bus: bus, actWriter: actWriter, sessionStore: sessStore}
}

func (s *agentService) resolveAgent(ctx context.Context, sessionKey string) (*agent.Agent, error) {
	msg := resolver.InboundMessage{
		SessionKey: sessionKey,
		Channel:    "web",
	}
	agentID, err := s.resolver.Resolve(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("resolve agent: %w", err)
	}
	a, ok := s.registry.Get(agentID)
	if !ok {
		return nil, fmt.Errorf("agent %q not found", agentID)
	}
	return a, nil
}

func (s *agentService) Health(ctx context.Context, r *http.Request) (*agentapi.HealthResponse, error) {
	return &agentapi.HealthResponse{AgentID: "supervisor", Status: "running"}, nil
}

func (s *agentService) Info(ctx context.Context, r *http.Request) (*agentapi.InfoResponse, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	return &agentapi.InfoResponse{
		AgentID:  "supervisor",
		Provider: a.Provider(),
		Model:    a.ModelName(),
		Mode:     string(a.Mode()),
		Tools:    a.Tools(),
	}, nil
}

func (s *agentService) Mode(ctx context.Context, r *http.Request) (*agentapi.ModeResponse, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	return &agentapi.ModeResponse{Mode: string(a.Mode())}, nil
}

func (s *agentService) SetMode(ctx context.Context, r *http.Request, mode string) (*agentapi.ModeResponse, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	a.SetMode(agentcore.Mode(mode))
	return &agentapi.ModeResponse{Mode: mode}, nil
}

func (s *agentService) Stats(ctx context.Context, r *http.Request) (agentapi.StatsResponse, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	resp := agentapi.StatsResponse{
		"agent_id":    "supervisor",
		"agent_count": len(s.registry.List()),
		"mode":        string(a.Mode()),
	}
	if cs := a.ContextStats(); cs != nil {
		var contextStats map[string]any
		raw, _ := json.Marshal(cs)
		if json.Unmarshal(raw, &contextStats) == nil {
			resp["context"] = contextStats
		}
	}
	return resp, nil
}

func (s *agentService) Chat(ctx context.Context, r *http.Request, req agentapi.ChatRequest) (*agentapi.ChatResponse, error) {
	a, err := s.resolveAgent(ctx, req.SessionKey)
	if err != nil {
		return nil, err
	}

	agentReq := agentcore.ChatRequest{
		Message:    req.Message,
		SessionKey: req.SessionKey,
		Stream:     req.Stream,
		Mode:       req.Mode,
	}

	if len(req.Files) > 0 {
		for i, f := range req.Files {
			saved, err := s.saveUploadedFile(f)
			if err != nil {
				return nil, agentapi.Error(http.StatusBadRequest,
					fmt.Sprintf("save file %s: %v", f.Name, err), err)
			}
			agentReq.Files = append(agentReq.Files, agentcore.FileAttachment{
				Name:     saved.Name,
				MimeType: saved.MimeType,
				Size:     saved.Size,
				Path:     saved.Path,
			})
			req.Files[i].Path = saved.Path
		}
	}

	resp, err := a.Chat(ctx, req.SessionKey, agentReq)
	if err != nil {
		return nil, err
	}
	return &agentapi.ChatResponse{
		Reply:        resp.Reply,
		Mode:         resp.Mode,
		Warning:      resp.Warning,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}, nil
}

func (s *agentService) saveUploadedFile(f agentapi.FileAttachment) (agentapi.FileAttachment, error) {
	if len(f.Content) == 0 {
		return f, nil
	}
	uploadsDir := filepath.Join(s.dir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return f, fmt.Errorf("create uploads dir: %w", err)
	}
	dest := filepath.Join(uploadsDir, f.Name)
	if err := os.WriteFile(dest, f.Content, 0o644); err != nil {
		return f, fmt.Errorf("write file: %w", err)
	}
	f.Path = dest
	return f, nil
}

func (s *agentService) Stop(ctx context.Context, r *http.Request) error {
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

func (s *agentService) Plan(ctx context.Context, r *http.Request) (*agentapi.Plan, error) {
	currentPlan, err := plan.NewStore(s.kb, agentcore.NSPlans, agentcore.KeyMasterPlan).Load()
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "plan not found", err)
	}
	return convertPlan(currentPlan)
}

func (s *agentService) Activity(ctx context.Context, r *http.Request, limit int) ([]agentapi.ActivityRecord, error) {
	events := s.actWriter.Recent(limit)
	out := make([]agentapi.ActivityRecord, 0, len(events))
	for _, event := range events {
		out = append(out, convertActivity(event))
	}
	return out, nil
}

func (s *agentService) StreamActivity(ctx context.Context, r *http.Request, emit func(agentapi.ActivityRecord) error) error {
	for _, event := range s.actWriter.Recent(64) {
		if err := emit(convertActivity(event)); err != nil {
			return err
		}
	}

	ch := s.bus.Subscribe(256)
	defer s.bus.Unsubscribe(ch)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if err := emit(convertActivity(event)); err != nil {
				return err
			}
		}
	}
}

func (s *agentService) Sessions(ctx context.Context, r *http.Request) ([]agentapi.SessionRecord, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	sessions, err := a.ListSessions()
	if err != nil {
		return nil, err
	}
	out := make([]agentapi.SessionRecord, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, convertSession(sess))
	}
	return out, nil
}

func (s *agentService) Session(ctx context.Context, r *http.Request, sessionKey string) (*agentapi.SessionRecord, error) {
	a, err := s.resolveAgent(ctx, sessionKey)
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "session not found", err)
	}
	sess, err := a.GetSession(sessionKey)
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "session not found", err)
	}
	return ptr(convertSession(sess)), nil
}

func (s *agentService) SessionActivity(ctx context.Context, r *http.Request, sessionKey string, limit int) ([]agentapi.ActivityRecord, error) {
	events, err := s.actWriter.History(sessionKey)
	if err != nil {
		return nil, err
	}
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	out := make([]agentapi.ActivityRecord, 0, len(events))
	for _, ev := range events {
		out = append(out, convertActivity(ev))
	}
	return out, nil
}

func (s *agentService) CreateSession(ctx context.Context, r *http.Request, req agentapi.CreateSessionRequest) (*agentapi.CreateSessionResponse, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	sess, err := a.CreateSession(session.Type(req.Type), req.Title)
	if err != nil {
		return nil, agentapi.Error(http.StatusBadRequest, err.Error(), err)
	}
	return &agentapi.CreateSessionResponse{
		Session: convertSession(sess),
	}, nil
}

func (s *agentService) DeleteSession(ctx context.Context, r *http.Request, sessionKey string) error {
	a, err := s.resolveAgent(ctx, sessionKey)
	if err != nil {
		return err
	}
	if err := a.DeleteSession(sessionKey); err != nil {
		return agentapi.Error(http.StatusBadRequest, err.Error(), err)
	}
	return nil
}

func (s *agentService) ApprovePlan(ctx context.Context, r *http.Request, planID string) (*agentapi.Plan, error) {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return nil, err
	}
	// planID is reserved for future multi-plan support; currently ignored (agent tracks one active plan).
	_ = planID
	p, err := a.ApprovePlan()
	if err != nil {
		return nil, agentapi.Error(http.StatusBadRequest, err.Error(), err)
	}
	return convertPlan(p)
}

func (s *agentService) RejectPlan(ctx context.Context, r *http.Request, planID string) error {
	a, err := s.resolveAgent(ctx, "")
	if err != nil {
		return err
	}
	// planID is reserved for future multi-plan support; currently ignored (agent tracks one active plan).
	_ = planID
	if err := a.RejectPlan(); err != nil {
		return agentapi.Error(http.StatusBadRequest, err.Error(), err)
	}
	return nil
}

func (s *agentService) SessionBudget(ctx context.Context, r *http.Request, sessionKey string) (*agentapi.BudgetResponse, error) {
	a, err := s.resolveAgent(ctx, sessionKey)
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "agent not found", err)
	}
	status, err := a.BudgetStatus(sessionKey)
	if err != nil {
		return nil, agentapi.Error(http.StatusNotFound, "session not found", err)
	}
	return &agentapi.BudgetResponse{
		TotalBudget:      status.TotalBudget,
		UsedTokens:       status.UsedTokens,
		AvailableTokens:  status.AvailableTokens,
		UsagePct:         status.UsagePct,
		AtSoftLimit:      status.CompactionNeeded,
		AtHardLimit:      status.AtHardLimit,
		CompactionNeeded: status.CompactionNeeded,
	}, nil
}

// ---------- helpers ----------

func convertPlan(currentPlan *plan.Plan) (*agentapi.Plan, error) {
	raw, err := json.Marshal(currentPlan)
	if err != nil {
		return nil, err
	}
	var converted agentapi.Plan
	if err := json.Unmarshal(raw, &converted); err != nil {
		return nil, err
	}
	return &converted, nil
}

func convertActivity(event activity.Event) agentapi.ActivityRecord {
	var raw json.RawMessage
	if event.Data != nil {
		if encoded, err := json.Marshal(event.Data); err == nil {
			raw = encoded
		}
	}
	return agentapi.ActivityRecord{
		ID:        event.ID,
		SessionID: event.SessionID,
		Type:      string(event.Kind),
		Timestamp: event.Timestamp,
		Data:      raw,
	}
}

func convertSession(sess *session.Session) agentapi.SessionRecord {
	return agentapi.SessionRecord{
		Key:       sess.Key,
		AgentID:   sess.AgentID,
		Type:      agentapi.SessionType(sess.Type),
		Status:    string(sess.Status),
		Title:     sess.Title,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
		EndedAt:   sess.EndedAt,
	}
}

func ptr[T any](v T) *T {
	return &v
}
