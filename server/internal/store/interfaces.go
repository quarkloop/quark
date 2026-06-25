// Package store defines the repository interfaces the Go control plane
// uses to talk to the Catalog service over NATS.
//
// Interfaces live on the CONSUMER side (Go duck typing) — the
// catalog_client.go implementation satisfies them implicitly. This
// makes testing with mocks trivial: define a stub struct that
// implements the methods you need, no inheritance required.
//
// Every method takes a context.Context as the first argument and
// respects cancellation. Every method returns an error (no panics,
// no logging — that's the caller's job).
package store

import (
	"context"

	"github.com/quarkloop/quark/server/internal/domain"
)

// SystemRepository persists and queries system records.
type SystemRepository interface {
	SaveSystem(ctx context.Context, rec *domain.SystemRecord) error
	GetSystem(ctx context.Context, namespace, name string) (*domain.SystemRecord, error)
	ListSystems(ctx context.Context, namespace string) ([]*domain.SystemRecord, error)
	ListAllSystems(ctx context.Context) ([]*domain.SystemRecord, error)
	DeleteSystem(ctx context.Context, namespace, name string) error
	UpdateSystemState(ctx context.Context, namespace, name, state, health string, version int64) error
}

// NodeRepository persists and queries node records.
type NodeRepository interface {
	SaveNode(ctx context.Context, rec *domain.NodeRecord) error
	SaveNodes(ctx context.Context, recs []*domain.NodeRecord) error
	FindNode(ctx context.Context, namespace, system, name string) (*domain.NodeRecord, error)
	ListNodesBySystem(ctx context.Context, namespace, system string) ([]*domain.NodeRecord, error)
	ListNodesByNamespace(ctx context.Context, namespace string) ([]*domain.NodeRecord, error)
	DeleteNodesBySystem(ctx context.Context, namespace, system string) error
	UpdateNodeState(ctx context.Context, namespace, system, name, state, health string, version int64, errMsg string) error
}

// EventStore appends and queries NodeEvents.
type EventStore interface {
	AppendEvent(ctx context.Context, e *domain.NodeEvent) error
	AppendEvents(ctx context.Context, events []*domain.NodeEvent) error
	QueryEvents(ctx context.Context, filter *EventFilter) ([]*domain.NodeEvent, error)
	CountEvents(ctx context.Context, filter *EventFilter) (int64, error)
}

// SourceRepository persists and queries the original .quark.ts source.
type SourceRepository interface {
	SaveSource(ctx context.Context, namespace, name, source string) error
	GetSource(ctx context.Context, namespace, name string) (string, error)
	ListSources(ctx context.Context) ([]*domain.SourceEntry, error)
}

// RegistryRepository persists and queries built-in node descriptors.
type RegistryRepository interface {
	SaveRegistry(ctx context.Context, rec *domain.RegistryRecord) error
	FindRegistry(ctx context.Context, uri string) (*domain.RegistryRecord, error)
	ListRegistry(ctx context.Context) ([]*domain.RegistryRecord, error)
	SearchRegistry(ctx context.Context, keyword string) ([]*domain.RegistryRecord, error)
	ExistsRegistry(ctx context.Context, uri string) (bool, error)
}

// EventFilter is the query filter for events. Mirrors the Java
// EventFilter record.
type EventFilter struct {
	Namespace  string
	SystemName string
	NodeName   string
	Kinds      []string
	Limit      int
}
