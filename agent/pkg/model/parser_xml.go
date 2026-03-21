package model

import (
	"encoding/json"
	"regexp"
	"strings"
)

// XMLToolCallParser extracts tool calls from the <tool_call> XML format
// emitted by many open-source models (Qwen, StepFun, GLM, etc.).
//
// Expected format:
//
//	<tool_call>
//	<function=bash>
//	<parameter=command>date</parameter>
//	</function>
//	</tool_call>
type XMLToolCallParser struct{}

var (
	xmlToolCallBlockRe = regexp.MustCompile(`(?s)<tool_call>.*?</tool_call>`)
	xmlFuncRe          = regexp.MustCompile(`(?s)<tool_call>\s*<function=(\w[\w./-]*)>(.*?)</function>\s*</tool_call>`)
	xmlParamRe         = regexp.MustCompile(`(?s)<parameter=(\w+)>(.*?)</parameter>`)
)

func (p *XMLToolCallParser) Parse(content string) ParseResult {
	m := xmlFuncRe.FindStringSubmatch(content)
	if m == nil {
		// Still strip any malformed <tool_call> blocks from content.
		cleaned := strings.TrimSpace(xmlToolCallBlockRe.ReplaceAllString(content, ""))
		return ParseResult{Content: cleaned}
	}

	funcName := m[1]
	body := m[2]

	params := map[string]string{}
	for _, pm := range xmlParamRe.FindAllStringSubmatch(body, -1) {
		params[pm[1]] = strings.TrimSpace(pm[2])
	}

	args, err := json.Marshal(params)
	if err != nil {
		return ParseResult{Content: content}
	}

	// Strip the matched <tool_call> block from content.
	cleaned := strings.TrimSpace(xmlToolCallBlockRe.ReplaceAllString(content, ""))

	return ParseResult{
		ToolCall: &ToolCall{
			ID:        newToolCallID(),
			Name:      funcName,
			Arguments: args,
		},
		Content: cleaned,
	}
}

func (p *XMLToolCallParser) FormatHint(toolNames []string) string {
	return `To use a tool, respond with:
<tool_call>
<function=tool-name>
<parameter=key>value</parameter>
</function>
</tool_call>

After you receive the tool result, use it to answer the user's question.`
}
