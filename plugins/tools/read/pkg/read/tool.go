// Package read implements the quark read tool.
package read

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const contentPreviewLimit = 512

// Request describes one read-tool invocation.
type Request struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

// Result describes file content returned by the read tool.
type Result struct {
	Path           string `json:"path"`
	Content        string `json:"content"`
	ContentPreview string `json:"content_preview,omitempty"`
	FileSize       int    `json:"file_size"`
	BytesRead      int    `json:"bytes_read"`
	TotalLines     int    `json:"total_lines"`
	StartLine      int    `json:"start_line"`
	EndLine        int    `json:"end_line"`
}

// Apply validates and executes a read-tool request.
func Apply(req Request) (Result, error) {
	nreq, err := normalizeRequest(req)
	if err != nil {
		return Result{}, err
	}

	content, err := loadRegularTextFile(nreq.Path)
	if err != nil {
		return Result{}, err
	}

	snippet, startLine, endLine, totalLines, err := sliceContent(content, nreq)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Path:           nreq.Path,
		Content:        snippet,
		ContentPreview: truncatePreview(snippet, contentPreviewLimit),
		FileSize:       len(content),
		BytesRead:      len(snippet),
		TotalLines:     totalLines,
		StartLine:      startLine,
		EndLine:        endLine,
	}, nil
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
				"path":     strings.TrimSpace(req.Path),
				"is_error": true,
				"error":    err.Error(),
			})
			return
		}

		_ = json.NewEncoder(w).Encode(resultPayload(res))
	}
}

// Serve starts the read tool server.
func Serve(addr string) error {
	http.HandleFunc("POST /read", RunHandler())
	fmt.Printf("read tool listening on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}

func resultPayload(res Result) map[string]interface{} {
	return map[string]interface{}{
		"path":            res.Path,
		"content":         res.Content,
		"content_preview": res.ContentPreview,
		"file_size":       res.FileSize,
		"bytes_read":      res.BytesRead,
		"total_lines":     res.TotalLines,
		"start_line":      res.StartLine,
		"end_line":        res.EndLine,
		"is_error":        false,
	}
}

func truncatePreview(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
