package read

import (
	"fmt"
	"path/filepath"
	"strings"
)

type normalizedRequest struct {
	Path      string
	StartLine int
	EndLine   int
	HasRange  bool
}

func normalizeRequest(req Request) (normalizedRequest, error) {
	path := filepath.Clean(strings.TrimSpace(req.Path))
	if path == "" || path == "." {
		return normalizedRequest{}, fmt.Errorf("path is required")
	}

	nreq := normalizedRequest{Path: path}
	if req.StartLine == 0 && req.EndLine == 0 {
		return nreq, nil
	}

	startLine := req.StartLine
	if startLine == 0 {
		startLine = 1
	}
	if startLine < 1 {
		return normalizedRequest{}, fmt.Errorf("start_line must be >= 1")
	}

	endLine := req.EndLine
	if endLine == 0 {
		endLine = startLine
	}
	if endLine < startLine {
		return normalizedRequest{}, fmt.Errorf("end_line must be >= start_line")
	}

	nreq.StartLine = startLine
	nreq.EndLine = endLine
	nreq.HasRange = true
	return nreq, nil
}
