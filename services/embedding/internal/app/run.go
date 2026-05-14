package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	embeddingv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/embedding/v1"
	servicev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/service/v1"
	"github.com/quarkloop/pkg/serviceapi/servicekit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Config struct {
	Address  string
	SkillDir string
	Logger   *slog.Logger
}

type server struct {
	embeddingv1.UnimplementedEmbeddingServiceServer
}

const defaultDimensions = 32

func Run(ctx context.Context, cfg Config) error {
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1:7304"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(servicekit.UnaryLoggingInterceptor(cfg.Logger)))
	embeddingv1.RegisterEmbeddingServiceServer(grpcServer, &server{})

	healthServer := health.NewServer()
	healthServer.SetServingStatus(embeddingv1.EmbeddingService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	registry := servicekit.NewRegistry()
	skillPath, err := resolveSkillPath(cfg.SkillDir)
	if err != nil {
		return err
	}
	skill, err := servicekit.SkillFromFile("service-embedding", "1.0.0", skillPath)
	if err != nil {
		return err
	}
	if err := registry.Register(&servicev1.ServiceDescriptor{
		Name:    "embedding",
		Type:    "embedding",
		Version: "1.0.0",
		Address: cfg.Address,
		Rpcs: []*servicev1.RpcDescriptor{
			{Service: embeddingv1.EmbeddingService_ServiceDesc.ServiceName, Method: "Embed", Request: "quark.embedding.v1.EmbedRequest", Response: "quark.embedding.v1.EmbedResponse", Description: "Create a deterministic local embedding vector for text."},
		},
		Skills: []*servicev1.SkillDescriptor{skill},
	}); err != nil {
		return err
	}
	servicev1.RegisterServiceRegistryServer(grpcServer, registry)

	ln, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Address, err)
	}
	errCh := make(chan error, 1)
	go func() {
		cfg.Logger.Info("embedding service listening", "addr", cfg.Address)
		errCh <- grpcServer.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		grpcServer.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *server) Embed(ctx context.Context, req *embeddingv1.EmbedRequest) (*embeddingv1.EmbedResponse, error) {
	dimensions := defaultDimensions
	model := strings.TrimSpace(req.GetModel())
	if model == "" {
		model = "local-hash-v1"
	}
	hash := sha256.Sum256([]byte(req.GetInput()))
	return &embeddingv1.EmbedResponse{
		Vector:      deterministicVector(req.GetInput(), dimensions),
		Model:       model,
		Dimensions:  int32(dimensions),
		Provider:    "local",
		ContentHash: hex.EncodeToString(hash[:]),
	}, nil
}

func deterministicVector(text string, dimensions int) []float32 {
	vector := make([]float32, dimensions)
	for _, token := range tokenize(text) {
		sum := sha256.Sum256([]byte(token))
		idx := int(sum[0]) % dimensions
		sign := float32(1)
		if sum[1]%2 == 1 {
			sign = -1
		}
		vector[idx] += sign * (1 + float32(len(token)%7)/10)
	}
	var norm float64
	for _, value := range vector {
		norm += float64(value * value)
	}
	if norm == 0 {
		vector[0] = 1
		return vector
	}
	scale := float32(1 / math.Sqrt(norm))
	for i := range vector {
		vector[i] *= scale
	}
	return vector
}

func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := fields[:0]
	for _, field := range fields {
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func resolveSkillPath(skillDir string) (string, error) {
	if skillDir != "" {
		path := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("find embedding skill at %s: %w", path, err)
		}
		return path, nil
	}
	for _, path := range []string{"SKILL.md", filepath.Join("plugins", "services", "embedding", "SKILL.md")} {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("embedding service SKILL.md not found; pass --skill-dir")
}
