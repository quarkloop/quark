package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/space"
)

// handleListSpaces serves GET /v1/spaces.
func (s *Server) handleListSpaces(w http.ResponseWriter, r *http.Request) {
	spaces, err := s.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]api.SpaceInfo, 0, len(spaces))
	for _, sp := range spaces {
		out = append(out, toSpaceInfo(sp))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCreateSpace serves POST /v1/spaces.
func (s *Server) handleCreateSpace(w http.ResponseWriter, r *http.Request) {
	var req api.CreateSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Quarkfile) == 0 {
		writeError(w, http.StatusBadRequest, "quarkfile is required")
		return
	}

	sp, err := s.store.Create(req.Name, req.Quarkfile)
	if err != nil {
		switch {
		case errors.Is(err, space.ErrAlreadyExists):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, toSpaceInfo(sp))
}

// handleGetSpace serves GET /v1/spaces/{name}.
func (s *Server) handleGetSpace(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	sp, err := s.store.Get(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	writeJSON(w, http.StatusOK, toSpaceInfo(sp))
}

// handleDeleteSpace serves DELETE /v1/spaces/{name}.
func (s *Server) handleDeleteSpace(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := s.agents.GetBySpace(name); err == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("cannot delete space %q while an agent is running", name))
		return
	}
	if err := s.store.Delete(name); err != nil {
		writeSpaceError(w, name, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetQuarkfile serves GET /v1/spaces/{name}/quarkfile.
func (s *Server) handleGetQuarkfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	data, version, err := s.store.Quarkfile(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	sp, err := s.store.Get(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	writeJSON(w, http.StatusOK, api.QuarkfileResponse{
		Name:      name,
		Version:   version,
		Quarkfile: data,
		UpdatedAt: sp.UpdatedAt,
	})
}

// handleUpdateQuarkfile serves PUT /v1/spaces/{name}/quarkfile.
func (s *Server) handleUpdateQuarkfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req api.UpdateQuarkfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if len(req.Quarkfile) == 0 {
		writeError(w, http.StatusBadRequest, "quarkfile is required")
		return
	}
	sp, err := s.store.UpdateQuarkfile(name, req.Quarkfile)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	writeJSON(w, http.StatusOK, toSpaceInfo(sp))
}

// handleDoctor serves POST /v1/spaces/{name}/doctor.
func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	resp, err := s.store.Doctor(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// writeSpaceError maps space errors to HTTP status codes.
func writeSpaceError(w http.ResponseWriter, name string, err error) {
	switch {
	case errors.Is(err, space.ErrNotFound):
		writeError(w, http.StatusNotFound, fmt.Sprintf("space %q not found", name))
	case errors.Is(err, space.ErrAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func toSpaceInfo(sp *space.Space) api.SpaceInfo {
	return api.SpaceInfo{
		Name:      sp.Name,
		Version:   sp.Version,
		CreatedAt: sp.CreatedAt,
		UpdatedAt: sp.UpdatedAt,
	}
}
