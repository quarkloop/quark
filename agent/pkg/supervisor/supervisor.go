// Package supervisor implements the supervisor agent process.
//
// A supervisor is a long-lived process that:
//   - Loads a Quarkfile and lock file from the space directory.
//   - Opens the space knowledge base.
//   - Resolves agent and model configuration.
//   - Runs the Executor (ORIENT → PLAN → DISPATCH → MONITOR → ASSESS loop).
//   - Listens on an HTTP port for health, stats, logs, and chat.
//   - Listens on a Unix socket for worker IPC connections.
//   - Reports health to the api-server.
//   - Spawns worker processes via exec for each plan step.
package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/infra/httpserver"
	"github.com/quarkloop/agent/pkg/ipc"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/core/pkg/kb"
)

// Config holds startup parameters for a supervisor process.
type Config struct {
	SpaceID   string
	Dir       string
	Port      int
	APIServer string
	IPCSocket string
}

// Supervisor is the running supervisor agent for one space.
type Supervisor struct {
	cfg      *Config
	kb       kb.Store
	executor *agent.Executor
	ipcSrv   *ipc.Server
	httpSrv  *httpserver.Server
}

// New wires all dependencies and returns a ready-to-run Supervisor.
func New(cfg *Config) (*Supervisor, error) {
	absDir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolving dir: %w", err)
	}
	cfg.Dir = absDir

	// Derive IPC socket path if not set.
	if cfg.IPCSocket == "" {
		home, _ := os.UserHomeDir()
		cfg.IPCSocket = fmt.Sprintf("%s/.quark/agents/%s/ipc.sock", home, cfg.SpaceID)
	}

	// Open knowledge base.
	k, err := kb.Open(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("opening kb: %w", err)
	}

	// Build model gateway from environment.
	gw, err := buildGateway(cfg.Dir)
	if err != nil {
		k.Close()
		return nil, fmt.Errorf("building gateway: %w", err)
	}

	// Build skill dispatcher (empty — workers use tool CLIs directly).
	disp := skill.NewHTTPDispatcher()

	// Resolve supervisor agent definition.
	supervisorDef := defaultSupervisorDef()

	// Create executor.
	exec := agent.NewExecutor(k, gw, disp, agent.NewSupervisorEngine(), supervisorDef, map[string]*agent.Definition{})
	if err := exec.InitContext(supervisorDef.Config.ContextWindow); err != nil {
		k.Close()
		return nil, fmt.Errorf("initialising context: %w", err)
	}

	// Set up IPC server for worker connections.
	ipcSrv, err := ipc.NewServer(cfg.IPCSocket, handleWorkerConn(exec))
	if err != nil {
		k.Close()
		return nil, fmt.Errorf("ipc server: %w", err)
	}

	// Set up HTTP API server.
	mux := http.NewServeMux()
	registerRoutes(mux, cfg.SpaceID, k, exec)
	srv := httpserver.New("127.0.0.1", cfg.Port, mux)

	return &Supervisor{
		cfg:      cfg,
		kb:       k,
		executor: exec,
		ipcSrv:   ipcSrv,
		httpSrv:  srv,
	}, nil
}

// Run starts the HTTP server, IPC server, reports health, and runs the agent loop.
// Blocks until ctx is cancelled.
func (s *Supervisor) Run(ctx context.Context) error {
	// Start HTTP server.
	go func() {
		if err := s.httpSrv.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("supervisor %s: http server: %v", s.cfg.SpaceID, err)
		}
	}()

	// Start IPC server.
	go s.ipcSrv.Serve()

	// Wait for HTTP to be ready then report health.
	if err := waitHTTP(fmt.Sprintf("http://127.0.0.1:%d/health", s.cfg.Port), 10*time.Second); err != nil {
		log.Printf("supervisor %s: http not ready: %v", s.cfg.SpaceID, err)
	}
	if err := s.reportHealth(); err != nil {
		log.Printf("supervisor %s: health report failed: %v", s.cfg.SpaceID, err)
	}
	go s.heartbeat(ctx)

	log.Printf("supervisor %s ready on port %d", s.cfg.SpaceID, s.cfg.Port)

	// Run the agent execution loop.
	return s.executor.Run(ctx)
}

// Close shuts down the HTTP server, IPC server, and KB.
func (s *Supervisor) Close() {
	s.ipcSrv.Close()
	s.httpSrv.Shutdown(context.Background())
	s.kb.Close()
}

// handleWorkerConn returns an IPC connection handler that processes worker results.
func handleWorkerConn(exec *agent.Executor) func(net.Conn) {
	return func(conn net.Conn) {
		defer conn.Close()
		res, err := ipc.ReadResult(conn, func(ev *ipc.EventMessage) {
			log.Printf("worker event [%s]: %s", ev.StepID, ev.Message)
		})
		if err != nil {
			log.Printf("ipc: read result: %v", err)
			return
		}
		log.Printf("worker result [%s]: status=%s", res.StepID, res.Status)
		// Result is written to KB by the worker directly; supervisor
		// reads it in the MONITOR phase of the next cycle.
	}
}

func (s *Supervisor) reportHealth() error {
	type healthReport struct {
		SpaceID string `json:"space_id"`
		PID     int    `json:"pid"`
		Port    int    `json:"port"`
	}
	body, _ := json.Marshal(healthReport{
		SpaceID: s.cfg.SpaceID,
		PID:     os.Getpid(),
		Port:    s.cfg.Port,
	})
	url := fmt.Sprintf("%s/api/v1/spaces/%s/health", s.cfg.APIServer, s.cfg.SpaceID)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (s *Supervisor) heartbeat(ctx context.Context) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := s.reportHealth(); err != nil {
				log.Printf("supervisor %s: heartbeat failed: %v", s.cfg.SpaceID, err)
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

func buildGateway(dir string) (model.Gateway, error) {
	if os.Getenv("QUARK_DRY_RUN") == "1" {
		log.Printf("QUARK_DRY_RUN=1 — using noop model gateway")
		return model.New(model.GatewayConfig{Provider: "noop", Model: "noop"})
	}

	// Read provider from Quarkfile via environment (set by api-server at launch).
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
	}

	return model.New(model.GatewayConfig{
		Provider: provider,
		Model:    modelName,
		APIKey:   apiKey,
	})
}

func defaultSupervisorDef() *agent.Definition {
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

// registerRoutes mounts the supervisor HTTP endpoints onto mux.
func registerRoutes(mux *http.ServeMux, spaceID string, k kb.Store, exec *agent.Executor) {
	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"space_id": spaceID, "status": "running"})
	})

	// Stats
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{"space_id": spaceID, "agent_count": 1}
		if cs := exec.ContextStats(); cs != nil {
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
		resp, err := exec.Chat(r.Context(), req)
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

// IPC socket path helper — mirrors the api-server's ipcSocketPath.
func IPCSocketPath(spaceID string) string {
	home, _ := os.UserHomeDir()
	return fmt.Sprintf("%s/.quark/agents/%s/ipc.sock", home, spaceID)
}
