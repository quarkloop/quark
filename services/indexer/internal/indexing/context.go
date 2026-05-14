package indexing

import (
	"fmt"
	"strings"

	"github.com/quarkloop/services/indexer/pkg/indexer"
)

func ReasoningContext(chunks []indexer.Chunk, graph *indexer.GraphFragment) string {
	var b strings.Builder
	for _, chunk := range chunks {
		fmt.Fprintf(&b, "Chunk %s (score %.4f):\n%s\n\n", chunk.ID, chunk.Score, chunk.Text)
		if chunk.Document.SourceURI != "" || chunk.Provenance.SourceURI != "" {
			fmt.Fprintf(&b, "Source: %s\n\n", firstNonEmpty(chunk.Provenance.SourceURI, chunk.Document.SourceURI))
		}
		if len(chunk.Facts) > 0 {
			b.WriteString("Facts:\n")
			for _, fact := range chunk.Facts {
				fmt.Fprintf(&b, "- %s %s %s", fact.Subject, fact.Predicate, fact.Object)
				if fact.Confidence > 0 {
					fmt.Fprintf(&b, " (confidence %.2f)", fact.Confidence)
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}
	if graph != nil && len(graph.Edges) > 0 {
		b.WriteString("Graph relationships:\n")
		for _, edge := range graph.Edges {
			fmt.Fprintf(&b, "- %s -[%s]-> %s\n", edge.FromID, edge.Relation, edge.ToID)
		}
	}
	return strings.TrimSpace(b.String())
}

func Citations(chunks []indexer.Chunk) []string {
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		for _, citation := range chunk.Citations {
			if citation.SourceURI != "" {
				out = append(out, citation.SourceURI)
			} else if citation.ChunkID != "" {
				out = append(out, citation.ChunkID)
			}
		}
		if len(chunk.Citations) > 0 {
			continue
		}
		if source := chunk.Provenance.SourceURI; source != "" {
			out = append(out, source)
			continue
		}
		if source := chunk.Document.SourceURI; source != "" {
			out = append(out, source)
			continue
		}
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
