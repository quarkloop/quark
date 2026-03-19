// Package runtime implements the agent runtime process.
//
// A runtime is a long-lived process that:
//   - Opens the space knowledge base.
//   - Resolves agent and model configuration.
//   - Runs the Agent (ORIENT → PLAN → DISPATCH → MONITOR → ASSESS loop).
//   - Listens on an HTTP port for health, stats, and chat.
//   - Reports health to the api-server.
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

	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/infra/httpserver"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/core/pkg/kb"
)

// Config holds startup parameters for a runtime process.
type Config struct {
	SpaceID   string
	Dir       string
	Port      int
	APIServer string
}

// Runtime is the running agent runtime for one space.
type Runtime struct {
	cfg     *Config
	kb      kb.Store
	agent   *agent.Agent
	httpSrv *httpserver.Server
}

// New wires all dependencies and returns a ready-to-run Runtime.
func New(cfg *Config) (*Runtime, error) {
	absDir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolving dir: %w", err)
	}
	cfg.Dir = absDir

	// Open knowledge base.
	k, err := kb.Open(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("opening kb: %w", err)
	}

	// Build model gateway from environment.
	gw, err := buildGateway()
	if err != nil {
		k.Close()
		return nil, fmt.Errorf("building gateway: %w", err)
	}

	// Build skill dispatcher (empty — workers use tool CLIs directly).
	disp := skill.NewHTTPDispatcher()

	// Resolve agent definition.
	def := defaultAgentDef()

	// Create agent.
	a := agent.NewAgent(def, k, gw, disp)
	if err := a.InitContext(def.Config.ContextWindow); err != nil {
		k.Close()
		return nil, fmt.Errorf("initialising context: %w", err)
	}

	// Set up HTTP API server.
	mux := http.NewServeMux()
	registerRoutes(mux, cfg.SpaceID, k, a)
	srv := httpserver.New("127.0.0.1", cfg.Port, mux)

	return &Runtime{
		cfg:     cfg,
		kb:      k,
		agent:   a,
		httpSrv: srv,
	}, nil
}

// Run starts the HTTP server, reports health, and runs the agent loop.
// Blocks until ctx is cancelled.
func (r *Runtime) Run(ctx context.Context) error {
	// Start HTTP server.
	go func() {
		if err := r.httpSrv.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("runtime %s: http server: %v", r.cfg.SpaceID, err)
		}
	}()

	// Wait for HTTP to be ready then report health.
	if err := waitHTTP(fmt.Sprintf("http://127.0.0.1:%d/health", r.cfg.Port), 10*time.Second); err != nil {
		log.Printf("runtime %s: http not ready: %v", r.cfg.SpaceID, err)
	}
	if err := r.reportHealth(); err != nil {
		log.Printf("runtime %s: health report failed: %v", r.cfg.SpaceID, err)
	}
	go r.heartbeat(ctx)

	log.Printf("runtime %s ready on port %d", r.cfg.SpaceID, r.cfg.Port)

	// Run the agent execution loop.
	return r.agent.Run(ctx)
}

// Close shuts down the HTTP server and KB.
func (r *Runtime) Close() {
	r.httpSrv.Shutdown(context.Background())
	r.kb.Close()
}

func (r *Runtime) reportHealth() error {
	type healthReport struct {
		SpaceID string `json:"space_id"`
		PID     int    `json:"pid"`
		Port    int    `json:"port"`
	}
	body, _ := json.Marshal(healthReport{
		SpaceID: r.cfg.SpaceID,
		PID:     os.Getpid(),
		Port:    r.cfg.Port,
	})
	url := fmt.Sprintf("%s/api/v1/spaces/%s/health", r.cfg.APIServer, r.cfg.SpaceID)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (r *Runtime) heartbeat(ctx context.Context) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := r.reportHealth(); err != nil {
				log.Printf("runtime %s: heartbeat failed: %v", r.cfg.SpaceID, err)
			}
		}
	}
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", url)
}

func buildGateway() (model.Gateway, error) {
	if os.Getenv("QUARK_DRY_RUN") == "1" {
		log.Printf("QUARK_DRY_RUN=1 — using noop model gateway")
		return model.New(model.GatewayConfig{Provider: "noop", Model: "noop"})
	}

	provider := os.Getenv("QUARK_MODEL_PROVIDER")
	modelName := os.Getenv("QUARK_MODEL_NAME")
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

func defaultAgentDef() *agent.Definition {
	return &agent.Definition{
		Ref:     "quark/supervisor@latest",
		Name:    "supervisor",
		Version: "1.0.0",
		Config: agent.Config{
			ContextWindow: 8192,
			Compaction:    "sliding",
			MemoryPolicy:  "summarize",
		},
		Capabilities: agent.Capabilities{
			SpawnAgents: true,
			MaxWorkers:  10,
			CreatePlans: true,
		},
	}
}

// registerRoutes mounts the runtime HTTP endpoints onto mux.
func registerRoutes(mux *http.ServeMux, spaceID string, k kb.Store, a *agent.Agent) {
	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"space_id": spaceID, "status": "running"})
	})

	// Mode
	mux.HandleFunc("GET /mode", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"mode": string(a.Mode())})
	})

	// Stats
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{"space_id": spaceID, "agent_count": 1, "mode": string(a.Mode())}
		if cs := a.ContextStats(); cs != nil {
			if raw, err := json.Marshal(cs); err == nil {
				var m map[string]interface{}
				if json.Unmarshal(raw, &m) == nil {
					result["context"] = m
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// Chat
	mux.HandleFunc("POST /chat", func(w http.ResponseWriter, r *http.Request) {
		var req agent.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		resp, err := a.Chat(r.Context(), req)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Stop (graceful self-shutdown)
	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		go func() {
			time.Sleep(100 * time.Millisecond)
			os.Exit(0)
		}()
	})
}
