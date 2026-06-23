// Package main — NATS server handlers for the Catalog service.
//
// Registers subscription handlers on catalog.* and registry.* subjects.
// Each handler deserializes a JSON request, performs the database operation,
// and replies with a JSON response.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

// Server wraps a NATS connection and a Store, routing requests to handlers.
type Server struct {
	nc    *nats.Conn
	store *Store
	subs  []*nats.Subscription
}

// NewServer creates a new Catalog server.
func NewServer(nc *nats.Conn, store *Store) *Server {
	return &Server{nc: nc, store: store}
}

// Start registers all NATS subscription handlers.
func (s *Server) Start() error {
	handlers := map[string]nats.MsgHandler{
		// System operations
		"catalog.system.save":       s.handleSaveSystem,
		"catalog.system.get":        s.handleGetSystem,
		"catalog.system.list":       s.handleListSystems,
		"catalog.system.delete":     s.handleDeleteSystem,
		"catalog.system.updateState": s.handleUpdateSystemState,

		// Node operations
		"catalog.node.save":     s.handleSaveNode,
		"catalog.node.saveAll":  s.handleSaveNodes,
		"catalog.node.list":     s.handleListNodes,
		"catalog.node.delete":   s.handleDeleteNodes,

		// Event operations
		"catalog.event.append":       s.handleAppendEvent,
		"catalog.event.appendBatch":  s.handleAppendEvents,
		"catalog.event.query":        s.handleQueryEvents,
		"catalog.event.count":        s.handleCountEvents,

		// Source operations
		"catalog.source.save":   s.handleSaveSource,
		"catalog.source.get":    s.handleGetSource,
		"catalog.source.list":   s.handleListSources,

		// Registry record operations (built-in node registration)
		"catalog.registry.save":  s.handleSaveRegistry,
		"catalog.registry.find":  s.handleFindRegistry,
		"catalog.registry.list":  s.handleListRegistry,
		"catalog.registry.exists": s.handleRegistryExists,

		// Node package registry operations
		"registry.node.push":   s.handlePushNode,
		"registry.node.pull":   s.handlePullNode,
		"registry.node.info":   s.handleNodeInfo,
		"registry.node.list":   s.handleListNodePackages,
		"registry.node.search": s.handleSearchNodes,
		"registry.node.delete": s.handleDeleteNodePackage,
		"registry.node.exists": s.handleNodeExists,
	}

	for subject, handler := range handlers {
		sub, err := s.nc.Subscribe(subject, handler)
		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
		}
		s.subs = append(s.subs, sub)
	}
	return nil
}

// reply sends a JSON response on the reply-to subject.
func reply(msg *nats.Msg, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		errResp, _ := json.Marshal(ErrorResponse{Error: err.Error(), Success: false})
		msg.Respond(errResp)
		return
	}
	msg.Respond(data)
}

// replyError sends an error response.
func replyError(msg *nats.Msg, format string, args ...interface{}) {
	errResp, _ := json.Marshal(ErrorResponse{Error: fmt.Sprintf(format, args...), Success: false})
	msg.Respond(errResp)
}

// decode decodes a JSON request body.
func decode(msg *nats.Msg, v interface{}) error {
	return json.Unmarshal(msg.Data, v)
}

// --- System handlers ---

func (s *Server) handleSaveSystem(msg *nats.Msg) {
	var req SaveSystemRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.SaveSystem(req); err != nil {
		replyError(msg, "save failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handleGetSystem(msg *nats.Msg) {
	var req GetSystemRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	sys, err := s.store.GetSystem(req.Namespace, req.Name)
	if err != nil {
		replyError(msg, "get failed: %v", err)
		return
	}
	if sys == nil {
		reply(msg, ErrorResponse{Success: false, Error: "not found"})
		return
	}
	reply(msg, sys)
}

func (s *Server) handleListSystems(msg *nats.Msg) {
	var req ListSystemsRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	systems, err := s.store.ListSystems(req.Namespace)
	if err != nil {
		replyError(msg, "list failed: %v", err)
		return
	}
	if systems == nil {
		systems = []SystemResponse{}
	}
	reply(msg, SystemListResponse{Systems: systems})
}

func (s *Server) handleDeleteSystem(msg *nats.Msg) {
	var req DeleteSystemRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.DeleteSystem(req.Namespace, req.Name); err != nil {
		replyError(msg, "delete failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handleUpdateSystemState(msg *nats.Msg) {
	var req UpdateSystemStateRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.UpdateSystemState(req.Namespace, req.Name, req.State, req.Health, req.Version); err != nil {
		replyError(msg, "update failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

// --- Node handlers ---

func (s *Server) handleSaveNode(msg *nats.Msg) {
	var req SaveNodeRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.SaveNode(req); err != nil {
		replyError(msg, "save failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handleSaveNodes(msg *nats.Msg) {
	var req SaveNodesRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.SaveNodes(req.Nodes); err != nil {
		replyError(msg, "saveAll failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handleListNodes(msg *nats.Msg) {
	var req ListNodesRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	var nodes []NodeResponse
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
		nodes = []NodeResponse{}
	}
	reply(msg, NodeListResponse{Nodes: nodes})
}

func (s *Server) handleDeleteNodes(msg *nats.Msg) {
	var req DeleteNodesRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.DeleteNodesBySystem(req.Namespace, req.SystemName); err != nil {
		replyError(msg, "delete failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

// --- Event handlers ---

func (s *Server) handleAppendEvent(msg *nats.Msg) {
	var req AppendEventRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.AppendEvent(req); err != nil {
		log.Printf("[ERROR] appendEvent failed: %v", err)
		replyError(msg, "append failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handleAppendEvents(msg *nats.Msg) {
	var req AppendEventsRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.AppendEvents(req.Events); err != nil {
		log.Printf("[ERROR] appendEvents failed: %v", err)
		replyError(msg, "appendBatch failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handleQueryEvents(msg *nats.Msg) {
	var req QueryEventsRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	events, err := s.store.QueryEvents(req)
	if err != nil {
		replyError(msg, "query failed: %v", err)
		return
	}
	if events == nil {
		events = []EventResponse{}
	}
	reply(msg, EventListResponse{Events: events})
}

func (s *Server) handleCountEvents(msg *nats.Msg) {
	var req CountEventsRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	count, err := s.store.CountEvents(req)
	if err != nil {
		replyError(msg, "count failed: %v", err)
		return
	}
	reply(msg, CountResponse{Count: count})
}

// --- Source handlers ---

func (s *Server) handleSaveSource(msg *nats.Msg) {
	var req SaveSourceRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.SaveSource(req.Namespace, req.Name, req.Source); err != nil {
		replyError(msg, "save failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handleGetSource(msg *nats.Msg) {
	var req GetSourceRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	source, err := s.store.GetSource(req.Namespace, req.Name)
	if err != nil {
		replyError(msg, "get failed: %v", err)
		return
	}
	if source == "" {
		reply(msg, ErrorResponse{Success: false, Error: "not found"})
		return
	}
	reply(msg, SourceResponse{Source: source})
}

func (s *Server) handleListSources(msg *nats.Msg) {
	sources, err := s.store.ListSources()
	if err != nil {
		replyError(msg, "list failed: %v", err)
		return
	}
	if sources == nil {
		sources = []SourceEntry{}
	}
	reply(msg, SourceListResponse{Sources: sources})
}

// --- Registry record handlers ---

func (s *Server) handleSaveRegistry(msg *nats.Msg) {
	var req SaveRegistryRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.SaveRegistryRecord(req); err != nil {
		replyError(msg, "save failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handleFindRegistry(msg *nats.Msg) {
	var req FindRegistryRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	rec, err := s.store.FindRegistryRecord(req.URI)
	if err != nil {
		replyError(msg, "find failed: %v", err)
		return
	}
	if rec == nil {
		reply(msg, ErrorResponse{Success: false, Error: "not found"})
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
		records = []RegistryResponse{}
	}
	reply(msg, RegistryListResponse{Records: records})
}

func (s *Server) handleRegistryExists(msg *nats.Msg) {
	var req FindRegistryRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	exists, err := s.store.RegistryExists(req.URI)
	if err != nil {
		replyError(msg, "exists check failed: %v", err)
		return
	}
	reply(msg, ExistsResponse{Exists: exists})
}

// --- Node package registry handlers ---

func (s *Server) handlePushNode(msg *nats.Msg) {
	var req PushNodeRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.PushNodePackage(req); err != nil {
		replyError(msg, "push failed: %v", err)
		return
	}
	log.Printf("[INFO] Node package pushed: %s (%s, %d bytes)", req.URI, req.ContentType, len(req.Content))
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handlePullNode(msg *nats.Msg) {
	var req PullNodeRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	pkg, err := s.store.PullNodePackage(req.URI)
	if err != nil {
		replyError(msg, "pull failed: %v", err)
		return
	}
	if pkg == nil {
		reply(msg, ErrorResponse{Success: false, Error: "not found"})
		return
	}
	reply(msg, pkg)
}

func (s *Server) handleNodeInfo(msg *nats.Msg) {
	var req NodeInfoRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	info, err := s.store.GetNodeInfo(req.URI)
	if err != nil {
		replyError(msg, "info failed: %v", err)
		return
	}
	if info == nil {
		reply(msg, ErrorResponse{Success: false, Error: "not found"})
		return
	}
	reply(msg, info)
}

func (s *Server) handleListNodePackages(msg *nats.Msg) {
	var req SearchNodesRequest
	if err := decode(msg, &req); err != nil {
		// Empty body → list all
		req = SearchNodesRequest{}
	}
	nodes, err := s.store.ListNodePackages(req.Category)
	if err != nil {
		replyError(msg, "list failed: %v", err)
		return
	}
	if nodes == nil {
		nodes = []NodeInfoResponse{}
	}
	reply(msg, NodeListResponseReg{Nodes: nodes})
}

func (s *Server) handleSearchNodes(msg *nats.Msg) {
	var req SearchNodesRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	nodes, err := s.store.SearchNodePackages(req.Keyword)
	if err != nil {
		replyError(msg, "search failed: %v", err)
		return
	}
	if nodes == nil {
		nodes = []NodeInfoResponse{}
	}
	reply(msg, NodeListResponseReg{Nodes: nodes})
}

func (s *Server) handleDeleteNodePackage(msg *nats.Msg) {
	var req PullNodeRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	if err := s.store.DeleteNodePackage(req.URI); err != nil {
		replyError(msg, "delete failed: %v", err)
		return
	}
	reply(msg, ErrorResponse{Success: true})
}

func (s *Server) handleNodeExists(msg *nats.Msg) {
	var req NodeInfoRequest
	if err := decode(msg, &req); err != nil {
		replyError(msg, "invalid request: %v", err)
		return
	}
	exists, err := s.store.NodePackageExists(req.URI)
	if err != nil {
		replyError(msg, "exists check failed: %v", err)
		return
	}
	reply(msg, ExistsResponse{Exists: exists})
}
