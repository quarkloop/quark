package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FencedBlockParser extracts tool calls from ```tool JSON fences.
// This is the default format used by well-instructed models (Anthropic, OpenAI).
//
// Expected format:
//
//	```tool
//	{"name":"bash","input":{"cmd":"date"}}
//	```
type FencedBlockParser struct{}

func (p *FencedBlockParser) Parse(content string) ParseResult {
	block, start, end := extractFencedBlock(content)
	if block == "" {
		return ParseResult{Content: content}
	}

	var call struct {
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal([]byte(block), &call); err != nil {
		return ParseResult{Content: content}
	}
	if call.Name == "" {
		return ParseResult{Content: content}
	}

	cleaned := strings.TrimSpace(content[:start] + content[end:])
	return ParseResult{
		ToolCall: &ToolCall{
			ID:        newToolCallID(),
			Name:      call.Name,
			Arguments: call.Input,
		},
		Content: cleaned,
	}
}

func (p *FencedBlockParser) FormatHint(toolNames []string) string {
	return fmt.Sprintf(`When you need to use a tool, wrap the call in a fenced block:
` + "```tool" + `
{"name":"<tool-name>","input":{<arguments>}}
` + "```" + `

IMPORTANT:
- Only use the fenced tool block when you want to invoke a tool.
- When answering the user, respond in plain natural language — NEVER wrap your answer in JSON.
- After you receive a tool result, summarise it in plain text for the user.
- Do NOT output XML or <tool_call> tags.`)
}

// extractFencedBlock finds the first ```tool or ```skill fenced block.
// Returns the block content, and the start/end byte offsets of the full
// fence (including markers) within content. Returns ("", 0, 0) if not found.
func extractFencedBlock(content string) (block string, fenceStart, fenceEnd int) {
	for _, fence := range []string{"tool", "skill", "json", "bash"} {
		marker := "```" + fence
		start := strings.Index(content, marker)
		if start < 0 {
			continue
		}
		afterMarker := start + len(marker)
		end := strings.Index(content[afterMarker:], "```")
		if end < 0 {
			continue
		}
		block := strings.TrimSpace(content[afterMarker : afterMarker+end])
		fenceEnd := afterMarker + end + 3 // include closing ```
		return block, start, fenceEnd
	}
	return "", 0, 0
}
