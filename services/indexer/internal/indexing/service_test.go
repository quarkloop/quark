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
			Facts:             []indexer.Fact{{ID: "fact-1", Subject: "Quark", Predicate: "stores", Object: "context", Confidence: 0.8}},
			Citations:         []indexer.Citation{{SourceURI: "fixture.pdf", ChunkID: "chunk-1"}},
			Provenance:        indexer.Provenance{SourceURI: "fixture.pdf", Metadata: map[string]string{"trace": "t1"}},
			Score:             1.2,
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
	result.Package.Chunks[0].Text = "mutated"
	result.Package.Facts[0].Object = "mutated"
	result.Package.Provenance[0].Metadata["trace"] = "mutated"
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
	if store.chunks[0].Facts[0].Object != "context" {
		t.Fatalf("chunk facts were mutated through package: %+v", store.chunks[0].Facts)
	}
	if store.graph.Nodes[0].Label != "Node" {
		t.Fatalf("graph node was mutated through result: %+v", store.graph.Nodes[0])
	}
	if store.graph.Edges[0].Relation != "related" {
		t.Fatalf("graph edge was mutated through result: %+v", store.graph.Edges[0])
	}
	if result.Chunks[0].Score != 1 {
		t.Fatalf("score was not normalized to [0,1]: %f", result.Chunks[0].Score)
	}
	if result.Package.Confidence != 1 {
		t.Fatalf("context package confidence = %f, want 1", result.Package.Confidence)
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

func TestIndexDocumentDeduplicatesPrimaryGraphWrites(t *testing.T) {
	store := &fakeStore{}
	svc, err := New(store)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = svc.IndexDocument(context.Background(), IndexCommand{
		ChunkID: "chunk-1",
		Text:    "Productivity apps include calendar planning.",
		Vector:  []float32{0.1, 0.2},
		Entities: []indexer.Entity{
			{ID: "category_productivity", Name: "Productivity", Type: "CATEGORY"},
			{ID: "category_productivity", Name: "Productivity", Type: "CATEGORY"},
			{ID: "calendar_planner", Name: "Calendar Planner", Type: "APP"},
		},
		Relations: []indexer.Relation{
			{FromID: "calendar_planner", ToID: "category_productivity", Relation: "BELONGS_TO"},
			{FromID: "calendar_planner", ToID: "category_productivity", Relation: "BELONGS_TO"},
		},
	})
	if err != nil {
		t.Fatalf("index document: %v", err)
	}

	if len(store.upserted) != 2 {
		t.Fatalf("upserted entities = %d, want 2: %+v", len(store.upserted), store.upserted)
	}
	if len(store.linked) != 2 {
		t.Fatalf("linked entities = %d, want 2: %+v", len(store.linked), store.linked)
	}
	if len(store.related) != 1 {
		t.Fatalf("relations = %d, want 1: %+v", len(store.related), store.related)
	}
}

func TestBuildContextPackageDeduplicatesEvidence(t *testing.T) {
	chunks := []indexer.Chunk{
		{
			ID:    "chunk-1",
			Text:  "alpha",
			Score: 0.2,
			Facts: []indexer.Fact{{
				ID:         "fact-1",
				Subject:    "A",
				Predicate:  "mentions",
				Object:     "B",
				Confidence: 0.7,
			}},
			Citations:  []indexer.Citation{{ID: "cite-1", SourceURI: "source.pdf", ChunkID: "chunk-1"}},
			Provenance: indexer.Provenance{SourceURI: "source.pdf", TraceID: "trace-1", Metadata: map[string]string{"source": "source.pdf"}},
		},
		{
			ID:         "chunk-2",
			Text:       "beta",
			Score:      0.8,
			Facts:      []indexer.Fact{{ID: "fact-1", Subject: "A", Predicate: "mentions", Object: "B"}},
			Citations:  []indexer.Citation{{ID: "cite-1", SourceURI: "source.pdf", ChunkID: "chunk-1"}},
			Provenance: indexer.Provenance{SourceURI: "source.pdf", TraceID: "trace-1"},
		},
	}

	pkg := BuildContextPackage(chunks, &indexer.GraphFragment{Nodes: []indexer.GraphNode{{ID: "A"}}})
	if len(pkg.Chunks) != 2 || len(pkg.Facts) != 1 || len(pkg.Citations) != 1 || len(pkg.Provenance) != 1 {
		t.Fatalf("unexpected context package: %+v", pkg)
	}
	if pkg.Confidence != 0.5 {
		t.Fatalf("confidence = %f, want 0.5", pkg.Confidence)
	}
	pkg.Provenance[0].Metadata["source"] = "mutated"
	if chunks[0].Provenance.Metadata["source"] != "source.pdf" {
		t.Fatalf("context package leaked provenance metadata backing map")
	}
}

type fakeStore struct {
	inserted []indexer.Chunk
	upserted []indexer.Entity
	linked   []string
	related  []indexer.Relation
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

func (s *fakeStore) UpsertEntity(_ context.Context, entity indexer.Entity) error {
	s.upserted = append(s.upserted, entity)
	return nil
}

func (s *fakeStore) LinkChunkEntity(_ context.Context, _, entityID string) error {
	s.linked = append(s.linked, entityID)
	return nil
}

func (s *fakeStore) RelateNodes(_ context.Context, relation indexer.Relation) error {
	s.related = append(s.related, relation)
	return nil
}

func (s *fakeStore) GetNeighborhood(context.Context, string, int) (*indexer.GraphFragment, error) {
	return s.graph, nil
}
