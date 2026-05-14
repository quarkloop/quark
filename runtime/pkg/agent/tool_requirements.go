package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/quarkloop/runtime/pkg/llm"
)

var toolRequirementPattern = regexp.MustCompile(`(?i)until\s+there\s+are\s+(\d+)\s+successful\s+([A-Za-z0-9_]+)\s+results?`)

type toolRequirementTracker struct {
	required map[string]int
	observed map[string]int
}

func newToolRequirementTracker(prompt string) *toolRequirementTracker {
	matches := toolRequirementPattern.FindAllStringSubmatch(prompt, -1)
	if len(matches) == 0 {
		return nil
	}
	tracker := &toolRequirementTracker{
		required: make(map[string]int),
		observed: make(map[string]int),
	}
	for _, match := range matches {
		count := parsePositiveCount(match[1])
		tool := strings.TrimSpace(match[2])
		if count <= 0 || tool == "" {
			continue
		}
		if tracker.required[tool] < count {
			tracker.required[tool] = count
		}
	}
	if len(tracker.required) == 0 {
		return nil
	}
	return tracker
}

func parsePositiveCount(value string) int {
	var count int
	if _, err := fmt.Sscanf(value, "%d", &count); err != nil || count <= 0 {
		return 0
	}
	return count
}

func (t *toolRequirementTracker) wrapToolHandler(next func(context.Context, string, string) (string, error)) func(context.Context, string, string) (string, error) {
	if t == nil {
		return next
	}
	return func(ctx context.Context, name, arguments string) (string, error) {
		result, err := next(ctx, name, arguments)
		t.record(name, result, err)
		return result, err
	}
}

func (t *toolRequirementTracker) record(name, result string, err error) {
	if t == nil || !toolResultSucceeded(result, err) {
		return
	}
	if _, ok := t.required[name]; !ok {
		return
	}
	t.observed[name]++
}

func toolResultSucceeded(result string, err error) bool {
	if err != nil {
		return false
	}
	trimmed := strings.TrimSpace(result)
	if trimmed == "" || strings.HasPrefix(trimmed, "error:") {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		if success, ok := payload["success"].(bool); ok {
			return success
		}
	}
	return true
}

func (t *toolRequirementTracker) finalGuard(content string) (string, bool) {
	if t == nil {
		return "", false
	}
	missing := t.missing()
	if len(missing) == 0 {
		return "", false
	}
	return "Runtime validation blocked finalization. " + strings.Join(missing, " ") + " Continue using the existing tool context and do not produce a final answer until these tool completion requirements are satisfied.", true
}

func (t *toolRequirementTracker) missing() []string {
	missing := make([]string, 0)
	for tool, required := range t.required {
		observed := t.observed[tool]
		if observed >= required {
			continue
		}
		missing = append(missing, fmt.Sprintf("%s has %d successful result(s), but %d are required.", tool, observed, required))
	}
	return missing
}

func combineFinalGuards(guards ...llm.FinalGuard) llm.FinalGuard {
	active := make([]llm.FinalGuard, 0, len(guards))
	for _, guard := range guards {
		if guard != nil {
			active = append(active, guard)
		}
	}
	if len(active) == 0 {
		return nil
	}
	return func(content string) (string, bool) {
		for _, guard := range active {
			if instruction, retry := guard(content); retry {
				return instruction, true
			}
		}
		return "", false
	}
}
