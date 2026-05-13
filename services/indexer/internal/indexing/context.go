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
