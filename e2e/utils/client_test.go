//go:build e2e

package utils

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestReadMessageTraceParsesTokensAndTools(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		"event: token",
		`data: "hello"`,
		"",
		"event: tool_start",
		`data: {"name":"embedding_Embed","arguments":"{}"}`,
		"",
		"event: tool_result",
		`data: {"name":"embedding_Embed","result":"{\"embeddingRef\":\"ref\"}"}`,
		"",
	}, "\n"))

	trace, err := readMessageTrace(context.Background(), stream, MessageTraceOptions{
		Label:       "unit trace",
		IdleTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	if !trace.Completed {
		t.Fatal("trace did not complete")
	}
	if trace.Text != "hello" {
		t.Fatalf("text = %q, want hello", trace.Text)
	}
	if len(trace.ToolStarts) != 1 || trace.ToolStarts[0] != "embedding_Embed" {
		t.Fatalf("tool starts = %v", trace.ToolStarts)
	}
	if len(trace.ToolResults) != 1 || trace.ToolResults[0] != "embedding_Embed" {
		t.Fatalf("tool results = %v", trace.ToolResults)
	}
}

func TestReadMessageTraceIdleTimeoutIncludesDiagnostics(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()

	_, err := readMessageTrace(context.Background(), reader, MessageTraceOptions{
		Label:       "idle unit trace",
		Prompt:      "index these PDFs",
		IdleTimeout: 20 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected idle timeout")
	}
	msg := err.Error()
	for _, want := range []string{"idle timeout", "idle unit trace", "index these PDFs"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
}
