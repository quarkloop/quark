package server

import (
	"github.com/nats-io/nats.go"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
)

// registerNodeHandlers wires every catalog.node.* subject.
func (s *Server) registerNodeHandlers() error {
	handlers := map[string]nats.MsgHandler{
		"catalog.node.save":    s.handleSaveNode,
		"catalog.node.saveAll": s.handleSaveNodes,
		"catalog.node.list":    s.handleListNodes,
		"catalog.node.delete":  s.handleDeleteNodes,
	}
	for subject, h := range handlers {
		if err := s.subscribe(subject, h); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleSaveNode(msg *nats.Msg) {
	var req api.SaveNodeRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.SaveNode(req); err != nil {
		replyError(msg, "save failed: %v", err)
		return
	}
	reply(msg, api.OK)
}

func (s *Server) handleSaveNodes(msg *nats.Msg) {
	var req api.SaveNodesRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.SaveNodes(req.Nodes); err != nil {
		replyError(msg, "saveAll failed: %v", err)
		return
	}
	reply(msg, api.OK)
}

func (s *Server) handleListNodes(msg *nats.Msg) {
	var req api.ListNodesRequest
	if !decode(msg, &req) {
		return
	}
	var nodes []api.NodeResponse
	var err error
	if req.SystemName != "" {
		nodes, err = s.store.ListNodes(req.Namespace, req.SystemName)
	} else {
		nodes, err = s.store.ListNodesByNamespace(req.Namespace)
	}
	if err != nil {
		replyError(msg, "list failed: %v", err)
		return
	}
	if nodes == nil {
		nodes = []api.NodeResponse{}
	}
	reply(msg, api.NodeListResponse{Nodes: nodes})
}

func (s *Server) handleDeleteNodes(msg *nats.Msg) {
	var req api.DeleteNodesRequest
	if !decode(msg, &req) {
		return
	}
	if err := s.store.DeleteNodesBySystem(req.Namespace, req.SystemName); err != nil {
		replyError(msg, "delete failed: %v", err)
		return
	}
	reply(msg, api.OK)
}
