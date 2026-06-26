// Package query — event queries (GET /api/v1/namespaces/:ns/events
// and /events/count).
package query

import (
	"context"
	"strings"

	"github.com/quarkloop/quark/server/internal/domain"
	"github.com/quarkloop/quark/server/internal/store"
)

// EventQueryService backs GET /api/v1/namespaces/:ns/events and
// /events/count.
type EventQueryService struct {
	eventStore store.EventStore
}

// NewEventQueryService constructs an EventQueryService.
func NewEventQueryService(eventStore store.EventStore) *EventQueryService {
	return &EventQueryService{eventStore: eventStore}
}

// Query fetches events matching the given filter. The namespace is
// required (enforced by the HTTP handler — admin mode is via ?all=true).
func (s *EventQueryService) Query(ctx context.Context, namespace, system, node, kindsCSV string, limit int) ([]*domain.NodeEvent, error) {
	var kinds []string
	if kindsCSV != "" {
		for _, k := range strings.Split(kindsCSV, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				kinds = append(kinds, strings.ToUpper(k))
			}
		}
	}
	if limit <= 0 {
		limit = 100
	}
	filter := &store.EventFilter{
		Namespace:  namespace,
		SystemName: system,
		NodeName:   node,
		Kinds:      kinds,
		Limit:      limit,
	}
	return s.eventStore.QueryEvents(ctx, filter)
}

// Count returns the number of events matching the given filter.
func (s *EventQueryService) Count(ctx context.Context, namespace, system, node, kindsCSV string) (int64, error) {
	var kinds []string
	if kindsCSV != "" {
		for _, k := range strings.Split(kindsCSV, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				kinds = append(kinds, strings.ToUpper(k))
			}
		}
	}
	filter := &store.EventFilter{
		Namespace:  namespace,
		SystemName: system,
		NodeName:   node,
		Kinds:      kinds,
		Limit:      100_000,
	}
	return s.eventStore.CountEvents(ctx, filter)
}
