package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ToolCall is a provider-normalised tool invocation extracted from LLM output.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ParseResult holds the outcome of parsing an LLM response for tool calls.
type ParseResult struct {
	// ToolCall is non-nil when a tool invocation was found.
	ToolCall *ToolCall
	// Content is the model's prose with tool-call markup removed.
	Content string
}

// ToolCallParser extracts tool calls from raw LLM response text.
// Implementations are provider/model-specific.
type ToolCallParser interface {
	// Parse examines content for a tool call. Returns ParseResult with
	// ToolCall set if found, and Content always stripped of tool-call markup.
	Parse(content string) ParseResult

	// FormatHint returns the system-prompt fragment that tells the model
	// how to format tool calls. toolNames lists available tools.
	FormatHint(toolNames []string) string
}

// ChainParser tries multiple parsers in order, returning the first match.
type ChainParser struct {
	Parsers []ToolCallParser
}

func (c ChainParser) Parse(content string) ParseResult {
	for _, p := range c.Parsers {
		r := p.Parse(content)
		if r.ToolCall != nil {
			return r
		}
	}
	// No tool call found — still strip markup from all parsers.
	cleaned := content
	for _, p := range c.Parsers {
		cleaned = p.Parse(cleaned).Content
	}
	return ParseResult{Content: cleaned}
}

func (c ChainParser) FormatHint(toolNames []string) string {
	if len(c.Parsers) > 0 {
		return c.Parsers[0].FormatHint(toolNames)
	}
	return ""
}

// newToolCallID generates a unique tool call ID.
func newToolCallID() string {
	return fmt.Sprintf("tc-%d", time.Now().UnixNano())
}

// fencedParser is the default; openrouter and zhipu use a chain.
var defaultParser ToolCallParser = &FencedBlockParser{}

// ParserFor returns the appropriate ToolCallParser for a provider.
func ParserFor(provider string) ToolCallParser {
	switch provider {
	case "openrouter", "zhipu":
		return ChainParser{Parsers: []ToolCallParser{
			&FencedBlockParser{},
			&XMLToolCallParser{},
		}}
	default:
		// anthropic, openai, noop — fenced block format.
		return defaultParser
	}
}

// FormatHintForTools is a convenience that builds a complete tool-use instruction
// block for system prompts, including the tool list and format hint.
func FormatHintForTools(parser ToolCallParser, toolNames []string) string {
	if len(toolNames) == 0 {
		return ""
	}
	return fmt.Sprintf("You have access to these tools: %s\n\n%s",
		strings.Join(toolNames, ", "),
		parser.FormatHint(toolNames))
}
