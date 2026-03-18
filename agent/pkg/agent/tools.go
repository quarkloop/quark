package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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

	var finalResult string
	for iter := 0; iter < MaxToolIterations; iter++ {
		resp, err := a.inferWithContext(ctx, workerCtx, userMsg)
		if err != nil {
			return "", fmt.Errorf("infer iter %d: %w", iter, err)
		}
		log.Printf("worker[%s]: iter %d, %d tokens", step.ID, iter, resp.TotalTokens())

		toolCall := parseToolCall(resp.Content)
		if toolCall == nil {
			finalResult = resp.Content
			break
		}

		log.Printf("worker[%s]: tool call %s(%s)", step.ID, toolCall.ToolName, string(toolCall.Arguments))
		result := a.executeTool(ctx, step.ID, *toolCall)

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
		finalResult = "Task completed with tool interactions."
	}

	if err := a.kb.Set(NSArtifacts, step.ID, []byte(finalResult)); err != nil {
		log.Printf("worker[%s]: failed to write artifact: %v", step.ID, err)
	}

	if report := workerCtx.DetectOrphans(); report.HasOrphans() {
		log.Printf("worker[%s]: %d orphaned messages detected", step.ID, len(report.OrphanIDs))
	}

	return finalResult, nil
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
// Supports ```skill JSON fences.
func parseToolCall(content string) *msg.ToolCallPayload {
	const marker = "```skill"
	start := strings.Index(content, marker)
	if start < 0 {
		return nil
	}
	end := strings.Index(content[start+len(marker):], "```")
	if end < 0 {
		return nil
	}
	block := strings.TrimSpace(content[start+len(marker) : start+len(marker)+end])
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

// executeTool dispatches a tool call to the skill dispatcher and returns
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
	log.Printf("worker[%s]: skill %s invoked successfully", stepID, call.ToolName)

	return msg.ToolResultPayload{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Content:    string(outData),
	}
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
