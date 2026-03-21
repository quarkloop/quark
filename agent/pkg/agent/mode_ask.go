package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/quarkloop/agent/pkg/activity"
	llmctx "github.com/quarkloop/agent/pkg/context"
	msg "github.com/quarkloop/agent/pkg/context/message"
	"github.com/quarkloop/agent/pkg/model"
)

// MaxAskToolIterations is the maximum number of tool-call rounds in ask mode.
const MaxAskToolIterations = 10

// chatAsk handles a chat request in ask mode. It performs LLM calls with
// an optional tool loop — the LLM can call registered tools to gather
// information, then return a final text answer. No plans are created.
func (a *Agent) chatAsk(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	userMsg := req.Message
	var totalIn, totalOut int
	parser := a.gateway.Parser()

	for iter := 0; iter < MaxAskToolIterations; iter++ {
		resp, err := a.inferWithContext(ctx, a.ctx, userMsg)
		if err != nil {
			return nil, fmt.Errorf("ask infer: %w", err)
		}
		totalIn += resp.InputTokens
		totalOut += resp.OutputTokens

		result := parser.Parse(resp.Content)
		if result.ToolCall == nil {
			// No tool call — this is the final answer.
			reply := result.Content
			cr := &ChatResponse{
				Reply:        reply,
				Mode:         string(ModeAsk),
				InputTokens:  totalIn,
				OutputTokens: totalOut,
			}
			if reply == "" {
				cr.Warning = "model returned an empty response"
			}
			a.saveCheckpoint()
			return cr, nil
		}

		// Convert model.ToolCall to msg.ToolCallPayload.
		toolCall := modelToolCallToPayload(result.ToolCall)

		// Execute the tool call.
		log.Printf("ask: tool call %s (iter %d)", toolCall.ToolName, iter)
		a.emit(activity.ToolCalled, buildToolCalledActivityData("ask", toolCall))
		toolResult := a.executeTool(ctx, "ask", toolCall)
		a.emit(activity.ToolCompleted, buildToolCompletedActivityData("ask", toolResult))

		// Append tool call + result to context.
		callID, _ := a.idGen.Next()
		resultID, _ := a.idGen.Next()
		agentAuthor, _ := llmctx.NewAuthorID(a.def.Name)
		toolAuthor, _ := llmctx.NewAuthorID(AuthorToolExecutor)

		tc, tr, err := llmctx.NewLinkedToolExchange(
			callID, agentAuthor, toolCall,
			resultID, toolAuthor, toolResult,
			a.tc,
		)
		if err != nil {
			log.Printf("ask: failed to create linked exchange: %v", err)
			break
		}

		tc = tc.WithVisibility(a.visPolicy.For(llmctx.ToolCallMessageType))
		tr = tr.WithVisibility(a.visPolicy.For(llmctx.ToolResultMessageType))
		a.ctx.AppendMessage(ctx, tc)
		a.ctx.AppendMessage(ctx, tr)

		userMsg = "" // next iteration: LLM processes tool results
	}

	// Exhausted iterations.
	a.saveCheckpoint()
	return &ChatResponse{
		Reply:        "Reached maximum tool iterations without a final answer.",
		Mode:         string(ModeAsk),
		Warning:      "max tool iterations",
		InputTokens:  totalIn,
		OutputTokens: totalOut,
	}, nil
}

// modelToolCallToPayload converts a model.ToolCall to the context message type.
func modelToolCallToPayload(tc *model.ToolCall) msg.ToolCallPayload {
	return msg.ToolCallPayload{
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		Arguments:  tc.Arguments,
	}
}
