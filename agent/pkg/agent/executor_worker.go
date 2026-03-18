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
)

// ─── WORKER ──────────────────────────────────────────────────────────────────

// runWorker drives one worker agent through its EXECUTE → REPORT loop.
func (e *Executor) runWorker(ctx context.Context, step Step) {
	log.Printf("worker[%s]: starting (%s)", step.ID, step.Agent)

	result, err := e.executeStep(ctx, step)

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
	if err := e.kb.Set(NSEvents, step.ID+"-done", data); err != nil {
		log.Printf("worker[%s]: failed to write event: %v", step.ID, err)
	}
}

// executeStep runs a single step using a short-lived AgentContext.
// Supports multi-turn tool loops with linked ToolCall/ToolResult pairs.
func (e *Executor) executeStep(ctx context.Context, step Step) (string, error) {
	def, ok := e.agents[step.Agent]
	if !ok && step.Agent != AuthorSupervisor {
		return "", fmt.Errorf("unknown agent %q", step.Agent)
	}
	if step.Agent == AuthorSupervisor {
		def = e.supervisor
	}

	systemPrompt := resolveSystemPrompt(e, def, step.Agent)

	workerCtx, err := e.buildWorkerContext(systemPrompt, def.Config.ContextWindow)
	if err != nil {
		return e.executeStepRaw(ctx, step, systemPrompt)
	}

	artifacts := e.gatherArtifacts()
	userMsg := fmt.Sprintf("Task: %s\n\nContext from knowledge base:\n%s\n\nComplete this task and provide a detailed result.",
		step.Description, artifacts)

	var finalResult string
	for iter := 0; iter < MaxToolIterations; iter++ {
		resp, err := e.inferWithContext(ctx, workerCtx, userMsg)
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
		result := e.executeTool(ctx, step.ID, *toolCall)

		callID, _ := e.idGen.Next()
		resultID, _ := e.idGen.Next()
		agentAuthor, _ := llmctx.NewAuthorID(step.Agent)
		toolAuthor, _ := llmctx.NewAuthorID(AuthorToolExecutor)

		tc, tr, err := llmctx.NewLinkedToolExchange(
			callID, agentAuthor, *toolCall,
			resultID, toolAuthor, result,
			e.tc,
		)
		if err != nil {
			log.Printf("worker[%s]: failed to create linked exchange: %v", step.ID, err)
			continue
		}

		tc = tc.WithVisibility(e.visPolicy.For(llmctx.ToolCallMessageType))
		tr = tr.WithVisibility(e.visPolicy.For(llmctx.ToolResultMessageType))

		workerCtx.AppendMessage(ctx, tc)
		workerCtx.AppendMessage(ctx, tr)

		userMsg = "" // next iteration: let LLM process tool results
	}

	if finalResult == "" {
		finalResult = "Task completed with tool interactions."
	}

	if err := e.kb.Set(NSArtifacts, step.ID, []byte(finalResult)); err != nil {
		log.Printf("worker[%s]: failed to write artifact: %v", step.ID, err)
	}

	if report := workerCtx.DetectOrphans(); report.HasOrphans() {
		log.Printf("worker[%s]: %d orphaned messages detected", step.ID, len(report.OrphanIDs))
	}

	return finalResult, nil
}

// executeStepRaw is the fallback for when worker context build fails.
func (e *Executor) executeStepRaw(ctx context.Context, step Step, systemPrompt string) (string, error) {
	artifacts := e.gatherArtifacts()
	userMsg := fmt.Sprintf("Task: %s\n\nContext from knowledge base:\n%s\n\nComplete this task and provide a detailed result.",
		step.Description, artifacts)

	sysID, _ := e.idGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(step.Agent)
	sysMsg, err := llmctx.NewSystemPromptMessage(sysID, agentAuthor, systemPrompt, e.tc)
	if err != nil {
		return "", err
	}

	window, _ := llmctx.NewContextWindow(DefaultContextWindow)
	ac, err := llmctx.NewAgentContextBuilder().
		WithSystemPrompt(sysMsg).
		WithContextWindow(window).
		WithTokenComputer(e.tc).
		Build()
	if err != nil {
		return "", err
	}

	m, _ := e.newUserMsg(userMsg)
	ac.AppendMessage(ctx, m)

	adapter, _ := e.adapterReg.Get(e.gateway.Provider())
	ca := llmctx.NewContextAdapter(ac, adapter)
	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     e.gateway.ModelName(),
		MaxTokens: e.gateway.MaxTokens(),
	})
	if err != nil {
		return "", err
	}

	resp, err := e.gateway.InferRaw(ctx, payload)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// buildWorkerContext creates a short-lived AgentContext for a worker step.
func (e *Executor) buildWorkerContext(systemPrompt string, windowSize int) (*llmctx.AgentContext, error) {
	if windowSize <= 0 {
		windowSize = DefaultContextWindow
	}
	window, _ := llmctx.NewContextWindow(int32(windowSize))

	sysID, _ := e.idGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(AuthorWorker)
	sysMsg, err := llmctx.NewSystemPromptMessage(sysID, agentAuthor, systemPrompt, e.tc)
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
		WithTokenComputer(e.tc).
		WithIDGenerator(e.idGen).
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
func (e *Executor) executeTool(ctx context.Context, stepID string, call msg.ToolCallPayload) msg.ToolResultPayload {
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

	out, err := e.dispatcher.Invoke(ctx, call.ToolName, input)
	if err != nil {
		return msg.ToolResultPayload{
			ToolCallID:   call.ToolCallID,
			ToolName:     call.ToolName,
			IsError:      true,
			ErrorMessage: err.Error(),
		}
	}

	outData, _ := json.Marshal(out)
	e.kb.Set(NSArtifacts, stepID+"-skill-"+call.ToolName, outData)
	log.Printf("worker[%s]: skill %s invoked successfully", stepID, call.ToolName)

	return msg.ToolResultPayload{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Content:    string(outData),
	}
}

// resolveSystemPrompt finds the system prompt for an agent, checking KB config,
// the agent definition, and falling back to a default.
func resolveSystemPrompt(e *Executor, def *Definition, agentName string) string {
	if def.SystemPrompt != "" {
		return def.SystemPrompt
	}
	if data, err := e.kb.Get(NSConfig, agentName+"-prompt"); err == nil {
		return string(data)
	}
	return fmt.Sprintf("You are %s, a specialized AI agent. Complete the assigned task thoroughly and return the result.", agentName)
}
