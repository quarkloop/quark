package indexing

import "github.com/quarkloop/services/indexer/pkg/indexer"

type IndexCommand struct {
	ChunkID   string
	Text      string
	Vector    []float32
	Metadata  map[string]string
	Entities  []indexer.Entity
	Relations []indexer.Relation
}

type ContextQuery struct {
	Vector  []float32
	Limit   int
	Depth   int
	Filters map[string]string
}

type ContextResult struct {
	ReasoningContext string
	Citations        []string
	Chunks           []indexer.Chunk
	Graph            *indexer.GraphFragment
}
