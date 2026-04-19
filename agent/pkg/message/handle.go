package message

import (
	"context"
	"fmt"

	"github.com/quarkloop/agent/pkg/llm"
	"github.com/quarkloop/agent/pkg/llmcontext"
	"github.com/quarkloop/agent/pkg/provider"
)

// Handle runs the full message handling flow:
//  1. Build LLM context (system prompt + work status + session history)
//  2. Call LLM via Infer loop (streaming + tool calling)
//  3. Return full assistant response text
//
// Tokens are streamed to resp as they arrive.
func Handle(ctx context.Context, history []Message, llmClient *llm.Client, systemPrompt string, workSummary string, tools []provider.Tool, onTool llm.ToolHandler, resp chan<- StreamMessage) (string, error) {
	if llmClient == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	// Build LLM messages
	var msgs []provider.Message

	// System prompt
	if systemPrompt != "" {
		msgs = append(msgs, provider.Message{Role: "system", Content: systemPrompt})
	}

	// Work status injection
	if workSummary != "" && workSummary != "No active work." {
		msgs = append(msgs, provider.Message{
			Role:    "system",
			Content: "Current work status: " + workSummary,
		})
	}

	// Session history — compact only when approaching the model's context window limit.
	contents := make([]int, len(history))
	for i, m := range history {
		contents[i] = len(m.Content)
	}
	start := llmcontext.CompactIndex(contents, llmClient.ContextWindow)
	for _, m := range history[start:] {
		msgs = append(msgs, provider.Message{Role: m.Role, Content: m.Content})
	}

	// Infer (LLM call → tool loop → streaming)
	return llmClient.Infer(ctx, msgs, tools, onTool, func(msgType string, data any) {
		resp <- StreamMessage{Type: msgType, Data: data}
	})
}
