package server

import (
	"github.com/nats-io/nats.go"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
)

// registerSourceHandlers wires every catalog.source.* subject.
func (s *Server) registerSourceHandlers() error {
	handlers := map[string]nats.MsgHandler{
		"catalog.source.save": s.handleSaveSource,
		"catalog.source.get":  s.handleGetSource,
		"catalog.source.list": s.handleListSources,
	}
	for subject, h := range handlers {
		if err := s.subscribe(subject, h); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleSaveSource(msg *nats.Msg) {
	var req api.SaveSourceRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.SaveSource(req.Namespace, req.Name, req.Source); err != nil {
		replyError(msg, "save failed: %v", err)
		return
	}
	reply(msg, api.OK)
}

func (s *Server) handleGetSource(msg *nats.Msg) {
	var req api.GetSourceRequest
	if !decode(msg, &req) {
		return
	}
	source, err := s.store.GetSource(req.Namespace, req.Name)
	if err != nil {
		replyError(msg, "get failed: %v", err)
		return
	}
	if source == "" {
		reply(msg, api.NotFound)
		return
	}
	reply(msg, api.SourceResponse{Source: source})
}

func (s *Server) handleListSources(msg *nats.Msg) {
	sources, err := s.store.ListSources()
	if err != nil {
		replyError(msg, "list failed: %v", err)
		return
	}
	if sources == nil {
		sources = []api.SourceEntry{}
	}
	reply(msg, api.SourceListResponse{Sources: sources})
}
