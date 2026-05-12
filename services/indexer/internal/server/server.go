package server

import (
	"context"
	"fmt"
	"strings"

	indexerv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/indexer/v1"
	"github.com/quarkloop/services/indexer/pkg/indexer"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	indexerv1.UnimplementedIndexerServiceServer
	driver indexer.GraphVectorDriver
}

func New(driver indexer.GraphVectorDriver) (*Server, error) {
	if driver == nil {
		return nil, fmt.Errorf("indexer driver is required")
	}
	return &Server{driver: driver}, nil
}

func (s *Server) IndexDocument(ctx context.Context, req *indexerv1.IndexRequest) (*indexerv1.IndexStatus, error) {
	if req.GetChunkId() == "" {
		return nil, status.Error(codes.InvalidArgument, "chunk_id is required")
	}
	if strings.TrimSpace(req.GetTextContent()) == "" {
		return nil, status.Error(codes.InvalidArgument, "text_content is required")
	}

	parentCtx := ctx
	g, ctx := errgroup.WithContext(parentCtx)
	chunk := indexer.Chunk{
		ID:       req.GetChunkId(),
		Text:     req.GetTextContent(),
		Vector:   append([]float32(nil), req.GetEmbedding()...),
		Metadata: cloneMap(req.GetSourceMetadata()),
	}
	g.Go(func() error { return s.driver.InsertChunk(ctx, chunk) })
	for _, entity := range req.GetEntities() {
		entity := entity
		g.Go(func() error {
			return s.driver.UpsertEntity(ctx, indexer.Entity{
				ID:   entity.GetId(),
				Name: entity.GetName(),
				Type: entity.GetType(),
			})
		})
	}
	for _, relation := range req.GetRelations() {
		relation := relation
		for _, endpoint := range []string{relation.GetFromId(), relation.GetToId()} {
			endpoint := endpoint
			if endpoint == "" {
				continue
			}
			g.Go(func() error {
				return s.driver.UpsertEntity(ctx, indexer.Entity{ID: endpoint, Name: endpoint, Type: "UNKNOWN"})
			})
		}
	}
	if err := g.Wait(); err != nil {
		return nil, status.Errorf(codes.Internal, "storage write failed: %v", err)
	}

	g, ctx = errgroup.WithContext(parentCtx)
	for _, entity := range req.GetEntities() {
		entityID := entity.GetId()
		if entityID == "" {
			entityID = indexer.EntityIDFromName(entity.GetName())
		}
		if entityID == "" {
			continue
		}
		g.Go(func() error { return s.driver.LinkChunkEntity(ctx, req.GetChunkId(), entityID) })
	}
	for _, relation := range req.GetRelations() {
		relation := relation
		g.Go(func() error {
			return s.driver.RelateNodes(ctx, indexer.Relation{
				FromID:   relation.GetFromId(),
				ToID:     relation.GetToId(),
				Relation: relation.GetRelation(),
			})
		})
	}
	if err := g.Wait(); err != nil {
		return nil, status.Errorf(codes.Internal, "storage write failed: %v", err)
	}
	return &indexerv1.IndexStatus{Success: true, Message: "indexed"}, nil
}

func (s *Server) GetContext(ctx context.Context, req *indexerv1.QueryRequest) (*indexerv1.ContextResponse, error) {
	if len(req.GetQueryVector()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "query_vector is required")
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 5
	}
	depth := int(req.GetDepth())
	if depth <= 0 {
		depth = 1
	}
	chunks, err := s.driver.VectorSearch(ctx, req.GetQueryVector(), limit, req.GetFilters())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "vector search failed: %v", err)
	}
	if len(chunks) == 0 {
		return &indexerv1.ContextResponse{}, nil
	}
	graph, err := s.driver.GetNeighborhood(ctx, chunks[0].ID, depth)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "graph traversal failed: %v", err)
	}
	return &indexerv1.ContextResponse{
		ReasoningContext: flattenContext(chunks, graph),
		Citations:        citations(chunks),
		Chunks:           toProtoChunks(chunks),
		Graph:            toProtoGraph(graph),
	}, nil
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func flattenContext(chunks []indexer.Chunk, graph *indexer.GraphFragment) string {
	var b strings.Builder
	for _, chunk := range chunks {
		fmt.Fprintf(&b, "Chunk %s (score %.4f):\n%s\n\n", chunk.ID, chunk.Score, chunk.Text)
	}
	if graph != nil && len(graph.Edges) > 0 {
		b.WriteString("Graph relationships:\n")
		for _, edge := range graph.Edges {
			fmt.Fprintf(&b, "- %s -[%s]-> %s\n", edge.FromID, edge.Relation, edge.ToID)
		}
	}
	return strings.TrimSpace(b.String())
}

func citations(chunks []indexer.Chunk) []string {
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if source := chunk.Metadata["source"]; source != "" {
			out = append(out, source)
			continue
		}
		if path := chunk.Metadata["path"]; path != "" {
			out = append(out, path)
			continue
		}
		out = append(out, chunk.ID)
	}
	return out
}

func toProtoChunks(chunks []indexer.Chunk) []*indexerv1.Chunk {
	out := make([]*indexerv1.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, &indexerv1.Chunk{
			Id:       chunk.ID,
			Text:     chunk.Text,
			Score:    chunk.Score,
			Metadata: cloneMap(chunk.Metadata),
		})
	}
	return out
}

func toProtoGraph(graph *indexer.GraphFragment) *indexerv1.GraphFragment {
	if graph == nil {
		return nil
	}
	out := &indexerv1.GraphFragment{
		Nodes: make([]*indexerv1.GraphNode, 0, len(graph.Nodes)),
		Edges: make([]*indexerv1.GraphEdge, 0, len(graph.Edges)),
	}
	for _, node := range graph.Nodes {
		out.Nodes = append(out.Nodes, &indexerv1.GraphNode{Id: node.ID, Label: node.Label, Type: node.Type})
	}
	for _, edge := range graph.Edges {
		out.Edges = append(out.Edges, &indexerv1.GraphEdge{FromId: edge.FromID, ToId: edge.ToID, Relation: edge.Relation})
	}
	return out
}
