package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/events"
	"github.com/quarkloop/supervisor/pkg/sessions"
)

// toAPISession converts the internal supervisor session type into the wire
// type returned on the HTTP API.
func toAPISession(s *sessions.Session) api.Session {
	return api.Session{
		ID:        s.ID,
		Space:     s.Space,
		Type:      api.SessionType(s.Type),
		Title:     s.Title,
		Status:    s.Status,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func toAPISessions(in []*sessions.Session) []api.Session {
	out := make([]api.Session, 0, len(in))
	for _, s := range in {
		out = append(out, toAPISession(s))
	}
	return out
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	store, err := s.store.Sessions(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	writeJSON(w, http.StatusOK, toAPISessions(store.List()))
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	store, err := s.store.Sessions(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	var req api.CreateSessionRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
	}
	sess, err := store.Create(sessions.Type(req.Type), req.Title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := toAPISession(sess)
	s.events.Publish(events.Event{
		Kind:    events.SessionCreated,
		Space:   name,
		Payload: events.SessionPayload(out.ID, string(out.Type), out.Title),
	})
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	store, err := s.store.Sessions(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	sess, err := store.Get(r.PathValue("id"))
	if errors.Is(err, sessions.ErrNotFound) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPISession(sess))
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")
	store, err := s.store.Sessions(name)
	if err != nil {
		writeSpaceError(w, name, err)
		return
	}
	if err := store.Delete(id); err != nil {
		if errors.Is(err, sessions.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.events.Publish(events.Event{
		Kind:    events.SessionDeleted,
		Space:   name,
		Payload: events.SessionPayload(id, "", ""),
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleEventStream serves GET /v1/spaces/{name}/events/stream as SSE.
//
// The stream emits one event per supervisor-published Event scoped to name.
// Clients (typically an agent process) should reconnect on disconnect; the
// supervisor never retains event history.
func (s *Server) handleEventStream(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := s.store.Get(name); err != nil {
		writeSpaceError(w, name, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cancel := s.events.Subscribe(name)
	defer cancel()

	// Send initial comment so clients know the stream is live.
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	enc := json.NewEncoder(w)
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write([]byte("event: " + string(ev.Kind) + "\ndata: "))
			if err := enc.Encode(api.Event{
				Kind:    string(ev.Kind),
				Space:   ev.Space,
				Time:    ev.Time,
				Payload: ev.Payload,
			}); err != nil {
				return
			}
			_, _ = w.Write([]byte("\n"))
			flusher.Flush()
		}
	}
}
