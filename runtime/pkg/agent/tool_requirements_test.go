package agent

import (
	"context"
	"strings"
	"testing"
)

func TestToolRequirementTrackerBlocksUntilDeclaredSuccessfulResults(t *testing.T) {
	tracker := newToolRequirementTracker("Do not send a final answer until there are 2 successful indexer_IndexDocument results.")
	if tracker == nil {
		t.Fatal("tracker was not created")
	}

	if instruction, retry := tracker.finalGuard(""); !retry || !strings.Contains(instruction, "0 successful") {
		t.Fatalf("initial guard = %q retry=%t", instruction, retry)
	}

	tracker.record("indexer_IndexDocument", `{"success": true}`, nil)
	if instruction, retry := tracker.finalGuard(""); !retry || !strings.Contains(instruction, "1 successful") {
		t.Fatalf("guard after one success = %q retry=%t", instruction, retry)
	}

	tracker.record("indexer_IndexDocument", `{"success": true}`, nil)
	if instruction, retry := tracker.finalGuard(""); retry || instruction != "" {
		t.Fatalf("guard after completion = %q retry=%t", instruction, retry)
	}
}

func TestToolRequirementTrackerIgnoresFailedResults(t *testing.T) {
	tracker := newToolRequirementTracker("until there are 1 successful indexer_IndexDocument result")
	if tracker == nil {
		t.Fatal("tracker was not created")
	}

	tracker.record("indexer_IndexDocument", `{"success": false}`, nil)
	tracker.record("indexer_IndexDocument", "error: write failed", nil)
	tracker.record("indexer_IndexDocument", "", context.Canceled)

	if instruction, retry := tracker.finalGuard(""); !retry || !strings.Contains(instruction, "0 successful") {
		t.Fatalf("guard after failures = %q retry=%t", instruction, retry)
	}
}

func TestCombineFinalGuardsUsesFirstRetryInstruction(t *testing.T) {
	guard := combineFinalGuards(
		func(string) (string, bool) { return "first", true },
		func(string) (string, bool) { return "second", true },
	)

	instruction, retry := guard("")
	if !retry || instruction != "first" {
		t.Fatalf("combined guard = %q retry=%t", instruction, retry)
	}
}
