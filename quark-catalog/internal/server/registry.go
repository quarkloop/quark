package server

import (
	"github.com/nats-io/nats.go"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
)

// registerRegistryHandlers wires every catalog.registry.* subject
// (the built-in node descriptor table — NOT the registry.node.*
// package push/pull subjects, which live in packages.go).
func (s *Server) registerRegistryHandlers() error {
	handlers := map[string]nats.MsgHandler{
		"catalog.registry.save":   s.handleSaveRegistry,
		"catalog.registry.find":   s.handleFindRegistry,
		"catalog.registry.list":   s.handleListRegistry,
		"catalog.registry.exists": s.handleRegistryExists,
	}
	for subject, h := range handlers {
		if err := s.subscribe(subject, h); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleSaveRegistry(msg *nats.Msg) {
	var req api.SaveRegistryRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.SaveRegistryRecord(req); err != nil {
		replyError(msg, "save failed: %v", err)
		return
	}
	reply(msg, api.OK)
}

func (s *Server) handleFindRegistry(msg *nats.Msg) {
	var req api.FindRegistryRequest
	if !decode(msg, &req) {
		return
	}
	rec, err := s.store.FindRegistryRecord(req.URI)
	if err != nil {
		replyError(msg, "find failed: %v", err)
		return
	}
	if rec == nil {
		reply(msg, api.NotFound)
		return
	}
	reply(msg, rec)
}

func (s *Server) handleListRegistry(msg *nats.Msg) {
	records, err := s.store.ListRegistryRecords()
	if err != nil {
		replyError(msg, "list failed: %v", err)
		return
	}
	if records == nil {
		records = []api.RegistryResponse{}
	}
	reply(msg, api.RegistryListResponse{Records: records})
}

func (s *Server) handleRegistryExists(msg *nats.Msg) {
	var req api.FindRegistryRequest
	if !decode(msg, &req) {
		return
	}
	exists, err := s.store.RegistryExists(req.URI)
	if err != nil {
		replyError(msg, "exists check failed: %v", err)
		return
	}
	reply(msg, api.ExistsResponse{Exists: exists})
}
