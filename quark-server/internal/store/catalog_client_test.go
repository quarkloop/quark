// Package store — integration tests for NatsCatalogClient.
//
// These tests start an in-process NATS server, register mock Catalog
// handlers, and exercise the NatsCatalogClient methods to verify wire
// format compatibility.
package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nats-io/nats.go"

	"github.com/quarkloop/quark/server/internal/domain"
)

// startInProcessNATS starts a real NATS server in-process and returns
// a connected *nats.Conn. The server is cleaned up via t.Cleanup.
//
// Same pattern as the Catalog's tests.
func startInProcessNATS(t *testing.T) *nats.Conn {
	t.Helper()
	s := runNATSServer(t)
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatalf("connect to in-process NATS: %v", err)
	}
	t.Cleanup(func() {
		nc.Close()
		s.Shutdown()
	})
	return nc
}

func TestNatsCatalogClient_SaveAndGetSystem(t *testing.T) {
	nc := startInProcessNATS(t)

	saveCalls := 0
	nc.Subscribe("catalog.system.save", func(msg *nats.Msg) {
		saveCalls++
		nc.Publish(msg.Reply, []byte(`{"success":true}`))
	})
	nc.Subscribe("catalog.system.get", func(msg *nats.Msg) {
		resp := map[string]any{
			"namespace": "alice",
			"name":      "monitor",
			"source":    "export default {...}",
			"state":     "ACTIVE",
			"health":    "HEALTHY",
			"version":   1,
			"createdAt": "2026-06-25T20:00:00Z",
			"updatedAt": "2026-06-25T20:00:00Z",
		}
		bs, _ := json.Marshal(resp)
		nc.Publish(msg.Reply, bs)
	})
	nc.Flush()

	client := NewNatsCatalogClient(nc)
	ctx := context.Background()

	rec := &domain.SystemRecord{
		Namespace: "alice", Name: "monitor", Source: "export default {...}",
		State: "ACTIVE", Health: "HEALTHY", Version: 1,
		CreatedAt: "2026-06-25T20:00:00Z", UpdatedAt: "2026-06-25T20:00:00Z",
	}
	if err := client.SaveSystem(ctx, rec); err != nil {
		t.Fatalf("SaveSystem: %v", err)
	}
	if saveCalls != 1 {
		t.Errorf("saveCalls = %d, want 1", saveCalls)
	}

	got, err := client.GetSystem(ctx, "alice", "monitor")
	if err != nil {
		t.Fatalf("GetSystem: %v", err)
	}
	if got.Name != "monitor" {
		t.Errorf("rec.Name = %q, want monitor", got.Name)
	}
	if got.Namespace != "alice" {
		t.Errorf("rec.Namespace = %q, want alice", got.Namespace)
	}
	if got.State != "ACTIVE" {
		t.Errorf("rec.State = %q, want ACTIVE", got.State)
	}
}

func TestNatsCatalogClient_GetSystem_NotFound(t *testing.T) {
	nc := startInProcessNATS(t)
	nc.Subscribe("catalog.system.get", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"success":false,"error":"not found"}`))
	})
	nc.Flush()

	client := NewNatsCatalogClient(nc)
	_, err := client.GetSystem(context.Background(), "alice", "missing")
	if err == nil {
		t.Error("GetSystem on missing record should return error")
	}
}

func TestNatsCatalogClient_SaveAndListNodes(t *testing.T) {
	nc := startInProcessNATS(t)
	nc.Subscribe("catalog.node.save", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"success":true}`))
	})
	nc.Subscribe("catalog.node.list", func(msg *nats.Msg) {
		resp := map[string]any{
			"nodes": []map[string]any{
				{
					"namespace": "alice", "systemName": "monitor", "name": "timer",
					"uri": "quark/time/schedule/timer:v1", "state": "ACTIVE", "health": "HEALTHY",
					"version": 1, "listens": []string{}, "events": []string{"tick"},
					"createdAt": "2026-06-25T20:00:00Z", "updatedAt": "2026-06-25T20:00:00Z",
				},
			},
		}
		bs, _ := json.Marshal(resp)
		nc.Publish(msg.Reply, bs)
	})
	nc.Flush()

	client := NewNatsCatalogClient(nc)
	ctx := context.Background()

	rec := &domain.NodeRecord{
		Namespace: "alice", SystemName: "monitor", Name: "timer",
		URI: "quark/time/schedule/timer:v1", State: "ACTIVE", Health: "HEALTHY",
		Version: 1, Listens: []string{}, Events: []string{"tick"},
		CreatedAt: "2026-06-25T20:00:00Z", UpdatedAt: "2026-06-25T20:00:00Z",
	}
	if err := client.SaveNode(ctx, rec); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	nodes, err := client.ListNodesBySystem(ctx, "alice", "monitor")
	if err != nil {
		t.Fatalf("ListNodesBySystem: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	if nodes[0].Name != "timer" {
		t.Errorf("nodes[0].Name = %q, want timer", nodes[0].Name)
	}
	if nodes[0].URI != "quark/time/schedule/timer:v1" {
		t.Errorf("nodes[0].URI = %q, want quark/time/schedule/timer:v1", nodes[0].URI)
	}
}

func TestNatsCatalogClient_AppendAndQueryEvents(t *testing.T) {
	nc := startInProcessNATS(t)
	nc.Subscribe("catalog.event.append", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"success":true}`))
	})
	nc.Subscribe("catalog.event.query", func(msg *nats.Msg) {
		resp := map[string]any{
			"events": []map[string]any{
				{
					"id": "abc-123", "kind": "NODE_CREATED", "nodeName": "timer",
					"systemName": "monitor", "namespace": "alice",
					"timestamp": "2026-06-25T20:00:00Z",
				},
			},
		}
		bs, _ := json.Marshal(resp)
		nc.Publish(msg.Reply, bs)
	})
	nc.Subscribe("catalog.event.count", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"count":42}`))
	})
	nc.Flush()

	client := NewNatsCatalogClient(nc)
	ctx := context.Background()

	ev := &domain.NodeEvent{
		ID: "abc-123", Kind: "NODE_CREATED", NodeName: "timer",
		SystemName: "monitor", Namespace: "alice",
		Timestamp: "2026-06-25T20:00:00Z",
	}
	if err := client.AppendEvent(ctx, ev); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	events, err := client.QueryEvents(ctx, &EventFilter{Namespace: "alice", Limit: 100})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].ID != "abc-123" {
		t.Errorf("events[0].ID = %q, want abc-123", events[0].ID)
	}

	count, err := client.CountEvents(ctx, &EventFilter{Namespace: "alice"})
	if err != nil {
		t.Fatalf("CountEvents: %v", err)
	}
	if count != 42 {
		t.Errorf("count = %d, want 42", count)
	}
}

func TestNatsCatalogClient_Source(t *testing.T) {
	nc := startInProcessNATS(t)
	nc.Subscribe("catalog.source.save", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"success":true}`))
	})
	nc.Subscribe("catalog.source.get", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"source":"export default { name: 'test' };"}`))
	})
	nc.Subscribe("catalog.source.list", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"sources":[{"namespace":"alice","name":"monitor"}]}`))
	})
	nc.Flush()

	client := NewNatsCatalogClient(nc)
	ctx := context.Background()

	if err := client.SaveSource(ctx, "alice", "monitor", "export default {}"); err != nil {
		t.Fatalf("SaveSource: %v", err)
	}

	src, err := client.GetSource(ctx, "alice", "monitor")
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if src != "export default { name: 'test' };" {
		t.Errorf("source = %q", src)
	}

	entries, err := client.ListSources(ctx)
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Namespace != "alice" || entries[0].Name != "monitor" {
		t.Errorf("entry = %+v", entries[0])
	}
}

func TestNatsCatalogClient_Registry(t *testing.T) {
	nc := startInProcessNATS(t)
	nc.Subscribe("catalog.registry.save", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"success":true}`))
	})
	nc.Subscribe("catalog.registry.find", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"uri":"quark/time/schedule/timer:v1","pattern":"timer","description":"tick emitter"}`))
	})
	nc.Subscribe("catalog.registry.list", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"records":[{"uri":"quark/time/schedule/timer:v1","pattern":"timer","description":"tick emitter"}]}`))
	})
	nc.Subscribe("catalog.registry.exists", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte(`{"exists":true}`))
	})
	nc.Flush()

	client := NewNatsCatalogClient(nc)
	ctx := context.Background()

	rec := &domain.RegistryRecord{URI: "quark/time/schedule/timer:v1", Pattern: "timer", Description: "tick emitter"}
	if err := client.SaveRegistry(ctx, rec); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}

	got, err := client.FindRegistry(ctx, "quark/time/schedule/timer:v1")
	if err != nil {
		t.Fatalf("FindRegistry: %v", err)
	}
	if got.URI != "quark/time/schedule/timer:v1" {
		t.Errorf("URI = %q", got.URI)
	}

	all, err := client.ListRegistry(ctx)
	if err != nil {
		t.Fatalf("ListRegistry: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("len(all) = %d, want 1", len(all))
	}

	exists, err := client.ExistsRegistry(ctx, "quark/time/schedule/timer:v1")
	if err != nil {
		t.Fatalf("ExistsRegistry: %v", err)
	}
	if !exists {
		t.Error("ExistsRegistry = false, want true")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s, sub string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "xyz", false},
		{"hello", "", true},
		{"", "a", false},
		{"abc", "abc", true},
	}
	for _, tt := range tests {
		got := contains(tt.s, tt.sub)
		if got != tt.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.sub, got, tt.want)
		}
	}
}
