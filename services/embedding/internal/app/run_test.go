package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalEmbedderUsesDeterministicMetadata(t *testing.T) {
	emb, err := newEmbedder(Config{Provider: "local"})
	if err != nil {
		t.Fatalf("new embedder: %v", err)
	}
	result, err := emb.Embed(context.Background(), "hello world", "", 0)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if result.Provider != "local" || result.Model != "local-hash-v1" || len(result.Vector) != defaultDimensions {
		t.Fatalf("unexpected local result: provider=%s model=%s dimensions=%d", result.Provider, result.Model, len(result.Vector))
	}
}

func TestOpenRouterEmbedderCallsEmbeddingsEndpoint(t *testing.T) {
	var sawAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("path = %s, want /embeddings", r.URL.Path)
		}
		sawAuth = r.Header.Get("Authorization") == "Bearer test-key"
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model": "private/openrouter/test-model",
			"data": [{"embedding": [0.1, 0.2, 0.3]}]
		}`))
	}))
	defer srv.Close()

	emb, err := newEmbedder(Config{
		Provider:          "openrouter",
		Model:             "test-model",
		Dimensions:        3,
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: srv.URL,
		HTTPClient:        srv.Client(),
	})
	if err != nil {
		t.Fatalf("new embedder: %v", err)
	}
	result, err := emb.Embed(context.Background(), "hello", "", 0)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if !sawAuth {
		t.Fatal("OpenRouter authorization header was not sent")
	}
	if result.Provider != "openrouter" || result.Model != "private/openrouter/test-model" || len(result.Vector) != 3 {
		t.Fatalf("unexpected OpenRouter result: %+v", result)
	}
}

func TestOpenRouterEmbedderRejectsDimensionMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"embedding": [0.1, 0.2]}]}`))
	}))
	defer srv.Close()

	emb, err := newEmbedder(Config{
		Provider:          "openrouter",
		Model:             "test-model",
		Dimensions:        3,
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: srv.URL,
		HTTPClient:        srv.Client(),
	})
	if err != nil {
		t.Fatalf("new embedder: %v", err)
	}
	if _, err := emb.Embed(context.Background(), "hello", "", 0); err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}
