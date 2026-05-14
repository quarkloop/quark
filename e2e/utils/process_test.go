//go:build e2e

package utils

import (
	"strings"
	"testing"
)

func TestFormatProcessLogLineAddsE2EPrefix(t *testing.T) {
	got := formatProcessLogLine("TestFormat", "indexer", `time=now level=INFO msg="ready"`)
	want := `[e2e][test=TestFormat][process=indexer] time=now level=INFO msg="ready"`
	if got != want {
		t.Fatalf("formatProcessLogLine() = %q, want %q", got, want)
	}
}

func TestFormatProcessLogLineRelabelsRuntimeChild(t *testing.T) {
	got := formatProcessLogLine("TestFormat", "supervisor", `time=now level=INFO msg="tool calls" process=runtime`)
	for _, want := range []string{"[e2e]", "[test=TestFormat]", "[process=runtime]", "[parent_process=supervisor]", "tool calls"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted line %q missing %q", got, want)
		}
	}
}
