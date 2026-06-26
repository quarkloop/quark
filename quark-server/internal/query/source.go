// Package query — source queries (GET /api/v1/namespaces/:ns/systems/:name/source).
package query

import (
	"context"

	"github.com/quarkloop/quark/server/internal/store"
)

// SourceQueryService backs GET /api/v1/namespaces/:ns/systems/:name/source.
type SourceQueryService struct {
	srcRepo store.SourceRepository
}

// NewSourceQueryService constructs a SourceQueryService.
func NewSourceQueryService(srcRepo store.SourceRepository) *SourceQueryService {
	return &SourceQueryService{srcRepo: srcRepo}
}

// GetSource returns the persisted .quark.ts source for a system.
// Returns ErrNotFound if the system has no source persisted.
func (s *SourceQueryService) GetSource(ctx context.Context, namespace, name string) (string, error) {
	src, err := s.srcRepo.GetSource(ctx, namespace, name)
	if err != nil {
		return "", ErrNotFound
	}
	return src, nil
}
