package model

// NativeToolCallParser is a no-op text parser used when native function
// calling is active. Tool calls arrive as structured data in
// RawResponse.ToolCalls rather than embedded in text, so Parse always
// returns the content unchanged and FormatHint returns an empty string.
type NativeToolCallParser struct{}

func (p *NativeToolCallParser) Parse(content string) ParseResult {
	return ParseResult{Content: content}
}

func (p *NativeToolCallParser) FormatHint(toolNames []string) string {
	return ""
}
