package space

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	stderrors "errors"

	agentapi "github.com/quarkloop/agent-api"
	"github.com/quarkloop/agent/pkg/errors"
	"github.com/quarkloop/agent/pkg/infra/httpserver"
	"github.com/quarkloop/tools/space/pkg/quarkfile"
)

// serverVersion is the api-server version reported in system info.
const serverVersion = "0.1.0-dev"

// Handler is the HTTP layer for the space subsystem.
//
// Route table:
//
//	POST   /api/v1/spaces                  Run      — create and launch
//	GET    /api/v1/spaces                  List     — all records
//	GET    /api/v1/spaces/{id}             Get      — single record
//	POST   /api/v1/spaces/{id}/stop        Stop     — SIGINT or SIGKILL
//	DELETE /api/v1/spaces/{id}             Delete   — stopped/failed only
//	GET    /api/v1/spaces/{id}/logs        Logs     — SSE: ring buffer + LastLogs
//	POST   /api/v1/spaces/prune            Prune    — bulk-delete stopped+failed
//	POST   /api/v1/agents/{id}/health      AgentHealth — heartbeat from agent runtime
//	GET    /api/v1/system/info             SystemInfo
//
// Handler implements the HTTP API for space lifecycle management.
// Routes are registered by RegisterRoutes on the provided mux:
//
//	POST/GET /api/v1/spaces, /api/v1/spaces/{id} — CRUD + stop + prune
//	GET      /api/v1/spaces/{id}/logs    — SSE log stream
//	POST     /api/v1/agents/{id}/health  — health report from agent runtime
//	GET      /api/v1/system/info         — version and space counts
type Handler struct {
	store      Store
	controller *Controller
}

// NewHandler creates a Handler wired to store s and controller c.
func NewHandler(s Store, c *Controller) *Handler {
	return &Handler{store: s, controller: c}
}

// RegisterRoutes registers space routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/spaces", h.Run)
	mux.HandleFunc("GET /api/v1/spaces", h.List)
	mux.HandleFunc("GET /api/v1/spaces/{id}", h.Get)
	mux.HandleFunc("POST /api/v1/spaces/{id}/stop", h.Stop)
	mux.HandleFunc("DELETE /api/v1/spaces/{id}", h.Delete)
	mux.HandleFunc("GET /api/v1/spaces/{id}/logs", h.Logs)
	mux.HandleFunc("POST /api/v1/spaces/prune", h.Prune)
	mux.HandleFunc("POST /api/v1/agents/{id}/health", h.AgentHealth)
	mux.HandleFunc("GET /api/v1/system/info", h.SystemInfo)

	agentapi.NewHandler(newAgentProxyService(h.store), agentapi.WithBasePath(agentapi.AgentProxyBasePattern())).RegisterRoutes(mux)
}

func (h *Handler) Run(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeQuarkError(w, errors.BadRequest(err.Error()))
		return
	}
	if req.RestartPolicy == "" {
		req.RestartPolicy = quarkfileRestartPolicy(req.Dir)
	}
	sp, err := h.controller.Launch(&req)
	if err != nil {
		writeQuarkError(w, errors.Internal("starting space", err))
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, sp)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	spaces, err := h.store.List()
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, spaces)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sp, err := h.store.Get(r.PathValue("id"))
	if err != nil {
		httpserver.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, sp)
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	var req StopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Force = false
	}
	if err := h.controller.Stop(r.PathValue("id"), req.Force); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sp, err := h.store.Get(id)
	if err != nil {
		httpserver.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if sp.Status != StatusStopped && sp.Status != StatusFailed {
		httpserver.WriteError(w, http.StatusConflict,
			fmt.Sprintf("space %q is %s — stop or kill it first", id, sp.Status))
		return
	}
	if err := h.store.Delete(id); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Logs streams log lines buffered from the space-runtime process output.
// Falls back to SSE proxy if the space is running and has a live port.
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sp, err := h.store.Get(id)
	if err != nil {
		httpserver.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	buf := h.controller.GetLogBuf(id)

	// Replay buffered lines using LinesSince(0) to get all lines in seq order.
	if buf != nil {
		history, _ := buf.LinesSince(0)
		for _, line := range history {
			payload, _ := json.Marshal(map[string]string{
				"time":     line.t.Format(time.RFC3339),
				"space_id": id,
				"msg":      line.text,
			})
			fmt.Fprintf(w, "data: %s\n\n", payload)
		}
		fl.Flush()
	}

	// If space is stopped/failed, serve persisted LastLogs then close.
	if sp.Status == StatusStopped || sp.Status == StatusFailed {
		if buf == nil && len(sp.LastLogs) > 0 {
			for _, line := range sp.LastLogs {
				payload, _ := json.Marshal(map[string]string{
					"space_id": id,
					"msg":      line,
				})
				fmt.Fprintf(w, "data: %s\n\n", payload)
			}
			fl.Flush()
		}
		return
	}

	// Stream new lines using monotonic seq so wrap-around never loses lines.
	var lastSeq uint64
	if buf != nil {
		_, lastSeq = buf.LinesSince(0) // capture current high-water mark after replay
	}
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			if buf == nil {
				buf = h.controller.GetLogBuf(id)
			}
			if buf == nil {
				fmt.Fprintf(w, "data: {\"event\":\"waiting\",\"space_id\":%q}\n\n", id)
				fl.Flush()
				continue
			}
			newLines, nextSeq := buf.LinesSince(lastSeq)
			if len(newLines) > 0 {
				for _, line := range newLines {
					payload, _ := json.Marshal(map[string]string{
						"time":     line.t.Format(time.RFC3339),
						"space_id": id,
						"msg":      line.text,
					})
					fmt.Fprintf(w, "data: %s\n\n", payload)
				}
				fl.Flush()
				lastSeq = nextSeq
			}
		}
	}
}

func (h *Handler) AgentHealth(w http.ResponseWriter, r *http.Request) {
	var report HealthReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	agentID := r.PathValue("id")
	if report.AgentID != "" && report.AgentID != agentID {
		httpserver.WriteError(w, http.StatusBadRequest, "agent id mismatch")
		return
	}
	sp, err := h.store.Get(agentID)
	if err != nil {
		httpserver.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	sp.Status = StatusRunning
	sp.PID = report.PID
	sp.Port = report.Port
	h.store.Save(sp)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Prune(w http.ResponseWriter, r *http.Request) {
	spaces, err := h.store.List()
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pruned := []string{}
	for _, sp := range spaces {
		if sp.Status == StatusStopped || sp.Status == StatusFailed {
			if err := h.store.Delete(sp.ID); err == nil {
				pruned = append(pruned, sp.ID)
			}
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string][]string{"pruned": pruned})
}

func (h *Handler) SystemInfo(w http.ResponseWriter, r *http.Request) {
	spaces, _ := h.store.List()
	running := 0
	for _, sp := range spaces {
		if sp.Status == StatusRunning {
			running++
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"version":        serverVersion,
		"spaces_total":   len(spaces),
		"spaces_running": running,
	})
}

func quarkfileRestartPolicy(dir string) string {
	if dir == "" {
		return "on-failure"
	}
	absDir := dir
	if !filepath.IsAbs(dir) {
		cwd, err := os.Getwd()
		if err == nil {
			absDir = filepath.Join(cwd, dir)
		}
	}
	qf, err := quarkfile.Load(absDir)
	if err != nil || qf.Restart == "" {
		return "on-failure"
	}
	return qf.Restart
}

func writeQuarkError(w http.ResponseWriter, err error) {
	var qe *errors.QuarkError
	if stderrors.As(err, &qe) {
		httpserver.WriteError(w, qe.Code, qe.Error())
		return
	}
	httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
}
