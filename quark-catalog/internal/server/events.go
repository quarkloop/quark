package server

import (
	"log"

	"github.com/nats-io/nats.go"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
)

// registerEventHandler wires every catalog.event.* subject.
//
// Note the method name is singular (registerEventHandler, not
// registerEventHandlers) to satisfy the Start() dispatcher's
// []func() error slice without renaming.
func (s *Server) registerEventHandler() error {
	handlers := map[string]nats.MsgHandler{
		"catalog.event.append":      s.handleAppendEvent,
		"catalog.event.appendBatch": s.handleAppendEvents,
		"catalog.event.query":       s.handleQueryEvents,
		"catalog.event.count":       s.handleCountEvents,
	}
	for subject, h := range handlers {
		if err := s.subscribe(subject, h); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleAppendEvent(msg *nats.Msg) {
	var req api.AppendEventRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.AppendEvent(req); err != nil {
		log.Printf("[ERROR] appendEvent failed: %v", err)
		replyError(msg, "append failed: %v", err)
		return
	}
	reply(msg, api.OK)
}

func (s *Server) handleAppendEvents(msg *nats.Msg) {
	var req api.AppendEventsRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.AppendEvents(req.Events); err != nil {
		log.Printf("[ERROR] appendEvents failed: %v", err)
		replyError(msg, "appendBatch failed: %v", err)
		return
	}
	reply(msg, api.OK)
}

func (s *Server) handleQueryEvents(msg *nats.Msg) {
	var req api.QueryEventsRequest
	if !decode(msg, &req) {
		return
	}
	events, err := s.store.QueryEvents(req)
	if err != nil {
		replyError(msg, "query failed: %v", err)
		return
	}
	if events == nil {
		events = []api.EventResponse{}
	}
	reply(msg, api.EventListResponse{Events: events})
}

func (s *Server) handleCountEvents(msg *nats.Msg) {
	var req api.CountEventsRequest
	if !decode(msg, &req) {
		return
	}
	count, err := s.store.CountEvents(req)
	if err != nil {
		replyError(msg, "count failed: %v", err)
		return
	}
	reply(msg, api.CountResponse{Count: count})
}
