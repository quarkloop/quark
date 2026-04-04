// Package execution isolates tool invocation and plan step execution.
// Single responsibility: run tools, execute multi-step LLM+tool loops.
package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	msg "github.com/quarkloop/agent/pkg/context/message"
	"github.com/quarkloop/agent/pkg/hooks"
	"github.com/quarkloop/agent/pkg/model"
	"github.com/quarkloop/agent/pkg/tool"
)

// InvokeToolWithHooks wraps InvokeTool with hook interception.
// BeforeToolCall hooks can block or modify the call.
// AfterToolCall hooks can observe or redact the result.
func InvokeToolWithHooks(
	ctx context.Context,
	reg *hooks.Registry,
	disp tool.Invoker,
	stepID string,
	call msg.ToolCallPayload,
	sessionID string,
) msg.ToolResultPayload {
	// BeforeToolCall hooks.
	hookPayload := &hooks.ToolCallPayload{
		ToolName:  call.ToolName,
		Arguments: rawToMap(call.Arguments),
		StepID:    stepID,
		SessionID: sessionID,
	}
	modified, decision, _ := reg.Execute(ctx, hooks.BeforeToolCall, hookPayload)
	if decision == hooks.Block || decision == hooks.Halt || decision == hooks.Collapse {
		reason := "blocked by hook"
		if modified != nil {
			if s, ok := modified.(string); ok {
				reason = s
			}
		}
		return msg.ToolResultPayload{
			ToolCallID:   call.ToolCallID,
			ToolName:     call.ToolName,
			IsError:      true,
			ErrorMessage: reason,
		}
	}
	if decision == hooks.Shape {
		if tp, ok := modified.(*hooks.ToolCallPayload); ok {
			call.ToolName = tp.ToolName
			if raw, err := json.Marshal(tp.Arguments); err == nil {
				call.Arguments = raw
			}
		}
	}

	result := InvokeTool(ctx, disp, stepID, call)

	// AfterToolCall hooks.
	resultPayload := &hooks.ToolResultPayload{
		ToolName: result.ToolName,
		Content:  result.Content,
		IsError:  result.IsError,
		StepID:   stepID,
	}
	reg.Execute(ctx, hooks.AfterToolCall, resultPayload)
	result.Content = resultPayload.Content
	result.IsError = resultPayload.IsError

	return result
}

// InvokeTool calls a tool via the dispatcher and returns the result payload.
func InvokeTool(
	ctx context.Context,
	disp tool.Invoker,
	stepID string,
	call msg.ToolCallPayload,
) msg.ToolResultPayload {
	var input map[string]any
	if err := json.Unmarshal(call.Arguments, &input); err != nil {
		log.Printf("subagent[%s]: failed to parse tool args: %v", stepID, err)
		return msg.ToolResultPayload{
			ToolCallID:   call.ToolCallID,
			ToolName:     call.ToolName,
			IsError:      true,
			ErrorMessage: fmt.Sprintf("invalid arguments: %v", err),
		}
	}

	// Normalise parameter aliases (e.g. "command" → "cmd" for bash).
	input = model.NormalizeArgs(call.ToolName, input)

	out, err := disp.Invoke(ctx, call.ToolName, input)
	if err != nil {
		return msg.ToolResultPayload{
			ToolCallID:   call.ToolCallID,
			ToolName:     call.ToolName,
			IsError:      true,
			ErrorMessage: err.Error(),
		}
	}

	outData, _ := json.Marshal(out)

	result := msg.ToolResultPayload{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Content:    string(outData),
	}

	if toolErr, ok := toolErrorMessage(out); ok {
		result.IsError = true
		result.ErrorMessage = toolErr
	}
	if exitCode, ok := exitCodeFromOutput(out); ok && exitCode != 0 {
		result.IsError = true
		result.ErrorMessage = toolExitErrorMessage(call.ToolName, exitCode, out)
	}

	return result
}

// ModelToolCallToPayload converts a model.ToolCall (from text parsing) to the
// context message type.
func ModelToolCallToPayload(tc *model.ToolCall) msg.ToolCallPayload {
	return msg.ToolCallPayload{
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		Arguments:  tc.Arguments,
	}
}

// NativeToolCallToPayload converts a model.NativeToolCall (from structured API
// response) to the context message type.
func NativeToolCallToPayload(tc model.NativeToolCall) msg.ToolCallPayload {
	return msg.ToolCallPayload{
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		Arguments:  tc.Arguments,
	}
}

// ─── Tool reminder helpers ──────────────────────────────────────────────────

// BuildRequiredToolReminder produces a format-aware reminder for the initial
// required tool call.
func BuildRequiredToolReminder(parser model.ToolCallParser, toolName string) string {
	hint := parser.FormatHint([]string{toolName})
	if toolName == "" {
		return fmt.Sprintf("Reminder: your first response must be only a tool call. Do not provide prose before the tool call.\n\n%s", hint)
	}
	return fmt.Sprintf("Reminder: your first response must be only a %s tool call. Do not provide prose before the tool call.\n\n%s", toolName, hint)
}

// BuildPendingToolReminder produces a format-aware reminder when a required
// tool has not been called yet before the model tries to give a summary.
func BuildPendingToolReminder(parser model.ToolCallParser, toolName string) string {
	hint := parser.FormatHint([]string{toolName})
	if toolName == "" {
		return fmt.Sprintf("Reminder: the task is not complete yet. You must make the required tool call before any final summary.\n\n%s", hint)
	}
	return fmt.Sprintf("Reminder: the task is not complete yet. You must use the %s tool before any final summary. Respond only with the tool call.\n\n%s", toolName, hint)
}

// ─── Tool requirement parsing ───────────────────────────────────────────────

// RequiredInitialToolCall checks if a step description mandates a specific
// tool to be called first.
func RequiredInitialToolCall(description string) (string, bool) {
	re := regexp.MustCompile(`(?i)first execution response must be a\s+([a-z0-9_-]+)\s+(?:tool|skill)\s+call`)
	matches := re.FindStringSubmatch(description)
	if len(matches) != 2 {
		return "", false
	}
	return strings.ToLower(matches[1]), true
}

// RequiredToolCallsBeforeSummary returns tool names that must be called before
// the model can give a summary response.
func RequiredToolCallsBeforeSummary(description string) []string {
	var out []string
	seen := map[string]bool{}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)must use the\s+([a-z0-9_-]+)\s+(?:tool|skill)[^.\n]*before[^.\n]*summary`),
		regexp.MustCompile(`(?i)the\s+([a-z0-9_-]+)\s+(?:tool|skill)\s+must[^.\n]*before[^.\n]*summary`),
	}
	for _, re := range patterns {
		for _, match := range re.FindAllStringSubmatch(description, -1) {
			if len(match) != 2 {
				continue
			}
			toolName := strings.ToLower(match[1])
			if toolName == "" || seen[toolName] {
				continue
			}
			seen[toolName] = true
			out = append(out, toolName)
		}
	}
	return out
}

// MissingRequiredToolCalls returns tools from required that haven't been called.
func MissingRequiredToolCalls(required []string, successful map[string]bool) []string {
	if len(required) == 0 {
		return nil
	}
	var missing []string
	for _, toolName := range required {
		if !successful[toolName] {
			missing = append(missing, toolName)
		}
	}
	return missing
}

// ─── Activity data builders ─────────────────────────────────────────────────

// BuildToolCalledActivityData creates activity data for a tool.called event.
func BuildToolCalledActivityData(stepID string, call msg.ToolCallPayload) map[string]string {
	data := map[string]string{
		"step": stepID,
		"tool": call.ToolName,
		"args": SanitizeActivityText(string(call.Arguments), 1024),
	}

	var input map[string]any
	if err := json.Unmarshal(call.Arguments, &input); err == nil {
		if cmd, ok := input["cmd"].(string); ok {
			data["cmd"] = SanitizeActivityText(cmd, 512)
		}
		if path, ok := input["path"].(string); ok {
			data["path"] = SanitizeActivityText(path, 512)
		}
		if operation, ok := input["operation"].(string); ok {
			data["operation"] = SanitizeActivityText(operation, 128)
		}
		if startLine, ok := intValue(input["start_line"]); ok {
			data["start_line"] = strconv.Itoa(startLine)
		}
		if endLine, ok := intValue(input["end_line"]); ok {
			data["end_line"] = strconv.Itoa(endLine)
		}
	}

	return data
}

// BuildToolCompletedActivityData creates activity data for a tool.completed event.
func BuildToolCompletedActivityData(stepID string, result msg.ToolResultPayload) map[string]string {
	data := map[string]string{
		"step":     stepID,
		"tool":     result.ToolName,
		"error":    SanitizeActivityText(result.ErrorMessage, 512),
		"result":   SanitizeActivityText(result.Content, 1024),
		"is_error": fmt.Sprintf("%t", result.IsError),
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result.Content), &out); err == nil {
		if exitCode, ok := exitCodeFromOutput(out); ok {
			data["exit_code"] = strconv.Itoa(exitCode)
		}
		if output, ok := out["output"].(string); ok {
			data["output"] = SanitizeActivityText(output, 512)
		}
		if preview, ok := out["content_preview"].(string); ok {
			data["content_preview"] = SanitizeActivityText(preview, 512)
		}
		if content, ok := out["content"].(string); ok {
			data["content"] = SanitizeActivityText(content, 512)
		}
		if path, ok := out["path"].(string); ok {
			data["path"] = SanitizeActivityText(path, 512)
		}
		if operation, ok := out["operation"].(string); ok {
			data["operation"] = SanitizeActivityText(operation, 128)
		}
		if bytesWritten, ok := intValue(out["bytes_written"]); ok {
			data["bytes_written"] = strconv.Itoa(bytesWritten)
		}
		if replacements, ok := intValue(out["replacements"]); ok {
			data["replacements"] = strconv.Itoa(replacements)
		}
		if editsApplied, ok := intValue(out["edits_applied"]); ok {
			data["edits_applied"] = strconv.Itoa(editsApplied)
		}
		if bytesRead, ok := intValue(out["bytes_read"]); ok {
			data["bytes_read"] = strconv.Itoa(bytesRead)
		}
		if totalLines, ok := intValue(out["total_lines"]); ok {
			data["total_lines"] = strconv.Itoa(totalLines)
		}
		if startLine, ok := intValue(out["start_line"]); ok {
			data["start_line"] = strconv.Itoa(startLine)
		}
		if endLine, ok := intValue(out["end_line"]); ok {
			data["end_line"] = strconv.Itoa(endLine)
		}
	}

	return data
}

// ─── Utility ────────────────────────────────────────────────────────────────

var activitySecretPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:API_KEY|TOKEN|SECRET|PASSWORD))=([^\s"'` + "`" + `]+)`), `$1=[REDACTED]`},
	{regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[^\s"'` + "`" + `]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]+\b`), `[REDACTED]`},
}

// SanitizeActivityText redacts secrets and truncates.
func SanitizeActivityText(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	for _, p := range activitySecretPatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	if maxLen > 0 && len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func exitCodeFromOutput(out map[string]any) (int, bool) {
	v, ok := out["exit_code"]
	if !ok {
		return 0, false
	}
	return intValue(v)
}

func intValue(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return int(i), true
		}
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

func toolErrorMessage(out map[string]any) (string, bool) {
	if isError, ok := out["is_error"].(bool); ok && isError {
		if msg, ok := out["error"].(string); ok {
			msg = SanitizeActivityText(msg, 256)
			if msg != "" {
				return msg, true
			}
		}
		return "tool returned an error", true
	}
	if msg, ok := out["error"].(string); ok {
		msg = SanitizeActivityText(msg, 256)
		if msg != "" {
			return msg, true
		}
	}
	return "", false
}

func toolExitErrorMessage(toolName string, exitCode int, out map[string]any) string {
	msg := fmt.Sprintf("%s exit code %d", toolName, exitCode)
	if output, ok := out["output"].(string); ok {
		output = SanitizeActivityText(output, 256)
		if output != "" {
			msg += ": " + output
		}
	}
	return msg
}

// Truncate shortens a string for logging.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func rawToMap(raw json.RawMessage) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}
