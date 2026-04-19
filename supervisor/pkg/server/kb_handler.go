package server

import (
	"encoding/json"
	"net/http"

	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/kb"
)

// openKB opens the KB store for the given space name.
func (s *Server) openKB(name string) (kb.Store, error) {
	return s.store.KB(name)
}

func (s *Server) handleKBGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	store, err := s.openKB(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	defer store.Close()

	val, err := store.Get(r.PathValue("namespace"), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, api.KBValueResponse{Value: val})
}

func (s *Server) handleKBSet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	store, err := s.openKB(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	defer store.Close()

	var req api.KBSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := store.Set(r.PathValue("namespace"), r.PathValue("key"), req.Value); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleKBDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	store, err := s.openKB(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	defer store.Close()

	if err := store.Delete(r.PathValue("namespace"), r.PathValue("key")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleKBList(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	store, err := s.openKB(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	defer store.Close()

	keys, err := store.List(r.PathValue("namespace"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, api.KBListResponse{Keys: keys})
}
