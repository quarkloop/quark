package indexing

import (
	"context"
	"testing"

	"github.com/quarkloop/services/indexer/pkg/indexer"
)

func TestGetContextReturnsOwnedCopies(t *testing.T) {
	store := &fakeStore{
		chunks: []indexer.Chunk{{
			ID:       "chunk-1",
			Text:     "hello",
			Vector:   []float32{0.1, 0.2},
			Metadata: map[string]string{"source": "fixture.pdf"},
		}},
		graph: &indexer.GraphFragment{
			Nodes: []indexer.GraphNode{{ID: "n1", Label: "Node", Type: "THING"}},
			Edges: []indexer.GraphEdge{{FromID: "n1", ToID: "n2", Relation: "related"}},
		},
	}
	svc, err := New(store)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, err := svc.GetContext(context.Background(), ContextQuery{Vector: []float32{0.1}, Limit: 1, Depth: 1})
	if err != nil {
		t.Fatalf("get context: %v", err)
	}

	result.Chunks[0].Vector[0] = 9
	result.Chunks[0].Metadata["source"] = "mutated"
	result.Graph.Nodes[0].Label = "mutated"
	result.Graph.Edges[0].Relation = "mutated"

	if store.chunks[0].Vector[0] != 0.1 {
		t.Fatalf("chunk vector was mutated through result: %+v", store.chunks[0].Vector)
	}
	if store.chunks[0].Metadata["source"] != "fixture.pdf" {
		t.Fatalf("chunk metadata was mutated through result: %+v", store.chunks[0].Metadata)
	}
	if store.graph.Nodes[0].Label != "Node" {
		t.Fatalf("graph node was mutated through result: %+v", store.graph.Nodes[0])
	}
	if store.graph.Edges[0].Relation != "related" {
		t.Fatalf("graph edge was mutated through result: %+v", store.graph.Edges[0])
	}
}

type fakeStore struct {
	chunks []indexer.Chunk
	graph  *indexer.GraphFragment
}

func (s *fakeStore) InsertChunk(context.Context, indexer.Chunk) error { return nil }

func (s *fakeStore) VectorSearch(context.Context, []float32, int, map[string]string) ([]indexer.Chunk, error) {
	return s.chunks, nil
}

func (s *fakeStore) UpsertEntity(context.Context, indexer.Entity) error { return nil }

func (s *fakeStore) LinkChunkEntity(context.Context, string, string) error { return nil }

func (s *fakeStore) RelateNodes(context.Context, indexer.Relation) error { return nil }

func (s *fakeStore) GetNeighborhood(context.Context, string, int) (*indexer.GraphFragment, error) {
	return s.graph, nil
}
