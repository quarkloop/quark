// Package query — built-in node descriptor registry
// (GET /api/v1/registry and /api/v1/registry/:uri).
//
// The Go control plane does NOT maintain an in-memory node registry.
// All lookups go through the Catalog via the RegistryRepository.
// This is consistent with the v6 architecture: the runtime is the
// only place that resolves node URIs to actual implementations.
package query

import (
	"context"

	"github.com/quarkloop/quark/server/internal/domain"
	"github.com/quarkloop/quark/server/internal/store"
)

// RegistryQueryService backs GET /api/v1/registry and
// GET /api/v1/registry/:uri.
type RegistryQueryService struct {
	regRepo store.RegistryRepository
}

// NewRegistryQueryService constructs a RegistryQueryService.
func NewRegistryQueryService(regRepo store.RegistryRepository) *RegistryQueryService {
	return &RegistryQueryService{regRepo: regRepo}
}

// RegistryEntry is the response shape. Mirrors cli/internal/model.RegistryEntry.
type RegistryEntry struct {
	URI         string `json:"uri"`
	Description string `json:"description"`
}

// List returns all registry records, optionally filtered by a search
// keyword. An empty keyword returns everything.
func (s *RegistryQueryService) List(ctx context.Context, keyword string) ([]*RegistryEntry, error) {
	var recs []*domain.RegistryRecord
	var err error
	if keyword != "" {
		recs, err = s.regRepo.SearchRegistry(ctx, keyword)
	} else {
		recs, err = s.regRepo.ListRegistry(ctx)
	}
	if err != nil {
		return nil, err
	}
	out := make([]*RegistryEntry, 0, len(recs))
	for _, r := range recs {
		out = append(out, &RegistryEntry{URI: r.URI, Description: r.Description})
	}
	return out, nil
}

// Lookup returns the registry record for a specific URI.
func (s *RegistryQueryService) Lookup(ctx context.Context, uri string) (*RegistryEntry, error) {
	rec, err := s.regRepo.FindRegistry(ctx, uri)
	if err != nil {
		return nil, ErrNotFound
	}
	return &RegistryEntry{URI: rec.URI, Description: rec.Description}, nil
}
