package server

import (
	indexerv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/indexer/v1"
	"github.com/quarkloop/services/indexer/internal/indexing"
	"github.com/quarkloop/services/indexer/pkg/indexer"
)

func indexCommand(req *indexerv1.IndexRequest) indexing.IndexCommand {
	return indexing.IndexCommand{
		ChunkID:           req.GetChunkId(),
		Text:              req.GetTextContent(),
		Vector:            cloneVector(req.GetEmbedding()),
		Metadata:          cloneMap(req.GetSourceMetadata()),
		Document:          protoDocument(req.GetDocument()),
		EmbeddingMetadata: protoEmbeddingMetadata(req.GetEmbeddingMetadata()),
		Entities:          protoEntities(req.GetEntities()),
		Relations:         protoRelations(req.GetRelations()),
		Facts:             protoFacts(req.GetFacts()),
		Citations:         protoCitations(req.GetCitations()),
		Provenance:        protoProvenance(req.GetProvenance()),
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

func protoDocument(document *indexerv1.Document) indexer.Document {
	if document == nil {
		return indexer.Document{}
	}
	return indexer.Document{
		ID:        document.GetId(),
		Name:      document.GetName(),
		Type:      document.GetType(),
		SourceURI: document.GetSourceUri(),
		Metadata:  cloneMap(document.GetMetadata()),
	}
}

func protoEmbeddingMetadata(embedding *indexerv1.EmbeddingMetadata) indexer.EmbeddingMetadata {
	if embedding == nil {
		return indexer.EmbeddingMetadata{}
	}
	return indexer.EmbeddingMetadata{
		Provider:    embedding.GetProvider(),
		Model:       embedding.GetModel(),
		Dimensions:  int(embedding.GetDimensions()),
		ContentHash: embedding.GetContentHash(),
		Version:     embedding.GetVersion(),
	}
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

func protoFacts(facts []*indexerv1.Fact) []indexer.Fact {
	out := make([]indexer.Fact, 0, len(facts))
	for _, fact := range facts {
		out = append(out, indexer.Fact{
			ID:         fact.GetId(),
			Subject:    fact.GetSubject(),
			Predicate:  fact.GetPredicate(),
			Object:     fact.GetObject(),
			Confidence: fact.GetConfidence(),
			Citations:  protoCitations(fact.GetCitations()),
			Metadata:   cloneMap(fact.GetMetadata()),
		})
	}
	return out
}

func protoCitations(citations []*indexerv1.Citation) []indexer.Citation {
	out := make([]indexer.Citation, 0, len(citations))
	for _, citation := range citations {
		out = append(out, indexer.Citation{
			ID:          citation.GetId(),
			SourceURI:   citation.GetSourceUri(),
			ChunkID:     citation.GetChunkId(),
			TextSpan:    citation.GetTextSpan(),
			StartOffset: int(citation.GetStartOffset()),
			EndOffset:   int(citation.GetEndOffset()),
			Confidence:  citation.GetConfidence(),
		})
	}
	return out
}

func protoProvenance(provenance *indexerv1.Provenance) indexer.Provenance {
	if provenance == nil {
		return indexer.Provenance{}
	}
	return indexer.Provenance{
		SourceURI:  provenance.GetSourceUri(),
		SourceHash: provenance.GetSourceHash(),
		IngestedAt: provenance.GetIngestedAt(),
		ProducedBy: provenance.GetProducedBy(),
		TraceID:    provenance.GetTraceId(),
		Metadata:   cloneMap(provenance.GetMetadata()),
	}
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
