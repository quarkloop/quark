package server

import (
	"context"
	"errors"
	"fmt"

	indexerv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/indexer/v1"
	"github.com/quarkloop/services/indexer/internal/indexing"
	"github.com/quarkloop/services/indexer/pkg/indexer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	indexerv1.UnimplementedIndexerServiceServer
	service *indexing.Service
}

func New(service *indexing.Service) (*Server, error) {
	if service == nil {
		return nil, fmt.Errorf("indexing service is required")
	}
	return &Server{service: service}, nil
}

func (s *Server) IndexDocument(ctx context.Context, req *indexerv1.IndexRequest) (*indexerv1.IndexStatus, error) {
	if err := s.service.IndexDocument(ctx, indexCommand(req)); err != nil {
		return nil, grpcError(err)
	}
	return &indexerv1.IndexStatus{Success: true, Message: "indexed"}, nil
}

func (s *Server) GetContext(ctx context.Context, req *indexerv1.QueryRequest) (*indexerv1.ContextResponse, error) {
	result, err := s.service.GetContext(ctx, contextQuery(req))
	if err != nil {
		return nil, grpcError(err)
	}
	return &indexerv1.ContextResponse{
		ReasoningContext: result.ReasoningContext,
		Citations:        result.Citations,
		Chunks:           toProtoChunks(result.Chunks),
		Graph:            toProtoGraph(result.Graph),
	}, nil
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

func grpcError(err error) error {
	var validation *indexing.ValidationError
	if errors.As(err, &validation) {
		return status.Error(codes.InvalidArgument, validation.Error())
	}
	return status.Errorf(codes.Internal, "%v", err)
}
