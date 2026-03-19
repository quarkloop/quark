// Package repo contains the api-server HTTP handlers for space filesystem operations.
// Filesystem work is delegated to the space module; this package handles HTTP
// encoding/decoding and converts between wire types and space module types.
package repo

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/quarkloop/agent/pkg/infra/httpserver"
	"github.com/quarkloop/api-server/pkg/api"
	spacerepo "github.com/quarkloop/tools/space/pkg/repo"
)

// Handler implements HTTP handlers for repo management operations.
type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/repo/init", h.Init)
	mux.HandleFunc("POST /api/v1/repo/lock", h.Lock)
	mux.HandleFunc("POST /api/v1/repo/validate", h.Validate)
	mux.HandleFunc("POST /api/v1/repo/agents", h.AddAgent)
	mux.HandleFunc("DELETE /api/v1/repo/agents/{name}", h.RemoveAgent)
	mux.HandleFunc("GET /api/v1/repo/agents", h.ListAgents)
	mux.HandleFunc("POST /api/v1/repo/skills", h.AddSkill)
	mux.HandleFunc("DELETE /api/v1/repo/skills/{name}", h.RemoveSkill)
	mux.HandleFunc("GET /api/v1/repo/skills", h.ListSkills)
	mux.HandleFunc("POST /api/v1/repo/kb", h.AddKB)
	mux.HandleFunc("DELETE /api/v1/repo/kb", h.RemoveKB)
	mux.HandleFunc("GET /api/v1/repo/kb", h.ListKB)
	mux.HandleFunc("GET /api/v1/repo/kb/show", h.ShowKB)
	mux.HandleFunc("POST /api/v1/registry/scaffold", h.ScaffoldRegistry)
}

type dirRequest interface{ GetDir() string }

func handleDir[T dirRequest](w http.ResponseWriter, r *http.Request, errStatus int, fn func(string) error) {
	var req T
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	dir, err := absDir(req.GetDir())
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := fn(dir); err != nil {
		httpserver.WriteError(w, errStatus, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Init(w http.ResponseWriter, r *http.Request) {
	handleDir[*api.InitRepoRequest](w, r, http.StatusInternalServerError, spacerepo.Init)
}
func (h *Handler) Lock(w http.ResponseWriter, r *http.Request) {
	handleDir[*api.LockRepoRequest](w, r, http.StatusInternalServerError, spacerepo.Lock)
}
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	handleDir[*api.ValidateRepoRequest](w, r, http.StatusBadRequest, spacerepo.Validate)
}

func (h *Handler) ScaffoldRegistry(w http.ResponseWriter, r *http.Request) {
	if err := spacerepo.ScaffoldRegistry(); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AddAgent(w http.ResponseWriter, r *http.Request) {
	var req api.AgentAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	dir, _ := absDir(req.Dir)
	if err := spacerepo.AddAgent(dir, req.Ref, req.Name); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveAgent(w http.ResponseWriter, r *http.Request) {
	var req api.AgentRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	dir, _ := absDir(req.Dir)
	if err := spacerepo.RemoveAgent(dir, req.Name); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	dir, _ := absDir(r.URL.Query().Get("dir"))
	entries, err := spacerepo.ListAgents(dir)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]api.AgentItem, len(entries))
	for i, e := range entries {
		items[i] = api.AgentItem{Name: e.Name, Ref: e.Ref}
	}
	httpserver.WriteJSON(w, http.StatusOK, api.AgentListResponse{Agents: items})
}

func (h *Handler) AddSkill(w http.ResponseWriter, r *http.Request) {
	var req api.SkillAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	dir, _ := absDir(req.Dir)
	if err := spacerepo.AddSkill(dir, req.Ref, req.Name); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveSkill(w http.ResponseWriter, r *http.Request) {
	var req api.SkillRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	dir, _ := absDir(req.Dir)
	if err := spacerepo.RemoveSkill(dir, req.Name); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListSkills(w http.ResponseWriter, r *http.Request) {
	dir, _ := absDir(r.URL.Query().Get("dir"))
	entries, err := spacerepo.ListSkills(dir)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]api.SkillItem, len(entries))
	for i, e := range entries {
		items[i] = api.SkillItem{Name: e.Name, Ref: e.Ref}
	}
	httpserver.WriteJSON(w, http.StatusOK, api.SkillListResponse{Skills: items})
}

func (h *Handler) AddKB(w http.ResponseWriter, r *http.Request) {
	var req api.KBAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	dir, _ := absDir(req.Dir)
	if err := spacerepo.AddKBEntry(dir, req.Path, req.Value); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveKB(w http.ResponseWriter, r *http.Request) {
	var req api.KBRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	dir, _ := absDir(req.Dir)
	if err := spacerepo.RemoveKBEntry(dir, req.Path); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListKB(w http.ResponseWriter, r *http.Request) {
	dir, _ := absDir(r.URL.Query().Get("dir"))
	entries, err := spacerepo.ListKBEntries(dir)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	files := make([]api.KBFile, len(entries))
	for i, e := range entries {
		files[i] = api.KBFile{Path: e.Path, Size: int64(e.Size)}
	}
	httpserver.WriteJSON(w, http.StatusOK, api.KBListResponse{Files: files})
}

func (h *Handler) ShowKB(w http.ResponseWriter, r *http.Request) {
	dir, _ := absDir(r.URL.Query().Get("dir"))
	path := r.URL.Query().Get("path")
	val, err := spacerepo.ShowKBEntry(dir, path)
	if err != nil {
		httpserver.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(val)
}

func absDir(dir string) (string, error) {
	if dir == "" {
		return ".", nil
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir), nil
	}
	return filepath.Abs(dir)
}

// ScaffoldRegistry seeds the local registry. Re-exported from space module for
// use by the api-server startup sequence.
func ScaffoldRegistry() error {
	return spacerepo.ScaffoldRegistry()
}
