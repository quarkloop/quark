package spacesvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	spacev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/space/v1"
	spacemodel "github.com/quarkloop/pkg/space"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestSpaceServiceLifecycle(t *testing.T) {
	t.Parallel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(store)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	workDir := t.TempDir()
	qf := spacemodel.DefaultQuarkfile("svc-space")

	created, err := server.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
		Name:       "svc-space",
		Quarkfile:  qf,
		WorkingDir: workDir,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.GetName() != "svc-space" {
		t.Fatalf("created name = %q", created.GetName())
	}
	if _, err := os.Stat(filepath.Join(workDir, spacemodel.QuarkfileName)); err != nil {
		t.Fatalf("working Quarkfile missing: %v", err)
	}

	listed, err := server.ListSpaces(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := len(listed.GetSpaces()); got != 1 {
		t.Fatalf("spaces = %d, want 1", got)
	}

	paths, err := server.GetSpacePaths(ctx, &spacev1.GetSpacePathsRequest{Name: "svc-space"})
	if err != nil {
		t.Fatalf("paths: %v", err)
	}
	if paths.GetPluginsDir() == "" || paths.GetKbDir() == "" || paths.GetSessionsDir() == "" {
		t.Fatalf("incomplete paths: %+v", paths)
	}

	qfResp, err := server.GetQuarkfile(ctx, &spacev1.GetQuarkfileRequest{Name: "svc-space"})
	if err != nil {
		t.Fatalf("quarkfile: %v", err)
	}
	if string(qfResp.GetQuarkfile()) != string(qf) {
		t.Fatal("stored Quarkfile mismatch")
	}
}
