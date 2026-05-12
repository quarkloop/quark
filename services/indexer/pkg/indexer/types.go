package indexer

import "context"

type Chunk struct {
	ID       string
	Text     string
	Vector   []float32
	Metadata map[string]string
	Score    float32
}

type Entity struct {
	ID   string
	Name string
	Type string
}

type Relation struct {
	FromID   string
	ToID     string
	Relation string
}

type GraphNode struct {
	ID    string
	Label string
	Type  string
}

type GraphEdge struct {
	FromID   string
	ToID     string
	Relation string
}

type GraphFragment struct {
	Nodes []GraphNode
	Edges []GraphEdge
}

// GraphVectorDriver is the storage seam for the indexer service. Implementors
// own persistence, vector search, graph writes, lifecycle, and concurrency.
type GraphVectorDriver interface {
	InsertChunk(ctx context.Context, chunk Chunk) error
	VectorSearch(ctx context.Context, queryVector []float32, limit int, filters map[string]string) ([]Chunk, error)
	UpsertEntity(ctx context.Context, entity Entity) error
	LinkChunkEntity(ctx context.Context, chunkID, entityID string) error
	RelateNodes(ctx context.Context, relation Relation) error
	GetNeighborhood(ctx context.Context, nodeID string, depth int) (*GraphFragment, error)
	Ping(ctx context.Context) error
	Close() error
}
