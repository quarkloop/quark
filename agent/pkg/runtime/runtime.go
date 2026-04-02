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
	"github.com/quarkloop/agent/pkg/config"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/infra/httpserver"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/session"
	"github.com/quarkloop/agent/pkg/tool"
	"github.com/quarkloop/core/pkg/kb"
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
	agent        *agent.Agent
	bus          *eventbus.Bus
	actWriter    *activity.Writer
	httpSrv      *httpserver.Server
	orchestrator *ToolOrchestrator
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

	gw, err := buildGateway(spaceCfg)
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
	sessStore := session.NewStore(k)
	cfgStore := config.New(k)

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

	res := &agentcore.Resources{
		KB:          k,
		ConfigStore: cfgStore,
		EventBus:    bus,
		Activity:    actWriter,
		Gateway:     gw,
		Dispatcher:  spaceCfg.dispatcher,
	}

	a := agent.New(cfg.AgentID, spaceCfg.supervisor, res, sessStore, spaceCfg.subAgents)
	if err := a.Init(); err != nil {
		if orchestrator != nil {
			orchestrator.Shutdown()
		}
		k.Close()
		return nil, fmt.Errorf("initialise agent: %w", err)
	}

	mux := http.NewServeMux()
	agentapi.NewHandler(
		newAgentService(cfg.AgentID, cfg.Dir, k, a, bus, actWriter, sessStore),
		agentapi.WithBasePath(agentapi.DefaultBasePath),
		agentapi.WithAliasBasePath(""),
	).RegisterRoutes(mux)
	srv := httpserver.New("127.0.0.1", cfg.Port, mux)

	return &Runtime{
		cfg:          cfg,
		kb:           k,
		agent:        a,
		bus:          bus,
		actWriter:    actWriter,
		httpSrv:      srv,
		orchestrator: orchestrator,
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
	return r.agent.Run(ctx)
}

func (r *Runtime) Close() {
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

func buildGateway(spaceCfg *spaceConfig) (model.Gateway, error) {
	if os.Getenv("QUARK_DRY_RUN") == "1" {
		log.Printf("runtime: QUARK_DRY_RUN=1, using noop model gateway")
		return model.New(model.GatewayConfig{Provider: "noop", Model: "noop"})
	}

	provider := stringsFirstNonEmpty(os.Getenv("QUARK_MODEL_PROVIDER"), spaceCfg.provider)
	modelName := stringsFirstNonEmpty(os.Getenv("QUARK_MODEL_NAME"), spaceCfg.modelName)
	if provider == "" {
		provider = "noop"
		modelName = "noop"
	}

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
	for _, name := range spaceCfg.dispatcher.List() {
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

var _ tool.Invoker = (*tool.HTTPDispatcher)(nil)

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
