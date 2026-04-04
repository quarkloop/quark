// Package kbserver provides HTTP handlers for the knowledge base.
// It wraps cli/pkg/kb and serves a RESTful API.
package kbserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/quarkloop/cli/pkg/kb"
)

// Mux returns an http.ServeMux wired to the KB REST API.
func Mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/kb/{id}", handleGet)
	mux.HandleFunc("POST /api/v1/kb", handleSet)
	mux.HandleFunc("DELETE /api/v1/kb/{id}", handleDelete)
	mux.HandleFunc("GET /api/v1/kb", handleList)
	return mux
}

type setRequestBody struct {
	ID    string `json:"id"`
	Value []byte `json:"value"`
}

type getResponseBody struct {
	Value []byte `json:"value"`
}

type listResponseBody struct {
	Keys []string `json:"keys"`
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, closer := openStore(r)
	if s == nil {
		return
	}
	defer closer()

	ns, key := kb.SplitID(id)
	val, err := s.Get(ns, key)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, getResponseBody{Value: val})
}

func handleSet(w http.ResponseWriter, r *http.Request) {
	s, closer := openStore(r)
	if s == nil {
		return
	}
	defer closer()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrCode(w, http.StatusBadRequest, fmt.Errorf("read body: %w", err))
		return
	}
	var req setRequestBody
	if err := json.Unmarshal(body, &req); err != nil {
		writeErrCode(w, http.StatusBadRequest, fmt.Errorf("invalid json"))
		return
	}
	if err := kb.ValidateID(req.ID); err != nil {
		writeErr(w, err)
		return
	}
	ns, key := kb.SplitID(req.ID)
	if err := s.Set(ns, key, req.Value); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, closer := openStore(r)
	if s == nil {
		return
	}
	defer closer()

	ns, key := kb.SplitID(id)
	if err := s.Delete(ns, key); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleList(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		writeErrCode(w, http.StatusBadRequest, fmt.Errorf("namespace query parameter required"))
		return
	}
	s, closer := openStore(r)
	if s == nil {
		return
	}
	defer closer()

	keys, err := s.List(ns)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, listResponseBody{Keys: keys})
}

// openStore creates a kb.Store from the request's ?dir= query parameter.
// Returns nil+true on error (already wrote the response). Caller must defer closer().
func openStore(r *http.Request) (kb.Store, func()) {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = "."
	}
	s, err := kb.Open(dir)
	if err != nil {
		writeErrCode(nil, http.StatusInternalServerError, err)
		return nil, func() {}
	}
	return s, func() { _ = s.Close() }
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	writeErrCode(w, http.StatusBadRequest, err)
}

func writeErrCode(w http.ResponseWriter, code int, err error) {
	if w == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
