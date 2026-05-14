package indexing

import (
	"context"
	"testing"

	"github.com/quarkloop/services/indexer/pkg/indexer"
)

func TestGetContextReturnsOwnedCopies(t *testing.T) {
	store := &fakeStore{
		chunks: []indexer.Chunk{{
			ID:                "chunk-1",
			Text:              "hello",
			Vector:            []float32{0.1, 0.2},
			Metadata:          map[string]string{"source": "fixture.pdf"},
			EmbeddingMetadata: indexer.EmbeddingMetadata{Provider: "local", Model: "local-hash-v1", Dimensions: 2},
			Citations:         []indexer.Citation{{SourceURI: "fixture.pdf", ChunkID: "chunk-1"}},
			Provenance:        indexer.Provenance{SourceURI: "fixture.pdf", Metadata: map[string]string{"trace": "t1"}},
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
	result.Chunks[0].Citations[0].SourceURI = "mutated"
	result.Chunks[0].Provenance.Metadata["trace"] = "mutated"
	result.Graph.Nodes[0].Label = "mutated"
	result.Graph.Edges[0].Relation = "mutated"

	if store.chunks[0].Vector[0] != 0.1 {
		t.Fatalf("chunk vector was mutated through result: %+v", store.chunks[0].Vector)
	}
	if store.chunks[0].Metadata["source"] != "fixture.pdf" {
		t.Fatalf("chunk metadata was mutated through result: %+v", store.chunks[0].Metadata)
	}
	if store.chunks[0].Citations[0].SourceURI != "fixture.pdf" {
		t.Fatalf("chunk citations were mutated through result: %+v", store.chunks[0].Citations)
	}
	if store.chunks[0].Provenance.Metadata["trace"] != "t1" {
		t.Fatalf("chunk provenance was mutated through result: %+v", store.chunks[0].Provenance)
	}
	if store.graph.Nodes[0].Label != "Node" {
		t.Fatalf("graph node was mutated through result: %+v", store.graph.Nodes[0])
	}
	if store.graph.Edges[0].Relation != "related" {
		t.Fatalf("graph edge was mutated through result: %+v", store.graph.Edges[0])
	}
}

func TestIndexDocumentNormalizesCanonicalRecord(t *testing.T) {
	store := &fakeStore{}
	svc, err := New(store)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = svc.IndexDocument(context.Background(), IndexCommand{
		ChunkID: "chunk-1",
		Text:    "Quark indexes agent-produced records.",
		Vector:  []float32{0.1, 0.2, 0.3},
		Metadata: map[string]string{
			"path":                   "/tmp/source.pdf",
			"filename":               "source.pdf",
			"document_type":          "paper",
			"embedding_provider":     "local",
			"embedding_model":        "local-hash-v1",
			"embedding_dimensions":   "3",
			"embedding_content_hash": "abc123",
			"trace_id":               "trace-1",
		},
		Facts: []indexer.Fact{{
			Subject:   "Quark",
			Predicate: "indexes",
			Object:    "records",
		}},
	})
	if err != nil {
		t.Fatalf("index document: %v", err)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("inserted chunks = %d, want 1", len(store.inserted))
	}
	chunk := store.inserted[0]
	if chunk.Document.SourceURI != "/tmp/source.pdf" || chunk.Document.Name != "source.pdf" || chunk.Document.Type != "paper" {
		t.Fatalf("document normalization failed: %+v", chunk.Document)
	}
	if chunk.EmbeddingMetadata.Provider != "local" || chunk.EmbeddingMetadata.Model != "local-hash-v1" || chunk.EmbeddingMetadata.Dimensions != 3 || chunk.EmbeddingMetadata.ContentHash != "abc123" {
		t.Fatalf("embedding metadata normalization failed: %+v", chunk.EmbeddingMetadata)
	}
	if chunk.Provenance.SourceURI != "/tmp/source.pdf" || chunk.Provenance.TraceID != "trace-1" {
		t.Fatalf("provenance normalization failed: %+v", chunk.Provenance)
	}
	if len(chunk.Citations) != 1 || chunk.Citations[0].SourceURI != "/tmp/source.pdf" || chunk.Citations[0].ChunkID != "chunk-1" {
		t.Fatalf("citation normalization failed: %+v", chunk.Citations)
	}
	if len(chunk.Facts) != 1 || chunk.Facts[0].Subject != "Quark" || len(chunk.Facts[0].Citations) != 1 {
		t.Fatalf("fact normalization failed: %+v", chunk.Facts)
	}
}

func TestIndexDocumentRejectsEmbeddingDimensionMismatch(t *testing.T) {
	svc, err := New(&fakeStore{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	err = svc.IndexDocument(context.Background(), IndexCommand{
		ChunkID:           "chunk-1",
		Text:              "hello",
		Vector:            []float32{0.1, 0.2},
		EmbeddingMetadata: indexer.EmbeddingMetadata{Dimensions: 3},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

type fakeStore struct {
	inserted []indexer.Chunk
	chunks   []indexer.Chunk
	graph    *indexer.GraphFragment
}

func (s *fakeStore) InsertChunk(_ context.Context, chunk indexer.Chunk) error {
	s.inserted = append(s.inserted, chunk)
	return nil
}

func (s *fakeStore) VectorSearch(context.Context, []float32, int, map[string]string) ([]indexer.Chunk, error) {
	return s.chunks, nil
}

func (s *fakeStore) UpsertEntity(context.Context, indexer.Entity) error { return nil }

func (s *fakeStore) LinkChunkEntity(context.Context, string, string) error { return nil }

func (s *fakeStore) RelateNodes(context.Context, indexer.Relation) error { return nil }

func (s *fakeStore) GetNeighborhood(context.Context, string, int) (*indexer.GraphFragment, error) {
	return s.graph, nil
}
