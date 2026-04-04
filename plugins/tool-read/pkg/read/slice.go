package read

import (
	"fmt"
	"strings"
)

func sliceContent(content string, req normalizedRequest) (snippet string, startLine, endLine, totalLines int, err error) {
	lines := splitLineSegments(content)
	totalLines = len(lines)

	if !req.HasRange {
		return content, 1, totalLines, totalLines, nil
	}
	if req.StartLine > totalLines {
		return "", 0, 0, totalLines, fmt.Errorf("start_line %d out of range", req.StartLine)
	}
	if req.EndLine > totalLines {
		return "", 0, 0, totalLines, fmt.Errorf("end_line %d out of range", req.EndLine)
	}

	return strings.Join(lines[req.StartLine-1:req.EndLine], ""), req.StartLine, req.EndLine, totalLines, nil
}

func splitLineSegments(content string) []string {
	if content == "" {
		return []string{""}
	}

	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
