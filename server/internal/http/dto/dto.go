// Package dto holds the request/response DTOs for the HTTP layer.
//
// These types match the JSON shapes the CLI (cli/internal/model/*.go)
// expects. Field tags are tuned for compatibility — same wire format
// as the Java server.
package dto

// --- System DTOs ---

// DeploySystemRequest is the body of POST /api/v1/namespaces/:ns/systems
// and PUT /api/v1/namespaces/:ns/systems/:name.
type DeploySystemRequest struct {
	Source    string `json:"source"`
	Namespace string `json:"namespace,omitempty"`
}

// DeploySystemResponse is returned on successful deploy (HTTP 201).
type DeploySystemResponse struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	NodeCount int      `json:"nodeCount"`
	State     string   `json:"state"`
	Health    string   `json:"health"`
	Nodes     []string `json:"nodes"`
}

// DeploySystemFailure is returned on a parse/validation failure (HTTP 400).
type DeploySystemFailure struct {
	Message string            `json:"message"`
	Errors  []ValidationError `json:"errors"`
}

// ValidationError is one validation error in a DeploySystemFailure.
type ValidationError struct {
	Path     string `json:"path"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// ApplyResult is the response for PUT /systems/:name.
type ApplyResult struct {
	Name      string        `json:"name"`
	Namespace string        `json:"namespace"`
	Created   bool          `json:"created"`
	Changed   bool          `json:"changed"`
	Changes   []ApplyChange `json:"changes"`
}

// ApplyChange represents one change in a declarative apply diff.
type ApplyChange struct {
	Type    string `json:"type"`
	Node    string `json:"node"`
	Details string `json:"details"`
}

// --- Node Registry DTOs (proxied to Catalog) ---
//
// These are intentionally map[string]any — the Go server just proxies
// them to the Catalog without typed decoding. This keeps the wire
// format flexible as the Catalog's package types evolve.

// PushNodeRequest is the body of POST /api/v1/registry/nodes.
// Content is base64-decoded by the handler before forwarding to NATS.
type PushNodeRequest struct {
	URI         string `json:"uri"`
	Version     string `json:"version"`
	Manifest    string `json:"manifest"`
	Content     []byte `json:"content"`
	ContentType string `json:"contentType"`
}

// NodeURIRequest is the body of POST /api/v1/registry/nodes/info
// and /api/v1/registry/nodes/pull.
type NodeURIRequest struct {
	URI string `json:"uri"`
}
