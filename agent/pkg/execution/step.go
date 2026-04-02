package execution

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	msg "github.com/quarkloop/agent/pkg/context/message"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/inference"
	"github.com/quarkloop/agent/pkg/plan"
)

// ExecuteStep runs a plan step in a dedicated context.
// It builds a short-lived AgentContext, loops LLM inference + tool calls
// (up to MaxToolIterations), and returns the final result text.
func ExecuteStep(
	ctx context.Context,
	res *agentcore.Resources,
	def *agentcore.Definition,
	step plan.Step,
	subAgents map[string]*agentcore.Definition,
	artifacts string,
) (string, error) {
	stepDef, ok := subAgents[step.Agent]
	if !ok && step.Agent != def.Name {
		return "", fmt.Errorf("unknown agent %q", step.Agent)
	}
	if step.Agent == def.Name {
		stepDef = def
	}

	systemPrompt := resolveSystemPrompt(res, stepDef, step.Agent)

	subagentCtx, err := buildSubagentContext(res, systemPrompt, agentcore.DefaultContextWindow)
	if err != nil {
		return executeStepRaw(ctx, res, step, systemPrompt, artifacts)
	}

	userMsg := fmt.Sprintf("Task: %s\n\nContext from knowledge base:\n%s\n\nComplete this task and provide a detailed result.",
		step.Description, artifacts)
	requiredToolName, requiresToolFirst := RequiredInitialToolCall(step.Description)
	requiredToolsBeforeSummary := RequiredToolCallsBeforeSummary(step.Description)
	successfulToolCalls := map[string]bool{}
	initialToolSatisfied := !requiresToolFirst
	parser := res.Gateway.Parser()

	var finalResult string
	for iter := 0; iter < agentcore.MaxToolIterations; iter++ {
		resp, err := inference.Infer(ctx, subagentCtx, res, userMsg)
		if err != nil {
			return "", fmt.Errorf("infer iter %d: %w", iter, err)
		}
		log.Printf("subagent[%s]: iter %d, %d tokens", step.ID, iter, resp.TotalTokens())

		// Prefer native tool calls over text-parsed ones.
		var toolCall msg.ToolCallPayload
		var hasToolCall bool
		var proseContent string

		if len(resp.ToolCalls) > 0 {
			tc := resp.ToolCalls[0]
			toolCall = NativeToolCallToPayload(tc)
			hasToolCall = true
			proseContent = resp.Content
		} else {
			parsed := parser.Parse(resp.Content)
			if parsed.ToolCall != nil {
				toolCall = ModelToolCallToPayload(parsed.ToolCall)
				hasToolCall = true
			}
			proseContent = parsed.Content
		}

		if !hasToolCall {
			if requiresToolFirst && !initialToolSatisfied && iter < agentcore.MaxToolIterations-1 {
				userMsg = BuildRequiredToolReminder(parser, requiredToolName)
				continue
			}
			if missing := MissingRequiredToolCalls(requiredToolsBeforeSummary, successfulToolCalls); len(missing) > 0 && iter < agentcore.MaxToolIterations-1 {
				userMsg = BuildPendingToolReminder(parser, missing[0])
				continue
			}
			finalResult = proseContent
			break
		}

		emitActivity(res.EventBus, eventbus.KindToolCalled, BuildToolCalledActivityData(step.ID, toolCall))
		result := InvokeTool(ctx, res.Dispatcher, step.ID, toolCall)
		emitActivity(res.EventBus, eventbus.KindToolCompleted, BuildToolCompletedActivityData(step.ID, result))

		if !result.IsError {
			successfulToolCalls[strings.ToLower(toolCall.ToolName)] = true
			if requiresToolFirst && strings.EqualFold(toolCall.ToolName, requiredToolName) {
				initialToolSatisfied = true
			}
		}

		callID, _ := res.IDGen.Next()
		resultID, _ := res.IDGen.Next()
		agentAuthor, _ := llmctx.NewAuthorID(step.Agent)
		toolAuthor, _ := llmctx.NewAuthorID(agentcore.AuthorToolExecutor)

		tc, tr, err := llmctx.NewLinkedToolExchange(
			callID, agentAuthor, toolCall,
			resultID, toolAuthor, result,
			res.TC,
		)
		if err != nil {
			log.Printf("subagent[%s]: failed to create linked exchange: %v", step.ID, err)
			continue
		}

		tc = tc.WithVisibility(res.VisPolicy.For(llmctx.ToolCallMessageType))
		tr = tr.WithVisibility(res.VisPolicy.For(llmctx.ToolResultMessageType))
		subagentCtx.AppendMessage(ctx, tc)
		subagentCtx.AppendMessage(ctx, tr)

		userMsg = "" // next iteration: let LLM process tool results
	}

	if finalResult == "" {
		if requiresToolFirst && !initialToolSatisfied {
			return "", fmt.Errorf("required initial tool %q was not executed", requiredToolName)
		}
		if missing := MissingRequiredToolCalls(requiredToolsBeforeSummary, successfulToolCalls); len(missing) > 0 {
			return "", fmt.Errorf("required tool calls not completed before summary: %s", strings.Join(missing, ", "))
		}
		finalResult = "Task completed with required tool interactions."
	}

	if err := res.KB.Set(agentcore.NSArtifacts, step.ID, []byte(finalResult)); err != nil {
		log.Printf("subagent[%s]: failed to write artifact: %v", step.ID, err)
	}

	if report := subagentCtx.DetectOrphans(); report.HasOrphans() {
		log.Printf("subagent[%s]: %d orphaned messages detected", step.ID, len(report.OrphanIDs))
	}

	return finalResult, nil
}

// executeStepRaw is the fallback for when worker context build fails.
func executeStepRaw(ctx context.Context, res *agentcore.Resources, step plan.Step, systemPrompt string, artifacts string) (string, error) {
	userMsg := fmt.Sprintf("Task: %s\n\nContext from knowledge base:\n%s\n\nComplete this task and provide a detailed result.",
		step.Description, artifacts)

	sysID, _ := res.IDGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(step.Agent)
	sysMsg, err := llmctx.NewSystemPromptMessage(sysID, agentAuthor, systemPrompt, res.TC)
	if err != nil {
		return "", err
	}

	window, _ := llmctx.NewContextWindow(agentcore.DefaultContextWindow)
	ac, err := llmctx.NewAgentContextBuilder().
		WithSystemPrompt(sysMsg).
		WithContextWindow(window).
		WithTokenComputer(res.TC).
		Build()
	if err != nil {
		return "", err
	}

	m, _ := inference.NewUserMessage(res.TC, res.IDGen, agentcore.AuthorUser, userMsg)
	ac.AppendMessage(ctx, m)

	adapter, _ := res.AdapterReg.Get(res.Gateway.Provider())
	ca := llmctx.NewContextAdapter(ac, adapter)
	payload, err := ca.BuildRequest(llmctx.RequestOptions{
		Model:     res.Gateway.ModelName(),
		MaxTokens: res.Gateway.MaxTokens(),
	})
	if err != nil {
		return "", err
	}

	resp, err := res.Gateway.InferRaw(ctx, payload)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// buildSubagentContext creates a short-lived AgentContext for a worker step.
func buildSubagentContext(res *agentcore.Resources, systemPrompt string, windowSize int) (*llmctx.AgentContext, error) {
	if windowSize <= 0 {
		windowSize = agentcore.DefaultContextWindow
	}
	window, _ := llmctx.NewContextWindow(int32(windowSize))

	sysID, _ := res.IDGen.Next()
	agentAuthor, _ := llmctx.NewAuthorID(agentcore.AuthorSubagent)
	sysMsg, err := llmctx.NewSystemPromptMessage(sysID, agentAuthor, systemPrompt, res.TC)
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
	compactor, err := llmctx.NewThresholdCompactor(pipeline, agentcore.DefaultCompactionThreshold)
	if err != nil {
		return nil, err
	}

	return llmctx.NewAgentContextBuilder().
		WithSystemPrompt(sysMsg).
		WithContextWindow(window).
		WithCompactor(compactor).
		WithTokenComputer(res.TC).
		WithIDGenerator(res.IDGen).
		Build()
}

// resolveSystemPrompt finds the system prompt for an agent.
func resolveSystemPrompt(res *agentcore.Resources, def *agentcore.Definition, agentName string) string {
	if def.SystemPrompt != "" {
		if !strings.Contains(def.SystemPrompt, "\n") {
			if data, err := os.ReadFile(def.SystemPrompt); err == nil {
				return string(data)
			}
		}
		return def.SystemPrompt
	}
	if data, err := res.KB.Get(agentcore.NSConfig, agentName+"-prompt"); err == nil {
		return string(data)
	}
	return fmt.Sprintf("You are %s, a specialized AI agent. Complete the assigned task thoroughly and return the result.", agentName)
}

// GatherArtifacts reads relevant KB entries to provide as worker context.
func GatherArtifacts(res *agentcore.Resources) string {
	var sb strings.Builder
	namespaces := []string{agentcore.NSMemory, agentcore.NSDocuments, agentcore.NSNotes, agentcore.NSArtifacts}
	for _, ns := range namespaces {
		keys, err := res.KB.List(ns)
		if err != nil || len(keys) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n[%s]\n", ns))
		for _, k := range keys {
			data, err := res.KB.Get(ns, k)
			if err != nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", k, Truncate(string(data), 1000)))
		}
	}
	return sb.String()
}

func emitActivity(bus *eventbus.Bus, kind eventbus.EventKind, data interface{}) {
	if bus == nil {
		return
	}
	bus.Emit(eventbus.Event{
		Kind: kind,
		Data: data,
	})
}
