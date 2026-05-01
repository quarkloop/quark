package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
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

func (s *Server) handleListSessions(c *fiber.Ctx) error {
	name := c.Params("name")
	store, err := s.store.Sessions(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	return writeJSON(c, fiber.StatusOK, toAPISessions(store.List()))
}

func (s *Server) handleCreateSession(c *fiber.Ctx) error {
	name := c.Params("name")
	store, err := s.store.Sessions(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	var req api.CreateSessionRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return writeError(c, fiber.StatusBadRequest, "invalid body: "+err.Error())
		}
	}
	sess, err := store.Create(sessions.Type(req.Type), req.Title)
	if err != nil {
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}
	out := toAPISession(sess)
	s.events.Publish(events.Event{
		Kind:    events.SessionCreated,
		Space:   name,
		Payload: events.SessionPayload(out.ID, string(out.Type), out.Title),
	})
	return writeJSON(c, fiber.StatusCreated, out)
}

func (s *Server) handleGetSession(c *fiber.Ctx) error {
	name := c.Params("name")
	store, err := s.store.Sessions(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	sess, err := store.Get(c.Params("id"))
	if errors.Is(err, sessions.ErrNotFound) {
		return writeError(c, fiber.StatusNotFound, "session not found")
	}
	if err != nil {
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}
	return writeJSON(c, fiber.StatusOK, toAPISession(sess))
}

func (s *Server) handleDeleteSession(c *fiber.Ctx) error {
	name := c.Params("name")
	id := c.Params("id")
	store, err := s.store.Sessions(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	if err := store.Delete(id); err != nil {
		if errors.Is(err, sessions.ErrNotFound) {
			return writeError(c, fiber.StatusNotFound, "session not found")
		}
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}
	s.events.Publish(events.Event{
		Kind:    events.SessionDeleted,
		Space:   name,
		Payload: events.SessionPayload(id, "", ""),
	})
	return c.SendStatus(fiber.StatusNoContent)
}

// handleEventStream serves GET /v1/spaces/:name/events/stream as SSE.
//
// The stream emits one event per supervisor-published Event scoped to name.
// Clients (typically an agent process) should reconnect on disconnect; the
// supervisor never retains event history.
func (s *Server) handleEventStream(c *fiber.Ctx) error {
	name := c.Params("name")
	if _, err := s.store.Get(name); err != nil {
		return s.writeSpaceError(c, name, err)
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ch, cancel := s.events.Subscribe(name)
	defer cancel()

	ctx := c.Context()
	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		// Send initial comment so clients know the stream is live.
		fmt.Fprintf(w, ": connected\n\n")
		w.Flush()

		enc := json.NewEncoder(w)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "event: %s\ndata: ", ev.Kind)
				if err := enc.Encode(ev.ToWire()); err != nil {
					return
				}
				fmt.Fprint(w, "\n")
				w.Flush()
			}
		}
	})

	return nil
}
