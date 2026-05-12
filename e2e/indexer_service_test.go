//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quarkloop/e2e/utils"
	indexerv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/indexer/v1"
	"github.com/quarkloop/pkg/serviceapi/servicekit"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestIndexerServiceWithRealDgraph(t *testing.T) {
	bins := utils.BuildAllOnce(t)
	dgraphAddr := utils.StartDgraph(t)
	indexerAddr := startIndexerService(t, bins.Indexer, dgraphAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := servicekit.Dial(ctx, indexerAddr)
	if err != nil {
		t.Fatalf("dial indexer: %v", err)
	}
	defer conn.Close()

	client := indexerv1.NewIndexerServiceClient(conn)
	if _, err := client.IndexDocument(ctx, &indexerv1.IndexRequest{
		ChunkId:     "e2e-chunk-1",
		TextContent: "Quark service extraction uses gRPC and Dgraph vector indexes.",
		Embedding:   []float32{1, 0, 0},
		Entities: []*indexerv1.Entity{
			{Id: "quark", Name: "Quark", Type: "PROJECT"},
			{Id: "dgraph", Name: "Dgraph", Type: "DATABASE"},
		},
		Relations: []*indexerv1.Relation{
			{FromId: "quark", ToId: "dgraph", Relation: "USES"},
		},
		SourceMetadata: map[string]string{"source": "e2e", "tenant": "quark"},
	}); err != nil {
		t.Fatalf("index document: %v", err)
	}
	resp, err := client.GetContext(ctx, &indexerv1.QueryRequest{
		QueryVector: []float32{1, 0, 0},
		Limit:       5,
		Depth:       2,
		Filters:     map[string]string{"tenant": "quark"},
	})
	if err != nil {
		t.Fatalf("get context: %v", err)
	}
	if len(resp.GetChunks()) == 0 || resp.GetChunks()[0].GetId() != "e2e-chunk-1" {
		t.Fatalf("unexpected chunks: %+v", resp.GetChunks())
	}
	if !strings.Contains(resp.GetReasoningContext(), "Quark service extraction") {
		t.Fatalf("context missing indexed text: %q", resp.GetReasoningContext())
	}
	if !strings.Contains(resp.GetReasoningContext(), "USES") {
		t.Fatalf("context missing graph relation: %q", resp.GetReasoningContext())
	}
}

func startIndexerService(t *testing.T, binary, dgraphAddr string) string {
	t.Helper()
	port := utils.ReservePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	utils.StartProcess(t, "indexer", binary, []string{
		"--addr", addr,
		"--dgraph", dgraphAddr,
		"--skill-dir", filepath.Join(utils.QuarkRoot(t), "services", "indexer"),
	}, utils.ProcessEnv(nil))

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		conn, err := servicekit.Dial(ctx, addr)
		if err == nil {
			_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{
				Service: indexerv1.IndexerService_ServiceDesc.ServiceName,
			})
			conn.Close()
		}
		cancel()
		if err == nil {
			return addr
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("indexer service did not become healthy at %s", addr)
	return ""
}
