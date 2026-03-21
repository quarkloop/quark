// Package write implements the quark write tool.
package write

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const contentPreviewLimit = 512

// Request describes one write-tool invocation.
type Request struct {
	Path        string `json:"path"`
	Operation   string `json:"operation,omitempty"`
	Content     string `json:"content,omitempty"`
	Find        string `json:"find,omitempty"`
	ReplaceWith string `json:"replace_with,omitempty"`

	StartLine   int    `json:"start_line,omitempty"`
	StartColumn int    `json:"start_column,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
	EndColumn   int    `json:"end_column,omitempty"`
	NewText     string `json:"new_text,omitempty"`

	Edits []Edit `json:"edits,omitempty"`
}

// Edit replaces the text in the half-open range [start, end) using 1-based
// line/column positions. If end is omitted, the edit becomes an insertion at
// the start position.
type Edit struct {
	StartLine   int    `json:"start_line"`
	StartColumn int    `json:"start_column"`
	EndLine     int    `json:"end_line,omitempty"`
	EndColumn   int    `json:"end_column,omitempty"`
	NewText     string `json:"new_text,omitempty"`
}

// Result describes the final state after a write-tool operation.
type Result struct {
	Path           string `json:"path"`
	Operation      string `json:"operation"`
	Created        bool   `json:"created"`
	Changed        bool   `json:"changed"`
	BytesWritten   int    `json:"bytes_written"`
	FileSize       int    `json:"file_size"`
	Replacements   int    `json:"replacements,omitempty"`
	EditsApplied   int    `json:"edits_applied,omitempty"`
	ContentPreview string `json:"content_preview,omitempty"`
}

// Apply validates and executes a write-tool request.
func Apply(req Request) (Result, error) {
	nreq, err := normalizeRequest(req)
	if err != nil {
		return Result{}, err
	}

	info, exists, err := statRegularTextFile(nreq.Path)
	if err != nil {
		return Result{}, err
	}

	switch nreq.Operation {
	case "write":
		return applyWrite(nreq, info, exists)
	case "append":
		return applyAppend(nreq, info, exists)
	case "replace":
		return applyReplace(nreq, exists)
	case "edit":
		return applyEdit(nreq, exists)
	default:
		return Result{}, fmt.Errorf("unsupported operation %q", nreq.Operation)
	}
}

// RunHandler exposes the tool through the tool HTTP protocol.
func RunHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		res, err := Apply(req)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"path":      strings.TrimSpace(req.Path),
				"operation": normalizedOperation(req.Operation),
				"is_error":  true,
				"error":     err.Error(),
			})
			return
		}

		_ = json.NewEncoder(w).Encode(resultPayload(res))
	}
}

// Serve starts the write tool server.
func Serve(addr string) error {
	http.HandleFunc("POST /apply", RunHandler())
	fmt.Printf("write tool listening on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}

func resultPayload(res Result) map[string]interface{} {
	payload := map[string]interface{}{
		"path":            res.Path,
		"operation":       res.Operation,
		"created":         res.Created,
		"changed":         res.Changed,
		"bytes_written":   res.BytesWritten,
		"file_size":       res.FileSize,
		"content_preview": res.ContentPreview,
		"is_error":        false,
	}
	if res.Replacements > 0 {
		payload["replacements"] = res.Replacements
	}
	if res.EditsApplied > 0 {
		payload["edits_applied"] = res.EditsApplied
	}
	return payload
}
