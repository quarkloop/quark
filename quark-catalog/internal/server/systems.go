package server

import (
	"github.com/nats-io/nats.go"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
)

// registerSystemHandlers wires every catalog.system.* subject to its
// handler. Each handler is a method on Server so it has access to
// s.store without closure capture.
func (s *Server) registerSystemHandlers() error {
	handlers := map[string]nats.MsgHandler{
		"catalog.system.save":        s.handleSaveSystem,
		"catalog.system.get":         s.handleGetSystem,
		"catalog.system.list":        s.handleListSystems,
		"catalog.system.delete":      s.handleDeleteSystem,
		"catalog.system.updateState": s.handleUpdateSystemState,
	}
	for subject, h := range handlers {
		if err := s.subscribe(subject, h); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleSaveSystem(msg *nats.Msg) {
	var req api.SaveSystemRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.SaveSystem(req); err != nil {
		replyError(msg, "save failed: %v", err)
		return
	}
	reply(msg, api.OK)
}

func (s *Server) handleGetSystem(msg *nats.Msg) {
	var req api.GetSystemRequest
	if !decode(msg, &req) {
		return
	}
	sys, err := s.store.GetSystem(req.Namespace, req.Name)
	if err != nil {
		replyError(msg, "get failed: %v", err)
		return
	}
	if sys == nil {
		reply(msg, api.NotFound)
		return
	}
	reply(msg, sys)
}

func (s *Server) handleListSystems(msg *nats.Msg) {
	var req api.ListSystemsRequest
	if !decode(msg, &req) {
		return
	}
	systems, err := s.store.ListSystems(req.Namespace)
	if err != nil {
		replyError(msg, "list failed: %v", err)
		return
	}
	if systems == nil {
		systems = []api.SystemResponse{}
	}
	reply(msg, api.SystemListResponse{Systems: systems})
}

func (s *Server) handleDeleteSystem(msg *nats.Msg) {
	var req api.DeleteSystemRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.DeleteSystem(req.Namespace, req.Name); err != nil {
		replyError(msg, "delete failed: %v", err)
		return
	}
	reply(msg, api.OK)
}

func (s *Server) handleUpdateSystemState(msg *nats.Msg) {
	var req api.UpdateSystemStateRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.UpdateSystemState(req.Namespace, req.Name, req.State, req.Health, req.Version); err != nil {
		replyError(msg, "update failed: %v", err)
		return
	}
	reply(msg, api.OK)
}
