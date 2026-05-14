package indexing

import (
	"context"
	"fmt"
	"strings"

	"github.com/quarkloop/services/indexer/pkg/indexer"
	"golang.org/x/sync/errgroup"
)

const (
	defaultContextLimit = 5
	defaultGraphDepth   = 1
	unknownEntityType   = "UNKNOWN"
)

type Store interface {
	InsertChunk(ctx context.Context, chunk indexer.Chunk) error
	VectorSearch(ctx context.Context, queryVector []float32, limit int, filters map[string]string) ([]indexer.Chunk, error)
	UpsertEntity(ctx context.Context, entity indexer.Entity) error
	LinkChunkEntity(ctx context.Context, chunkID, entityID string) error
	RelateNodes(ctx context.Context, relation indexer.Relation) error
	GetNeighborhood(ctx context.Context, nodeID string, depth int) (*indexer.GraphFragment, error)
}

type Service struct {
	store Store
}

func New(store Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("indexing store is required")
	}
	return &Service{store: store}, nil
}

func (s *Service) IndexDocument(ctx context.Context, cmd IndexCommand) error {
	cmd = normalizeIndexCommand(cmd)
	if err := validateIndexCommand(cmd); err != nil {
		return err
	}

	if err := s.writePrimaryRecords(ctx, cmd); err != nil {
		return err
	}
	if err := s.writeGraphEdges(ctx, cmd); err != nil {
		return err
	}
	return nil
}

func (s *Service) GetContext(ctx context.Context, query ContextQuery) (*ContextResult, error) {
	query = normalizeContextQuery(query)
	if err := validateContextQuery(query); err != nil {
		return nil, err
	}

	chunks, err := s.store.VectorSearch(ctx, query.Vector, query.Limit, query.Filters)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	if len(chunks) == 0 {
		return &ContextResult{}, nil
	}

	graph, err := s.store.GetNeighborhood(ctx, chunks[0].ID, query.Depth)
	if err != nil {
		return nil, fmt.Errorf("graph traversal: %w", err)
	}
	return &ContextResult{
		ReasoningContext: ReasoningContext(chunks, graph),
		Citations:        Citations(chunks),
		Chunks:           cloneChunks(chunks),
		Graph:            cloneGraphFragment(graph),
	}, nil
}

func (s *Service) writePrimaryRecords(ctx context.Context, cmd IndexCommand) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return s.store.InsertChunk(ctx, indexer.Chunk{
			ID:       cmd.ChunkID,
			Text:     cmd.Text,
			Vector:   cloneVector(cmd.Vector),
			Metadata: cloneMetadata(cmd.Metadata),
		})
	})
	for _, entity := range cmd.Entities {
		entity := entity
		g.Go(func() error { return s.store.UpsertEntity(ctx, entity) })
	}
	for _, relation := range cmd.Relations {
		for _, endpoint := range relationEndpoints(relation) {
			endpoint := endpoint
			g.Go(func() error { return s.store.UpsertEntity(ctx, endpoint) })
		}
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("write index records: %w", err)
	}
	return nil
}

func (s *Service) writeGraphEdges(ctx context.Context, cmd IndexCommand) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, entity := range cmd.Entities {
		entityID := normalizedEntityID(entity)
		if entityID == "" {
			continue
		}
		linkEntityID := entityID
		g.Go(func() error { return s.store.LinkChunkEntity(ctx, cmd.ChunkID, linkEntityID) })
	}
	for _, relation := range cmd.Relations {
		relation := relation
		if !completeRelation(relation) {
			continue
		}
		g.Go(func() error { return s.store.RelateNodes(ctx, relation) })
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("write graph edges: %w", err)
	}
	return nil
}

func normalizeIndexCommand(cmd IndexCommand) IndexCommand {
	cmd.ChunkID = strings.TrimSpace(cmd.ChunkID)
	cmd.Text = strings.TrimSpace(cmd.Text)
	cmd.Vector = cloneVector(cmd.Vector)
	cmd.Metadata = cloneMetadata(cmd.Metadata)
	cmd.Entities = normalizeEntities(cmd.Entities)
	cmd.Relations = normalizeRelations(cmd.Relations)
	return cmd
}

func validateIndexCommand(cmd IndexCommand) error {
	if cmd.ChunkID == "" {
		return invalid("chunk_id", "is required")
	}
	if cmd.Text == "" {
		return invalid("text_content", "is required")
	}
	if len(cmd.Vector) == 0 {
		return invalid("embedding", "is required")
	}
	return nil
}

func normalizeContextQuery(query ContextQuery) ContextQuery {
	if query.Limit <= 0 {
		query.Limit = defaultContextLimit
	}
	if query.Depth <= 0 {
		query.Depth = defaultGraphDepth
	}
	query.Vector = cloneVector(query.Vector)
	query.Filters = cloneMetadata(query.Filters)
	return query
}

func validateContextQuery(query ContextQuery) error {
	if len(query.Vector) == 0 {
		return invalid("query_vector", "is required")
	}
	return nil
}

func normalizeEntities(entities []indexer.Entity) []indexer.Entity {
	out := make([]indexer.Entity, 0, len(entities))
	for _, entity := range entities {
		entity.ID = strings.TrimSpace(entity.ID)
		entity.Name = strings.TrimSpace(entity.Name)
		entity.Type = strings.TrimSpace(entity.Type)
		if entity.ID == "" {
			entity.ID = indexer.EntityIDFromName(entity.Name)
		}
		if entity.Name == "" {
			entity.Name = entity.ID
		}
		if entity.Type == "" {
			entity.Type = unknownEntityType
		}
		if entity.ID == "" {
			continue
		}
		out = append(out, entity)
	}
	return out
}

func normalizeRelations(relations []indexer.Relation) []indexer.Relation {
	out := make([]indexer.Relation, 0, len(relations))
	for _, relation := range relations {
		relation.FromID = strings.TrimSpace(relation.FromID)
		relation.ToID = strings.TrimSpace(relation.ToID)
		relation.Relation = strings.TrimSpace(relation.Relation)
		if relation.FromID == "" || relation.ToID == "" || relation.Relation == "" {
			continue
		}
		out = append(out, relation)
	}
	return out
}

func relationEndpoints(relation indexer.Relation) []indexer.Entity {
	if !completeRelation(relation) {
		return nil
	}
	return []indexer.Entity{
		{ID: relation.FromID, Name: relation.FromID, Type: unknownEntityType},
		{ID: relation.ToID, Name: relation.ToID, Type: unknownEntityType},
	}
}

func completeRelation(relation indexer.Relation) bool {
	return relation.FromID != "" && relation.ToID != "" && relation.Relation != ""
}

func normalizedEntityID(entity indexer.Entity) string {
	if entity.ID != "" {
		return entity.ID
	}
	return indexer.EntityIDFromName(entity.Name)
}

func cloneVector(in []float32) []float32 {
	if len(in) == 0 {
		return nil
	}
	out := make([]float32, len(in))
	copy(out, in)
	return out
}

func cloneMetadata(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneChunks(in []indexer.Chunk) []indexer.Chunk {
	out := make([]indexer.Chunk, len(in))
	for i, chunk := range in {
		out[i] = indexer.Chunk{
			ID:       chunk.ID,
			Text:     chunk.Text,
			Vector:   cloneVector(chunk.Vector),
			Metadata: cloneMetadata(chunk.Metadata),
			Score:    chunk.Score,
		}
	}
	return out
}

func cloneGraphFragment(in *indexer.GraphFragment) *indexer.GraphFragment {
	if in == nil {
		return nil
	}
	out := &indexer.GraphFragment{
		Nodes: make([]indexer.GraphNode, len(in.Nodes)),
		Edges: make([]indexer.GraphEdge, len(in.Edges)),
	}
	copy(out.Nodes, in.Nodes)
	copy(out.Edges, in.Edges)
	return out
}
