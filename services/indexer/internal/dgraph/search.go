package dgraph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/quarkloop/services/indexer/pkg/indexer"
)

func (d *Driver) VectorSearch(ctx context.Context, queryVector []float32, limit int, filters map[string]string) ([]indexer.Chunk, error) {
	if limit <= 0 {
		limit = 5
	}
	if err := d.ensureMetadataPredicates(ctx, filters); err != nil {
		return nil, err
	}
	resp, err := d.client.NewReadOnlyTxn().QueryWithVars(ctx, vectorSearchQuery(limit, filters), map[string]string{
		"$vec": vectorLiteral(queryVector),
	})
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	var payload vectorSearchPayload
	if err := json.Unmarshal(resp.GetJson(), &payload); err != nil {
		return nil, fmt.Errorf("decode vector search: %w", err)
	}
	return payload.chunks(), nil
}

type vectorSearchPayload struct {
	Chunks []struct {
		ID            string  `json:"quark.chunk_id"`
		Text          string  `json:"quark.text_content"`
		MetadataJSON  string  `json:"quark.metadata_json"`
		CanonicalJSON string  `json:"quark.canonical_json"`
		Score         float32 `json:"score"`
	} `json:"chunks"`
}

func (p vectorSearchPayload) chunks() []indexer.Chunk {
	out := make([]indexer.Chunk, 0, len(p.Chunks))
	for _, row := range p.Chunks {
		meta := map[string]string{}
		if row.MetadataJSON != "" {
			_ = json.Unmarshal([]byte(row.MetadataJSON), &meta)
		}
		var canonical canonicalChunk
		if row.CanonicalJSON != "" {
			_ = json.Unmarshal([]byte(row.CanonicalJSON), &canonical)
		}
		out = append(out, indexer.Chunk{
			ID:                row.ID,
			Text:              row.Text,
			Metadata:          meta,
			Document:          canonical.Document,
			EmbeddingMetadata: canonical.EmbeddingMetadata,
			Facts:             canonical.Facts,
			Citations:         canonical.Citations,
			Provenance:        canonical.Provenance,
			Score:             row.Score,
		})
	}
	return out
}
