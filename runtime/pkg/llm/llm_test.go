package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/quarkloop/pkg/plugin"
)

type fakeProvider struct {
	stream func(context.Context, *plugin.ChatRequest) (<-chan plugin.StreamEvent, error)
	parse  func(string) ([]plugin.ToolCall, string)
}

func (p fakeProvider) ChatCompletionStream(ctx context.Context, req *plugin.ChatRequest) (<-chan plugin.StreamEvent, error) {
	return p.stream(ctx, req)
}

func (p fakeProvider) ParseToolCalls(content string) ([]plugin.ToolCall, string) {
	if p.parse == nil {
		return nil, content
	}
	return p.parse(content)
}

func TestInferStopsEndlessToolLoop(t *testing.T) {
	provider := fakeProvider{
		stream: func(context.Context, *plugin.ChatRequest) (<-chan plugin.StreamEvent, error) {
			ch := make(chan plugin.StreamEvent, 2)
			ch <- plugin.StreamEvent{
				ToolCalls: []plugin.ToolCall{{
					Index: 0,
					ID:    "call-1",
					Type:  "function",
					Function: plugin.ToolCallFunction{
						Name:      "looping_tool",
						Arguments: `{}`,
					},
				}},
			}
			ch <- plugin.StreamEvent{Done: true}
			close(ch)
			return ch, nil
		},
	}
	client := NewClientWithLimits(provider, "test-model", 0, InferenceLimits{MaxTurns: 3, MaxFinalGuardRetries: 2})

	_, err := client.Infer(
		context.Background(),
		[]plugin.Message{{Role: "user", Content: "start"}},
		[]plugin.ToolSchema{{Name: "looping_tool"}},
		func(context.Context, string, string) (string, error) { return "{}", nil },
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected endless tool loop to fail")
	}
	if !strings.Contains(err.Error(), "exceeded 3 model turns") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInferStopsEndlessFinalGuardLoop(t *testing.T) {
	provider := fakeProvider{
		stream: func(context.Context, *plugin.ChatRequest) (<-chan plugin.StreamEvent, error) {
			ch := make(chan plugin.StreamEvent, 2)
			ch <- plugin.StreamEvent{Delta: "not done"}
			ch <- plugin.StreamEvent{Done: true}
			close(ch)
			return ch, nil
		},
	}
	client := NewClientWithLimits(provider, "test-model", 0, InferenceLimits{MaxTurns: 10, MaxFinalGuardRetries: 2})

	_, err := client.Infer(
		context.Background(),
		[]plugin.Message{{Role: "user", Content: "start"}},
		nil,
		nil,
		nil,
		func(string) (string, bool) { return "try again", true },
	)
	if err == nil {
		t.Fatal("expected endless finalization guard loop to fail")
	}
	if !strings.Contains(err.Error(), "finalization guard exceeded 2 retries") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInferPropagatesProviderError(t *testing.T) {
	want := errors.New("provider down")
	provider := fakeProvider{
		stream: func(context.Context, *plugin.ChatRequest) (<-chan plugin.StreamEvent, error) {
			return nil, want
		},
	}
	client := NewClientWithLimits(provider, "test-model", 0, InferenceLimits{MaxTurns: 3, MaxFinalGuardRetries: 2})

	_, err := client.Infer(context.Background(), nil, nil, nil, nil, nil)
	if !errors.Is(err, want) {
		t.Fatalf("expected provider error %v, got %v", want, err)
	}
}

func TestInferStreamsTraceableToolEvents(t *testing.T) {
	provider := fakeProvider{
		stream: func(context.Context, *plugin.ChatRequest) (<-chan plugin.StreamEvent, error) {
			ch := make(chan plugin.StreamEvent, 2)
			ch <- plugin.StreamEvent{
				ToolCalls: []plugin.ToolCall{{
					Index: 0,
					ID:    "call-1",
					Type:  "function",
					Function: plugin.ToolCallFunction{
						Name:      "indexer_IndexDocument",
						Arguments: `{"chunkId":"chunk-1"}`,
					},
				}},
			}
			ch <- plugin.StreamEvent{Done: true}
			close(ch)
			return ch, nil
		},
	}
	client := NewClientWithLimits(provider, "test-model", 0, InferenceLimits{MaxTurns: 1, MaxFinalGuardRetries: 1})

	var events []map[string]any
	_, err := client.Infer(
		context.Background(),
		[]plugin.Message{{Role: "user", Content: "index"}},
		[]plugin.ToolSchema{{Name: "indexer_IndexDocument"}},
		func(context.Context, string, string) (string, error) { return "", fmt.Errorf("write failed") },
		func(kind string, data any) {
			payload, ok := data.(map[string]any)
			if !ok {
				t.Fatalf("event %s payload type = %T", kind, data)
			}
			payload["kind"] = kind
			events = append(events, payload)
		},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "exceeded 1 model turns") {
		t.Fatalf("expected bounded loop error after tool result, got %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want start and result", events)
	}
	if events[0]["kind"] != "tool_start" || events[0]["id"] != "call-1" || events[0]["name"] != "indexer_IndexDocument" {
		t.Fatalf("tool start event not traceable: %+v", events[0])
	}
	if events[1]["kind"] != "tool_result" || events[1]["id"] != "call-1" || events[1]["error"] != true {
		t.Fatalf("tool result event not traceable: %+v", events[1])
	}
}
