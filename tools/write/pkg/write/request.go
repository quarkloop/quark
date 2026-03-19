package write

import (
	"fmt"
	"path/filepath"
	"strings"
)

type normalizedRequest struct {
	Path        string
	Operation   string
	Content     string
	Find        string
	ReplaceWith string
	Edits       []Edit
}

func normalizeRequest(req Request) (normalizedRequest, error) {
	path := filepath.Clean(strings.TrimSpace(req.Path))
	if path == "" || path == "." {
		return normalizedRequest{}, fmt.Errorf("path is required")
	}

	nreq := normalizedRequest{
		Path:        path,
		Operation:   normalizedOperation(req.Operation),
		Content:     req.Content,
		Find:        req.Find,
		ReplaceWith: req.ReplaceWith,
	}

	switch nreq.Operation {
	case "write", "append":
		if strings.ContainsRune(nreq.Content, '\x00') {
			return normalizedRequest{}, fmt.Errorf("content must be valid text")
		}
	case "replace":
		if nreq.Find == "" {
			return normalizedRequest{}, fmt.Errorf("find is required for replace")
		}
		if strings.ContainsRune(nreq.Find, '\x00') || strings.ContainsRune(nreq.ReplaceWith, '\x00') {
			return normalizedRequest{}, fmt.Errorf("replace arguments must be valid text")
		}
	case "edit":
		edits, err := normalizeEdits(req)
		if err != nil {
			return normalizedRequest{}, err
		}
		nreq.Edits = edits
	default:
		return normalizedRequest{}, fmt.Errorf("operation must be write, append, replace, or edit")
	}

	return nreq, nil
}

func normalizedOperation(op string) string {
	op = strings.TrimSpace(strings.ToLower(op))
	if op == "" {
		return "write"
	}
	return op
}

func normalizeEdits(req Request) ([]Edit, error) {
	edits := append([]Edit(nil), req.Edits...)
	if len(edits) == 0 && hasSingleEdit(req) {
		edits = []Edit{{
			StartLine:   req.StartLine,
			StartColumn: req.StartColumn,
			EndLine:     req.EndLine,
			EndColumn:   req.EndColumn,
			NewText:     req.NewText,
		}}
	}
	if len(edits) == 0 {
		return nil, fmt.Errorf("edit requires at least one edit range")
	}

	for i := range edits {
		edit, err := normalizeEdit(edits[i])
		if err != nil {
			return nil, fmt.Errorf("edit %d: %w", i+1, err)
		}
		edits[i] = edit
	}
	return edits, nil
}

func hasSingleEdit(req Request) bool {
	return req.StartLine != 0 ||
		req.StartColumn != 0 ||
		req.EndLine != 0 ||
		req.EndColumn != 0 ||
		req.NewText != ""
}

func normalizeEdit(edit Edit) (Edit, error) {
	if edit.StartLine < 1 {
		return Edit{}, fmt.Errorf("start_line must be >= 1")
	}
	if edit.StartColumn < 1 {
		return Edit{}, fmt.Errorf("start_column must be >= 1")
	}
	if edit.EndLine == 0 {
		edit.EndLine = edit.StartLine
	}
	if edit.EndColumn == 0 {
		edit.EndColumn = edit.StartColumn
	}
	if edit.EndLine < 1 {
		return Edit{}, fmt.Errorf("end_line must be >= 1")
	}
	if edit.EndColumn < 1 {
		return Edit{}, fmt.Errorf("end_column must be >= 1")
	}
	if edit.EndLine < edit.StartLine || (edit.EndLine == edit.StartLine && edit.EndColumn < edit.StartColumn) {
		return Edit{}, fmt.Errorf("end position must not be before start position")
	}
	if strings.ContainsRune(edit.NewText, '\x00') {
		return Edit{}, fmt.Errorf("new_text must be valid text")
	}
	return edit, nil
}
