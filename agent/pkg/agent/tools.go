package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/quarkloop/agent/pkg/activity"
	llmctx "github.com/quarkloop/agent/pkg/context"
	msg "github.com/quarkloop/agent/pkg/context/message"
	"github.com/quarkloop/agent/pkg/plan"
)

// ─── WORKER ──────────────────────────────────────────────────────────────────

// runWorker drives one worker agent through its EXECUTE → REPORT loop.
func (a *Agent) runWorker(ctx context.Context, step plan.Step) {
	log.Printf("worker[%s]: starting (%s)", step.ID, step.Agent)

	result, err := a.executeStep(ctx, step)

	event := map[string]string{
		"step_id": step.ID,
		"status":  "complete",
		"result":  result,
	}
	if err != nil {
		log.Printf("worker[%s]: failed: %v", step.ID, err)
		event["status"] = "failed"
		event["error"] = err.Error()
	} else {
		log.Printf("worker[%s]: complete", step.ID)
	}

	data, _ := json.Marshal(event)
	if err := a.kb.Set(NSEvents, step.ID+"-done", data); err != nil {
		log.Printf("worker[%s]: failed to write event: %v", step.ID, err)
	}
}

// executeStep runs a single step using a short-lived AgentContext.
// Supports multi-turn tool loops with linked ToolCall/ToolResult pairs.
func (a *Agent) executeStep(ctx context.Context, step plan.Step) (string, error) {
	def, ok := a.subAgents[step.Agent]
	if !ok && step.Agent != a.def.Name {
		return "", fmt.Errorf("unknown agent %q", step.Agent)
	}
	if step.Agent == a.def.Name {
		def = a.def
	}

	systemPrompt := resolveSystemPrompt(a, def, step.Agent)

	workerCtx, err := a.buildWorkerContext(systemPrompt, def.Config.ContextWindow)
	if err != nil {
		return a.executeStepRaw(ctx, step, systemPrompt)
	}

	artifacts := a.gatherArtifacts()
	userMsg := fmt.Sprintf("Task: %s\n\nContext from knowledge base:\n%s\n\nComplete this task and provide a detailed result.",
		step.Description, artifacts)
	requiredToolName, requiresToolFirst := requiredInitialToolCall(step.Description)
	requiredToolsBeforeSummary := requiredToolCallsBeforeSummary(step.Description)
	successfulToolCalls := map[string]bool{}
	initialToolSatisfied := !requiresToolFirst
	log.Printf("worker[%s]: parsed tool requirements first=%q requires_first=%t before_summary=%v", step.ID, requiredToolName, requiresToolFirst, requiredToolsBeforeSummary)
	if len(requiredToolsBeforeSummary) == 0 && strings.Contains(strings.ToLower(step.Description), "before any summary") {
		log.Printf("worker[%s]: no before-summary tool requirement matched step description: %q", step.ID, step.Description)
	}

	var finalResult string
	for iter := 0; iter < MaxToolIterations; iter++ {
		resp, err := a.inferWithContext(ctx, workerCtx, userMsg)
		if err != nil {
			return "", fmt.Errorf("infer iter %d: %w", iter, err)
		}
		log.Printf("worker[%s]: iter %d, %d tokens", step.ID, iter, resp.TotalTokens())

		toolCall := parseToolCall(resp.Content)
		if toolCall == nil {
			if requiresToolFirst && !initialToolSatisfied && iter < MaxToolIterations-1 {
				log.Printf("worker[%s]: prose response before required initial tool %q; retrying", step.ID, requiredToolName)
				userMsg = buildRequiredToolReminder(requiredToolName)
				continue
			}
			if missing := missingRequiredToolCalls(requiredToolsBeforeSummary, successfulToolCalls); len(missing) > 0 && iter < MaxToolIterations-1 {
				log.Printf("worker[%s]: prose response before required tool(s) %v; successful_tools=%v", step.ID, missing, successfulToolCalls)
				userMsg = buildPendingToolReminder(missing[0])
				continue
			}
			log.Printf("worker[%s]: accepting prose response; successful_tools=%v", step.ID, successfulToolCalls)
			finalResult = resp.Content
			break
		}

		a.emit(activity.ToolCalled, buildToolCalledActivityData(step.ID, *toolCall))
		result := a.executeTool(ctx, step.ID, *toolCall)
		a.emit(activity.ToolCompleted, buildToolCompletedActivityData(step.ID, result))
		if !result.IsError {
			successfulToolCalls[strings.ToLower(toolCall.ToolName)] = true
			if requiresToolFirst && strings.EqualFold(toolCall.ToolName, requiredToolName) {
				initialToolSatisfied = true
			}
			log.Printf("worker[%s]: successful tool %q; successful_tools=%v", step.ID, strings.ToLower(toolCall.ToolName), successfulToolCalls)
		} else {
			log.Printf("worker[%s]: tool %q failed: %s", step.ID, strings.ToLower(toolCall.ToolName), result.ErrorMessage)
		}

		callID, _ := a.idGen.Next()
		resultID, _ := a.idGen.Next()
		agentAuthor, _ := llmctx.NewAuthorID(step.Agent)
		toolAuthor, _ := llmctx.NewAuthorID(AuthorToolExecutor)

		tc, tr, err := llmctx.NewLinkedToolExchange(
			callID, agentAuthor, *toolCall,
			resultID, toolAuthor, result,
			a.tc,
		)
		if err != nil {
			log.Printf("worker[%s]: failed to create linked exchange: %v", step.ID, err)
			continue
		}

		tc = tc.WithVisibility(a.visPolicy.For(llmctx.ToolCallMessageType))
		tr = tr.WithVisibility(a.visPolicy.For(llmctx.ToolResultMessageType))

		workerCtx.AppendMessage(ctx, tc)
		workerCtx.AppendMessage(ctx, tr)

		userMsg = "" // next iteration: let LLM process tool results
	}

	if finalResult == "" {
		if requiresToolFirst && !initialToolSatisfied {
			return "", fmt.Errorf("required initial tool %q was not executed", requiredToolName)
		}
		if missing := missingRequiredToolCalls(requiredToolsBeforeSummary, successfulToolCalls); len(missing) > 0 {
			return "", fmt.Errorf("required tool calls not completed before summary: %s", strings.Join(missing, ", "))
		}
		finalResult = "Task completed with required tool interactions."
	}

	if err := a.kb.Set(NSArtifacts, step.ID, []byte(finalResult)); err != nil {
		log.Printf("worker[%s]: failed to write artifact: %v", step.ID, err)
	}

	if report := workerCtx.DetectOrphans(); report.HasOrphans() {
		log.Printf("worker[%s]: %d orphaned messages detected", step.ID, len(report.OrphanIDs))
	}

	return finalResult, nil
}

func truncateActivityResult(s string) string {
	return sanitizeActivityText(s, 1024)
}

// executeStepRaw is the fallback for when worker context build fails.
func (a *Agent) executeStepRaw(ctx context.Context, step plan.Step, systemPrompt string) (string, error) {
	artifacts := a.gatherArtifacts()
	userMsg := fmt.Sprintf("Task: %s\n\nContext from knowledge base:\n%s\n\nComplete this task and provide a detailed result.",
		step.Description, artifacts)

	sysID, _ := a.idGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(step.Agent)
	sysMsg, err := llmctx.NewSystemPromptMessage(sysID, agentAuthor, systemPrompt, a.tc)
	if err != nil {
		return "", err
	}

	window, _ := llmctx.NewContextWindow(DefaultContextWindow)
	ac, err := llmctx.NewAgentContextBuilder().
		WithSystemPrompt(sysMsg).
		WithContextWindow(window).
		WithTokenComputer(a.tc).
		Build()
	if err != nil {
		return "", err
	}

	m, _ := a.newUserMsg(userMsg)
	ac.AppendMessage(ctx, m)

	adapter, _ := a.adapterReg.Get(a.gateway.Provider())
	ca := llmctx.NewContextAdapter(ac, adapter)
	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     a.gateway.ModelName(),
		MaxTokens: a.gateway.MaxTokens(),
	})
	if err != nil {
		return "", err
	}

	resp, err := a.gateway.InferRaw(ctx, payload)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// buildWorkerContext creates a short-lived AgentContext for a worker step.
func (a *Agent) buildWorkerContext(systemPrompt string, windowSize int) (*llmctx.AgentContext, error) {
	if windowSize <= 0 {
		windowSize = DefaultContextWindow
	}
	window, _ := llmctx.NewContextWindow(int32(windowSize))

	sysID, _ := a.idGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(AuthorWorker)
	sysMsg, err := llmctx.NewSystemPromptMessage(sysID, agentAuthor, systemPrompt, a.tc)
	if err != nil {
		return nil, err
	}

	pipeline, err := llmctx.NewPipelineCompactor(
		llmctx.NewWeightBasedCompactor(),
		llmctx.NewFIFOCompactor(),
	)
	if err != nil {
		return nil, err
	}
	compactor, err := llmctx.NewThresholdCompactor(pipeline, DefaultCompactionThreshold)
	if err != nil {
		return nil, err
	}

	return llmctx.NewAgentContextBuilder().
		WithSystemPrompt(sysMsg).
		WithContextWindow(window).
		WithCompactor(compactor).
		WithTokenComputer(a.tc).
		WithIDGenerator(a.idGen).
		Build()
}

// ─── Tool call helpers ───────────────────────────────────────────────────────

// parseToolCall extracts a tool call from the LLM response.
// Prefers ```tool JSON fences and accepts a legacy alternate fence.
func parseToolCall(content string) *msg.ToolCallPayload {
	block := extractFencedToolBlock(content, "tool")
	if block == "" {
		block = extractFencedToolBlock(content, "skill")
	}
	if block == "" {
		return nil
	}
	var call struct {
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal([]byte(block), &call); err != nil {
		return nil
	}
	return &msg.ToolCallPayload{
		ToolCallID: fmt.Sprintf("tc-%d", time.Now().UnixNano()),
		ToolName:   call.Name,
		Arguments:  call.Input,
	}
}

func extractFencedToolBlock(content, fence string) string {
	marker := "```" + fence
	start := strings.Index(content, marker)
	if start < 0 {
		return ""
	}
	end := strings.Index(content[start+len(marker):], "```")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(content[start+len(marker) : start+len(marker)+end])
}

// executeTool dispatches a tool call through the dispatcher and returns
// a structured result payload.
func (a *Agent) executeTool(ctx context.Context, stepID string, call msg.ToolCallPayload) msg.ToolResultPayload {
	var input map[string]interface{}
	if err := json.Unmarshal(call.Arguments, &input); err != nil {
		log.Printf("worker[%s]: failed to parse tool args: %v", stepID, err)
		return msg.ToolResultPayload{
			ToolCallID:   call.ToolCallID,
			ToolName:     call.ToolName,
			IsError:      true,
			ErrorMessage: fmt.Sprintf("invalid arguments: %v", err),
		}
	}

	out, err := a.dispatcher.Invoke(ctx, call.ToolName, input)
	if err != nil {
		return msg.ToolResultPayload{
			ToolCallID:   call.ToolCallID,
			ToolName:     call.ToolName,
			IsError:      true,
			ErrorMessage: err.Error(),
		}
	}

	outData, _ := json.Marshal(out)
	a.kb.Set(NSArtifacts, stepID+"-skill-"+call.ToolName, outData)

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

// resolveSystemPrompt finds the system prompt for an agent, checking KB config,
// the agent definition, and falling back to a default.
func resolveSystemPrompt(a *Agent, def *Definition, agentName string) string {
	if def.SystemPrompt != "" {
		return def.SystemPrompt
	}
	if data, err := a.kb.Get(NSConfig, agentName+"-prompt"); err == nil {
		return string(data)
	}
	return fmt.Sprintf("You are %s, a specialized AI agent. Complete the assigned task thoroughly and return the result.", agentName)
}

func buildToolCalledActivityData(stepID string, call msg.ToolCallPayload) map[string]string {
	data := map[string]string{
		"step": stepID,
		"tool": call.ToolName,
		"args": sanitizeActivityText(string(call.Arguments), 1024),
	}

	var input map[string]interface{}
	if err := json.Unmarshal(call.Arguments, &input); err == nil {
		if cmd, ok := input["cmd"].(string); ok {
			data["cmd"] = sanitizeActivityText(cmd, 512)
		}
		if path, ok := input["path"].(string); ok {
			data["path"] = sanitizeActivityText(path, 512)
		}
		if operation, ok := input["operation"].(string); ok {
			data["operation"] = sanitizeActivityText(operation, 128)
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

func buildToolCompletedActivityData(stepID string, result msg.ToolResultPayload) map[string]string {
	data := map[string]string{
		"step":     stepID,
		"tool":     result.ToolName,
		"error":    sanitizeActivityText(result.ErrorMessage, 512),
		"result":   sanitizeActivityText(result.Content, 1024),
		"is_error": fmt.Sprintf("%t", result.IsError),
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content), &out); err == nil {
		if exitCode, ok := exitCodeFromOutput(out); ok {
			data["exit_code"] = strconv.Itoa(exitCode)
		}
		if output, ok := out["output"].(string); ok {
			data["output"] = sanitizeActivityText(output, 512)
		}
		if preview, ok := out["content_preview"].(string); ok {
			data["content_preview"] = sanitizeActivityText(preview, 512)
		}
		if content, ok := out["content"].(string); ok {
			data["content"] = sanitizeActivityText(content, 512)
		}
		if path, ok := out["path"].(string); ok {
			data["path"] = sanitizeActivityText(path, 512)
		}
		if operation, ok := out["operation"].(string); ok {
			data["operation"] = sanitizeActivityText(operation, 128)
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

func requiredInitialToolCall(description string) (string, bool) {
	re := regexp.MustCompile(`(?i)first execution response must be a\s+([a-z0-9_-]+)\s+(?:tool|skill)\s+call`)
	matches := re.FindStringSubmatch(description)
	if len(matches) != 2 {
		return "", false
	}
	return strings.ToLower(matches[1]), true
}

func requiredToolCallsBeforeSummary(description string) []string {
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

func missingRequiredToolCalls(required []string, successful map[string]bool) []string {
	if len(required) == 0 {
		return nil
	}

	var missing []string
	for _, toolName := range required {
		if successful[toolName] {
			continue
		}
		missing = append(missing, toolName)
	}
	return missing
}

func buildRequiredToolReminder(toolName string) string {
	if toolName == "" {
		return "Reminder: your first execution response must be only a tool call in a ```tool fenced JSON block. Do not provide prose before the tool call."
	}
	if toolName == "write" {
		return "Reminder: your first execution response must be only a write tool call in a ```tool fenced JSON block. Do not provide prose before the tool call.\n\nFor file creation use:\n```tool\n{\"name\":\"write\",\"input\":{\"path\":\"<path>\",\"operation\":\"write\",\"content\":\"<text>\"}}\n```\n\nFor code updates use:\n```tool\n{\"name\":\"write\",\"input\":{\"path\":\"<path>\",\"operation\":\"edit\",\"edits\":[{\"start_line\":1,\"start_column\":1,\"end_line\":1,\"end_column\":1,\"new_text\":\"<replacement>\"}]}}\n```"
	}
	if toolName == "read" {
		return "Reminder: your first execution response must be only a read tool call in a ```tool fenced JSON block. Do not provide prose before the tool call.\n\nFor file inspection use:\n```tool\n{\"name\":\"read\",\"input\":{\"path\":\"<path>\"}}\n```\n\nFor a partial read use:\n```tool\n{\"name\":\"read\",\"input\":{\"path\":\"<path>\",\"start_line\":1,\"end_line\":20}}\n```\n\nIf the task is an update, the read call is only the inspection step. After you see the file content, make the required write tool call before any final summary."
	}
	return fmt.Sprintf("Reminder: your first execution response must be only a %s tool call in a ```tool fenced JSON block. Do not provide prose before the tool call.", toolName)
}

func buildPendingToolReminder(toolName string) string {
	if toolName == "" {
		return "Reminder: the task is not complete yet. You still must make the required tool call before any final summary."
	}
	if toolName == "write" {
		return "Reminder: the task is not complete yet. You still must use the write tool before any final summary. Respond only with the next write tool call in a ```tool fenced JSON block.\n\nFor file creation use:\n```tool\n{\"name\":\"write\",\"input\":{\"path\":\"<path>\",\"operation\":\"write\",\"content\":\"<text>\"}}\n```\n\nFor code updates use:\n```tool\n{\"name\":\"write\",\"input\":{\"path\":\"<path>\",\"operation\":\"edit\",\"edits\":[{\"start_line\":1,\"start_column\":1,\"end_line\":1,\"end_column\":1,\"new_text\":\"<replacement>\"}]}}\n```"
	}
	if toolName == "read" {
		return "Reminder: the task is not complete yet. You still must use the read tool before any final summary. Respond only with the next read tool call in a ```tool fenced JSON block.\n\nFor file inspection use:\n```tool\n{\"name\":\"read\",\"input\":{\"path\":\"<path>\"}}\n```"
	}
	return fmt.Sprintf("Reminder: the task is not complete yet. You still must use the %s tool before any final summary. Respond only with the next %s tool call in a ```tool fenced JSON block.", toolName, toolName)
}

func exitCodeFromOutput(out map[string]interface{}) (int, bool) {
	v, ok := out["exit_code"]
	if !ok {
		return 0, false
	}
	return intValue(v)
}

func intValue(v interface{}) (int, bool) {
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

func toolErrorMessage(out map[string]interface{}) (string, bool) {
	if isError, ok := out["is_error"].(bool); ok && isError {
		if msg, ok := out["error"].(string); ok {
			msg = sanitizeActivityText(msg, 256)
			if msg != "" {
				return msg, true
			}
		}
		return "tool returned an error", true
	}
	if msg, ok := out["error"].(string); ok {
		msg = sanitizeActivityText(msg, 256)
		if msg != "" {
			return msg, true
		}
	}
	return "", false
}

func toolExitErrorMessage(toolName string, exitCode int, out map[string]interface{}) string {
	msg := fmt.Sprintf("%s exit code %d", toolName, exitCode)
	if output, ok := out["output"].(string); ok {
		output = sanitizeActivityText(output, 256)
		if output != "" {
			msg += ": " + output
		}
	}
	return msg
}

var activitySecretPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:API_KEY|TOKEN|SECRET|PASSWORD))=([^\s"'` + "`" + `]+)`), `$1=[REDACTED]`},
	{regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[^\s"'` + "`" + `]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]+\b`), `[REDACTED]`},
}

func sanitizeActivityText(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	for _, p := range activitySecretPatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	if maxLen > 0 && len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
