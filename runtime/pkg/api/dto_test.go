package api

import (
	"testing"

	"github.com/quarkloop/runtime/pkg/message"
)

func TestMapMessageResponsesCopiesBoundaryShape(t *testing.T) {
	in := []message.Message{{
		ID:        "m1",
		Role:      "assistant",
		Content:   "hello",
		Timestamp: "2026-05-14T00:00:00Z",
	}}

	out := mapMessageResponses(in)
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].ID != "m1" || out[0].Role != "assistant" || out[0].Content != "hello" || out[0].Timestamp == "" {
		t.Fatalf("unexpected mapped message: %+v", out[0])
	}

	in[0].Content = "mutated"
	if out[0].Content != "hello" {
		t.Fatalf("response reused input backing data: %+v", out[0])
	}
}
