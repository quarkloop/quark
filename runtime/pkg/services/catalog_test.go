package services

import (
	"strings"
	"testing"

	servicev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/service/v1"
)

func TestParseEndpoints(t *testing.T) {
	t.Parallel()

	got := ParseEndpoints("indexer=127.0.0.1:7301, 127.0.0.1:7302;space=127.0.0.1:7303")
	if len(got) != 3 {
		t.Fatalf("endpoints = %d, want 3", len(got))
	}
	if got[0].Name != "indexer" || got[0].Address != "127.0.0.1:7301" {
		t.Fatalf("first endpoint = %+v", got[0])
	}
	if got[1].Name != "" || got[1].Address != "127.0.0.1:7302" {
		t.Fatalf("second endpoint = %+v", got[1])
	}
}

func TestEndpointsFromEnvCanBeDisabled(t *testing.T) {
	t.Setenv(EnvDisableDiscovery, "true")
	t.Setenv(EnvIndexerAddr, "127.0.0.1:7301")
	if got := EndpointsFromEnv(); len(got) != 0 {
		t.Fatalf("got %v, want no endpoints", got)
	}
}

func TestPromptBlockIncludesServiceSkillsAndRPCs(t *testing.T) {
	t.Parallel()

	block := PromptBlock([]*servicev1.ServiceDescriptor{{
		Name:    "indexer",
		Type:    "indexer",
		Version: "1.0.0",
		Address: "127.0.0.1:7301",
		Rpcs: []*servicev1.RpcDescriptor{{
			Service:  "quark.indexer.v1.IndexerService",
			Method:   "GetContext",
			Request:  "quark.indexer.v1.QueryRequest",
			Response: "quark.indexer.v1.ContextResponse",
		}},
		Skills: []*servicev1.SkillDescriptor{{
			Name:     "service-indexer",
			Markdown: "# service-indexer\n\nUse query vectors.",
		}},
	}})

	for _, want := range []string{"Available gRPC Services", "indexer", "GetContext", "service-indexer", "Use query vectors."} {
		if !strings.Contains(block, want) {
			t.Fatalf("prompt block missing %q:\n%s", want, block)
		}
	}
}

func TestExecutorExposesGenericGRPCTool(t *testing.T) {
	t.Parallel()

	executor := NewExecutor([]*servicev1.ServiceDescriptor{{
		Name:    "indexer",
		Address: "127.0.0.1:7301",
		Rpcs: []*servicev1.RpcDescriptor{{
			Service:  "quark.indexer.v1.IndexerService",
			Method:   "GetContext",
			Request:  "quark.indexer.v1.QueryRequest",
			Response: "quark.indexer.v1.ContextResponse",
		}},
	}})
	tools := executor.ToolSchemas()
	if len(tools) != 1 || tools[0].Name != ToolName {
		t.Fatalf("tools = %+v", tools)
	}
	if _, _, err := executor.resolve("indexer", "GetContext"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
}
