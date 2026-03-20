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
	"github.com/quarkloop/agent/pkg/infra/httpserver"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/core/pkg/kb"
)

type Config struct {
	AgentID   string
	Dir       string
	Port      int
	APIServer string
}

type Runtime struct {
	cfg     *Config
	kb      kb.Store
	agent   *agent.Agent
	feed    *activity.Feed
	httpSrv *httpserver.Server
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

	gw, err := buildGateway(spaceCfg)
	if err != nil {
		k.Close()
		return nil, fmt.Errorf("build gateway: %w", err)
	}

	feed := activity.NewFeed(1024, k)
	a := agent.NewAgent(
		spaceCfg.supervisor,
		k,
		gw,
		spaceCfg.dispatcher,
		agent.WithSubAgents(spaceCfg.subAgents),
		agent.WithActivitySink(feed),
	)
	if err := a.InitContext(spaceCfg.supervisor.Config.ContextWindow); err != nil {
		k.Close()
		return nil, fmt.Errorf("initialise context: %w", err)
	}

	mux := http.NewServeMux()
	agentapi.NewHandler(
		newAgentService(cfg.AgentID, k, a, feed),
		agentapi.WithBasePath(agentapi.DefaultBasePath),
		agentapi.WithAliasBasePath(""),
	).RegisterRoutes(mux)
	srv := httpserver.New("127.0.0.1", cfg.Port, mux)

	return &Runtime{
		cfg:     cfg,
		kb:      k,
		agent:   a,
		feed:    feed,
		httpSrv: srv,
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
	body, _ := json.Marshal(healthReport{
		AgentID: r.cfg.AgentID,
		PID:     os.Getpid(),
		Port:    r.cfg.Port,
	})
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

var _ skill.Invoker = (*skill.HTTPDispatcher)(nil)
