package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
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
	Address           string
	SkillDir          string
	Provider          string
	Model             string
	Dimensions        int
	OpenRouterAPIKey  string
	OpenRouterBaseURL string
	HTTPClient        *http.Client
	Logger            *slog.Logger
}

type server struct {
	embeddingv1.UnimplementedEmbeddingServiceServer
	embedder embedder
}

const defaultDimensions = 32
const defaultOpenRouterBaseURL = "https://openrouter.ai/api/v1"

func Run(ctx context.Context, cfg Config) error {
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1:7304"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	embedder, err := newEmbedder(cfg)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(servicekit.UnaryLoggingInterceptor(cfg.Logger)))
	embeddingv1.RegisterEmbeddingServiceServer(grpcServer, &server{embedder: embedder})

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
			{Service: embeddingv1.EmbeddingService_ServiceDesc.ServiceName, Method: "Embed", Request: "quark.embedding.v1.EmbedRequest", Response: "quark.embedding.v1.EmbedResponse", Description: embedder.Description()},
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
		cfg.Logger.Info("embedding service listening", "addr", cfg.Address, "provider", embedder.Provider(), "model", embedder.Model(), "dimensions", embedder.Dimensions())
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
	model := strings.TrimSpace(req.GetModel())
	dimensions := int(req.GetDimensions())
	hash := sha256.Sum256([]byte(req.GetInput()))
	result, err := s.embedder.Embed(ctx, req.GetInput(), model, dimensions)
	if err != nil {
		return nil, err
	}
	return &embeddingv1.EmbedResponse{
		Vector:      result.Vector,
		Model:       result.Model,
		Dimensions:  int32(len(result.Vector)),
		Provider:    result.Provider,
		ContentHash: hex.EncodeToString(hash[:]),
	}, nil
}

type embedder interface {
	Embed(ctx context.Context, input, model string, dimensions int) (embeddingResult, error)
	Provider() string
	Model() string
	Dimensions() int
	Description() string
}

type embeddingResult struct {
	Vector   []float32
	Model    string
	Provider string
}

func newEmbedder(cfg Config) (embedder, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "local"
	}
	switch provider {
	case "local":
		dimensions := cfg.Dimensions
		if dimensions <= 0 {
			dimensions = defaultDimensions
		}
		model := strings.TrimSpace(cfg.Model)
		if model == "" {
			model = "local-hash-v1"
		}
		return localEmbedder{model: model, dimensions: dimensions}, nil
	case "openrouter":
		if strings.TrimSpace(cfg.OpenRouterAPIKey) == "" {
			return nil, fmt.Errorf("openrouter embedding provider requires an API key")
		}
		model := strings.TrimSpace(cfg.Model)
		if model == "" {
			return nil, fmt.Errorf("openrouter embedding provider requires a model")
		}
		baseURL := strings.TrimRight(strings.TrimSpace(cfg.OpenRouterBaseURL), "/")
		if baseURL == "" {
			baseURL = defaultOpenRouterBaseURL
		}
		httpClient := cfg.HTTPClient
		if httpClient == nil {
			httpClient = &http.Client{}
		}
		return &openRouterEmbedder{
			apiKey:     cfg.OpenRouterAPIKey,
			baseURL:    baseURL,
			model:      model,
			dimensions: cfg.Dimensions,
			httpClient: httpClient,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider %q", cfg.Provider)
	}
}

type localEmbedder struct {
	model      string
	dimensions int
}

func (e localEmbedder) Embed(ctx context.Context, input, model string, dimensions int) (embeddingResult, error) {
	_ = ctx
	if strings.TrimSpace(model) == "" {
		model = e.model
	}
	if dimensions <= 0 {
		dimensions = e.dimensions
	}
	return embeddingResult{
		Vector:   deterministicVector(input, dimensions),
		Model:    model,
		Provider: e.Provider(),
	}, nil
}

func (e localEmbedder) Provider() string { return "local" }
func (e localEmbedder) Model() string    { return e.model }
func (e localEmbedder) Dimensions() int  { return e.dimensions }
func (e localEmbedder) Description() string {
	return "Create a deterministic local embedding vector for text."
}

type openRouterEmbedder struct {
	apiKey     string
	baseURL    string
	model      string
	dimensions int
	httpClient *http.Client
}

func (e *openRouterEmbedder) Embed(ctx context.Context, input, model string, dimensions int) (embeddingResult, error) {
	if strings.TrimSpace(model) == "" {
		model = e.model
	}
	reqBody := openRouterEmbeddingRequest{
		Model: model,
		Input: []openRouterEmbeddingInput{{
			Content: []openRouterEmbeddingContent{{
				Type: "text",
				Text: input,
			}},
		}},
		EncodingFormat: "float",
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return embeddingResult{}, fmt.Errorf("marshal openrouter embedding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(data))
	if err != nil {
		return embeddingResult{}, fmt.Errorf("create openrouter embedding request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return embeddingResult{}, fmt.Errorf("openrouter embedding request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return embeddingResult{}, fmt.Errorf("read openrouter embedding response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return embeddingResult{}, fmt.Errorf("openrouter embedding returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out openRouterEmbeddingResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return embeddingResult{}, fmt.Errorf("decode openrouter embedding response: %w", err)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return embeddingResult{}, fmt.Errorf("openrouter embedding response did not include an embedding")
	}
	if dimensions > 0 && dimensions != len(out.Data[0].Embedding) {
		return embeddingResult{}, fmt.Errorf("openrouter embedding dimensions mismatch: requested %d got %d", dimensions, len(out.Data[0].Embedding))
	}
	if e.dimensions > 0 && e.dimensions != len(out.Data[0].Embedding) {
		return embeddingResult{}, fmt.Errorf("openrouter embedding dimensions mismatch: configured %d got %d", e.dimensions, len(out.Data[0].Embedding))
	}
	responseModel := strings.TrimSpace(out.Model)
	if responseModel == "" {
		responseModel = model
	}
	return embeddingResult{
		Vector:   cloneVector(out.Data[0].Embedding),
		Model:    responseModel,
		Provider: e.Provider(),
	}, nil
}

func (e *openRouterEmbedder) Provider() string { return "openrouter" }
func (e *openRouterEmbedder) Model() string    { return e.model }
func (e *openRouterEmbedder) Dimensions() int  { return e.dimensions }
func (e *openRouterEmbedder) Description() string {
	return "Create an OpenRouter provider-backed embedding vector for text."
}

type openRouterEmbeddingRequest struct {
	Model          string                     `json:"model"`
	Input          []openRouterEmbeddingInput `json:"input"`
	EncodingFormat string                     `json:"encoding_format"`
}

type openRouterEmbeddingInput struct {
	Content []openRouterEmbeddingContent `json:"content"`
}

type openRouterEmbeddingContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openRouterEmbeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
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

func cloneVector(in []float32) []float32 {
	out := make([]float32, len(in))
	copy(out, in)
	return out
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
