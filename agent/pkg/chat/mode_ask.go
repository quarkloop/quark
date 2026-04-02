package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/quarkloop/agent/pkg/agentcore"
	llmctx "github.com/quarkloop/agent/pkg/context"
	msg "github.com/quarkloop/agent/pkg/context/message"
	"github.com/quarkloop/agent/pkg/eventbus"
	"github.com/quarkloop/agent/pkg/execution"
	"github.com/quarkloop/agent/pkg/inference"
)

// processAsk handles a chat request in ask mode. It performs LLM calls with
// an optional tool loop — the LLM can call registered tools to gather
// information, then return a final text answer. No plans are created.
func processAsk(
	ctx context.Context,
	ac *llmctx.AgentContext,
	res *agentcore.Resources,
	deps *Deps,
	req agentcore.ChatRequest,
) (*agentcore.ChatResponse, error) {
	userMsg := req.Message
	var totalIn, totalOut int
	parser := res.Gateway.Parser()

	for iter := 0; iter < agentcore.MaxAskToolIterations; iter++ {
		resp, err := inference.Infer(ctx, ac, res, userMsg)
		if err != nil {
			return nil, fmt.Errorf("ask infer: %w", err)
		}
		totalIn += resp.InputTokens
		totalOut += resp.OutputTokens

		// Prefer native tool calls (structured) over text-parsed ones.
		var toolCall msg.ToolCallPayload
		var hasToolCall bool
		var replyContent string

		if len(resp.ToolCalls) > 0 {
			tc := resp.ToolCalls[0]
			toolCall = execution.NativeToolCallToPayload(tc)
			hasToolCall = true
			replyContent = resp.Content
		} else {
			result := parser.Parse(resp.Content)
			if result.ToolCall != nil {
				toolCall = execution.ModelToolCallToPayload(result.ToolCall)
				hasToolCall = true
			}
			replyContent = result.Content
		}

		if !hasToolCall {
			cr := &agentcore.ChatResponse{
				Reply:        unwrapJSONMessage(replyContent),
				Mode:         string(agentcore.ModeAsk),
				InputTokens:  totalIn,
				OutputTokens: totalOut,
			}
			if replyContent == "" {
				cr.Warning = "model returned an empty response"
			}
			return cr, nil
		}

		// Execute the tool call.
		log.Printf("ask: tool call %s (iter %d)", toolCall.ToolName, iter)
		emitActivity(res.EventBus, req.SessionKey, eventbus.KindToolCalled, execution.BuildToolCalledActivityData("ask", toolCall))
		toolResult := execution.InvokeTool(ctx, res.Dispatcher, "ask", toolCall)
		emitActivity(res.EventBus, req.SessionKey, eventbus.KindToolCompleted, execution.BuildToolCompletedActivityData("ask", toolResult))

		// Store tool output as artifact.
		if !toolResult.IsError {
			res.KB.Set(agentcore.NSArtifacts, "ask-tool-"+toolCall.ToolName, []byte(toolResult.Content))
		}

		// Append tool call + result to context.
		callID, _ := res.IDGen.Next()
		resultID, _ := res.IDGen.Next()
		agentAuthor, _ := llmctx.NewAuthorID(deps.Def.Name)
		toolAuthor, _ := llmctx.NewAuthorID(agentcore.AuthorToolExecutor)

		tc, tr, err := llmctx.NewLinkedToolExchange(
			callID, agentAuthor, toolCall,
			resultID, toolAuthor, toolResult,
			res.TC,
		)
		if err != nil {
			log.Printf("ask: failed to create linked exchange: %v", err)
			break
		}

		tc = tc.WithVisibility(res.VisPolicy.For(llmctx.ToolCallMessageType))
		tr = tr.WithVisibility(res.VisPolicy.For(llmctx.ToolResultMessageType))
		ac.AppendMessage(ctx, tc)
		ac.AppendMessage(ctx, tr)

		userMsg = "" // next iteration: LLM processes tool results
	}

	// Exhausted iterations.
	return &agentcore.ChatResponse{
		Reply:        "Reached maximum tool iterations without a final answer.",
		Mode:         string(agentcore.ModeAsk),
		Warning:      "max tool iterations",
		InputTokens:  totalIn,
		OutputTokens: totalOut,
	}, nil
}

// unwrapJSONMessage strips JSON wrappers that some models add around plain
// text responses. Handles both simple wrappers like {"message": "Hello!"}
// and nested structures like {"results": {"key": "value"}, "status": "done"}.
func unwrapJSONMessage(s string) string {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 2 || trimmed[0] != '{' {
		return s
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return s
	}
	// Collect all string values from the object (recursively).
	var parts []string
	collectStrings(obj, &parts)
	if len(parts) == 0 {
		return s
	}
	return strings.Join(parts, "\n")
}

func collectStrings(v any, out *[]string) {
	switch val := v.(type) {
	case string:
		if s := strings.TrimSpace(val); s != "" {
			*out = append(*out, s)
		}
	case map[string]any:
		for _, child := range val {
			collectStrings(child, out)
		}
	case []any:
		for _, child := range val {
			collectStrings(child, out)
		}
	}
}

func emitActivity(bus *eventbus.Bus, sessionKey string, kind eventbus.EventKind, data interface{}) {
	if bus == nil {
		return
	}
	now := time.Now()
	bus.Emit(eventbus.Event{
		ID:        fmt.Sprintf("%s-%d", kind, now.UnixNano()),
		SessionID: sessionKey,
		Kind:      kind,
		Timestamp: now.UTC(),
		Data:      data,
	})
}
