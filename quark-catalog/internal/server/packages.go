package server

import (
        "encoding/json"
        "log"

        "github.com/nats-io/nats.go"

        "github.com/quarkloop/quark/quark-catalog/internal/api"
)

// registerPackageHandlers wires every registry.node.* subject (the
// node package push/pull API — NOT the catalog.registry.* built-in
// descriptor subjects, which live in registry.go).
func (s *Server) registerPackageHandlers() error {
        handlers := map[string]nats.MsgHandler{
                "registry.node.push":   s.handlePushNode,
                "registry.node.pull":   s.handlePullNode,
                "registry.node.info":   s.handleNodeInfo,
                "registry.node.list":   s.handleListNodePackages,
                "registry.node.search": s.handleSearchNodes,
                "registry.node.delete": s.handleDeleteNodePackage,
                "registry.node.exists": s.handleNodeExists,
        }
        for subject, h := range handlers {
                if err := s.subscribe(subject, h); err != nil {
                        return err
                }
        }
        return nil
}

func (s *Server) handlePushNode(msg *nats.Msg) {
        var req api.PushNodeRequest
        if !decode(msg, &req) {
                return
        }
        if err := s.store.PushNodePackage(req); err != nil {
                replyError(msg, "push failed: %v", err)
                return
        }
        log.Printf("[INFO] Node package pushed: %s (%s, %d bytes)", req.URI, req.ContentType, len(req.Content))
        reply(msg, api.OK)
}

func (s *Server) handlePullNode(msg *nats.Msg) {
        var req api.PullNodeRequest
        if !decode(msg, &req) {
                return
        }
        pkg, err := s.store.PullNodePackage(req.URI)
        if err != nil {
                replyError(msg, "pull failed: %v", err)
                return
        }
        if pkg == nil {
                reply(msg, api.NotFound)
                return
        }
        reply(msg, pkg)
}

func (s *Server) handleNodeInfo(msg *nats.Msg) {
        var req api.NodeInfoRequest
        if !decode(msg, &req) {
                return
        }
        info, err := s.store.GetNodeInfo(req.URI)
        if err != nil {
                replyError(msg, "info failed: %v", err)
                return
        }
        if info == nil {
                reply(msg, api.NotFound)
                return
        }
        reply(msg, info)
}

func (s *Server) handleListNodePackages(msg *nats.Msg) {
        // Empty body → list all; tolerate decode failure.
        var req api.SearchNodesRequest
        _ = json.Unmarshal(msg.Data, &req) //nolint:errcheck // empty body is valid here
        nodes, err := s.store.ListNodePackages()
        if err != nil {
                replyError(msg, "list failed: %v", err)
                return
        }
        if nodes == nil {
                nodes = []api.NodeInfoResponse{}
        }
        reply(msg, api.NodePackageListResponse{Nodes: nodes})
}

func (s *Server) handleSearchNodes(msg *nats.Msg) {
        var req api.SearchNodesRequest
        if !decode(msg, &req) {
                return
        }
        nodes, err := s.store.SearchNodePackages(req.Keyword)
        if err != nil {
                replyError(msg, "search failed: %v", err)
                return
        }
        if nodes == nil {
                nodes = []api.NodeInfoResponse{}
        }
        reply(msg, api.NodePackageListResponse{Nodes: nodes})
}

func (s *Server) handleDeleteNodePackage(msg *nats.Msg) {
        var req api.PullNodeRequest
        if !decode(msg, &req) {
                return
        }
        if err := s.store.DeleteNodePackage(req.URI); err != nil {
                replyError(msg, "delete failed: %v", err)
                return
        }
        reply(msg, api.OK)
}

func (s *Server) handleNodeExists(msg *nats.Msg) {
        var req api.NodeInfoRequest
        if !decode(msg, &req) {
                return
        }
        exists, err := s.store.NodePackageExists(req.URI)
        if err != nil {
                replyError(msg, "exists check failed: %v", err)
                return
        }
        reply(msg, api.ExistsResponse{Exists: exists})
}
