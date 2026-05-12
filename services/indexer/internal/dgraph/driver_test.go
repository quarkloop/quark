package dgraph

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	indexerv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/indexer/v1"
	"github.com/quarkloop/services/indexer/internal/server"
)

func TestDgraphHelpersBuildVectorAndFilters(t *testing.T) {
	t.Parallel()
	if got := vectorLiteral([]float32{1, 0.5}); got != "[1,0.5]" {
		t.Fatalf("vectorLiteral = %q", got)
	}
	filter := dgraphFilter(map[string]string{"tenant": "acme"})
	if !strings.Contains(filter, "@filter(eq(quark.meta_tenant_") || !strings.Contains(filter, `"acme"`) {
		t.Fatalf("unexpected filter: %q", filter)
	}
}

func TestDgraphEntityListAcceptsScalarAndList(t *testing.T) {
	t.Parallel()

	var scalar dgraphEntityList
	if err := json.Unmarshal([]byte(`{"quark.entity_id":"quark","quark.entity_name":"Quark"}`), &scalar); err != nil {
		t.Fatalf("decode scalar entity: %v", err)
	}
	if len(scalar) != 1 || scalar[0].ID != "quark" {
		t.Fatalf("scalar decode = %+v", scalar)
	}

	var list dgraphEntityList
	if err := json.Unmarshal([]byte(`[{"quark.entity_id":"dgraph","quark.entity_name":"Dgraph"}]`), &list); err != nil {
		t.Fatalf("decode entity list: %v", err)
	}
	if len(list) != 1 || list[0].ID != "dgraph" {
		t.Fatalf("list decode = %+v", list)
	}
}

func TestIndexerWithDgraph(t *testing.T) {
	addr := os.Getenv("DGRAPH_TEST_ADDR")
	if addr == "" {
		t.Skip("set DGRAPH_TEST_ADDR to run Dgraph-backed indexer integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	driver, err := New(ctx, Config{Address: addr})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	srv, err := server.New(driver)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.IndexDocument(ctx, &indexerv1.IndexRequest{
		ChunkId:     "chunk-1",
		TextContent: "Quark extracts services behind gRPC contracts.",
		Embedding:   []float32{1, 0},
		Entities: []*indexerv1.Entity{
			{Id: "quark", Name: "Quark", Type: "PROJECT"},
			{Id: "grpc", Name: "gRPC", Type: "TECH"},
		},
		Relations: []*indexerv1.Relation{
			{FromId: "quark", ToId: "grpc", Relation: "USES"},
		},
		SourceMetadata: map[string]string{"source": "docs/plan.md", "tenant": "test"},
	}); err != nil {
		t.Fatal(err)
	}
	resp, err := srv.GetContext(ctx, &indexerv1.QueryRequest{
		QueryVector: []float32{1, 0},
		Limit:       3,
		Depth:       2,
		Filters:     map[string]string{"tenant": "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(resp.GetChunks()); got != 1 {
		t.Fatalf("chunks = %d, want 1", got)
	}
	if resp.GetChunks()[0].GetId() != "chunk-1" {
		t.Fatalf("top chunk = %q, want chunk-1", resp.GetChunks()[0].GetId())
	}
	if !strings.Contains(resp.GetReasoningContext(), "Graph relationships") {
		t.Fatalf("reasoning context missing graph: %q", resp.GetReasoningContext())
	}
}
