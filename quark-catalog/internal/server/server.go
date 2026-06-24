// Package server wires the Catalog's Store to NATS subscription
// handlers. Each domain (systems, nodes, events, sources, registry,
// packages) has its own file in this package so the handler set is
// navigable; all handlers are methods on Server so callers don't have
// to compose multiple structs.
//
// Convention: every handler is named handleXxx and registered on a
// subject of the form "catalog.<entity>.<verb>" or "registry.node.<verb>".
// Handlers decode the JSON request, call the corresponding Store
// method, and reply with either the result or an ErrorResponse.
package server

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
	"github.com/quarkloop/quark/quark-catalog/internal/store"
)

// Server holds the dependencies every handler needs: a NATS connection
// to subscribe on and a Store to dispatch to.
type Server struct {
	nc    *nats.Conn
	store *store.Store
	subs  []*nats.Subscription
}

// New constructs a Server that will serve the given Store over nc.
// The caller must call Start to register subscriptions.
func New(nc *nats.Conn, st *store.Store) *Server {
	return &Server{nc: nc, store: st}
}

// Start registers all NATS subscription handlers. The handler set is
// assembled from per-domain registration methods (registerSystemHandlers,
// registerNodeHandlers, ...) so adding a new domain only touches one
// file.
func (s *Server) Start() error {
	for _, reg := range []func() error{
		s.registerSystemHandlers,
		s.registerNodeHandlers,
		s.registerEventHandler,
		s.registerSourceHandlers,
		s.registerRegistryHandlers,
		s.registerPackageHandlers,
	} {
		if err := reg(); err != nil {
			return err
		}
	}
	return nil
}

// subscribe is a tiny wrapper around nc.Subscribe that records the
// subscription so Stop() can drain them if needed.
func (s *Server) subscribe(subject string, handler nats.MsgHandler) error {
	sub, err := s.nc.Subscribe(subject, handler)
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", subject, err)
	}
	s.subs = append(s.subs, sub)
	return nil
}

// --- helpers (used by every handler file) ---

// reply serializes v as JSON and publishes it on msg.Reply. If
// serialization fails, replies with an ErrorResponse so the caller
// gets *something* useful on the reply subject rather than silence.
func reply(msg *nats.Msg, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		errBody, _ := json.Marshal(api.NewError("marshal response: %v", err))
		_ = msg.Respond(errBody)
		return
	}
	_ = msg.Respond(data)
}

// replyError replies with an ErrorResponse built from the formatted message.
func replyError(msg *nats.Msg, format string, args ...any) {
	body, _ := json.Marshal(api.NewError(format, args...))
	_ = msg.Respond(body)
}

// decode unmarshals msg.Data into v. On failure, replies with an
// ErrorResponse and returns false so the handler can early-return.
func decode(msg *nats.Msg, v any) bool {
	if err := json.Unmarshal(msg.Data, v); err != nil {
		replyError(msg, "invalid request: %v", err)
		return false
	}
	return true
}
