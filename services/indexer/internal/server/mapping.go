package server

import (
	indexerv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/indexer/v1"
	"github.com/quarkloop/services/indexer/internal/indexing"
	"github.com/quarkloop/services/indexer/pkg/indexer"
)

func indexCommand(req *indexerv1.IndexRequest) indexing.IndexCommand {
	return indexing.IndexCommand{
		ChunkID:   req.GetChunkId(),
		Text:      req.GetTextContent(),
		Vector:    cloneVector(req.GetEmbedding()),
		Metadata:  cloneMap(req.GetSourceMetadata()),
		Entities:  protoEntities(req.GetEntities()),
		Relations: protoRelations(req.GetRelations()),
	}
}

func contextQuery(req *indexerv1.QueryRequest) indexing.ContextQuery {
	return indexing.ContextQuery{
		Vector:  cloneVector(req.GetQueryVector()),
		Depth:   int(req.GetDepth()),
		Limit:   int(req.GetLimit()),
		Filters: cloneMap(req.GetFilters()),
	}
}

func protoEntities(entities []*indexerv1.Entity) []indexer.Entity {
	out := make([]indexer.Entity, 0, len(entities))
	for _, entity := range entities {
		out = append(out, indexer.Entity{
			ID:   entity.GetId(),
			Name: entity.GetName(),
			Type: entity.GetType(),
		})
	}
	return out
}

func protoRelations(relations []*indexerv1.Relation) []indexer.Relation {
	out := make([]indexer.Relation, 0, len(relations))
	for _, relation := range relations {
		out = append(out, indexer.Relation{
			FromID:   relation.GetFromId(),
			ToID:     relation.GetToId(),
			Relation: relation.GetRelation(),
		})
	}
	return out
}

func cloneVector(in []float32) []float32 {
	if len(in) == 0 {
		return nil
	}
	out := make([]float32, len(in))
	copy(out, in)
	return out
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
