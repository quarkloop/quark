package api

// --- Node package registry request/response types ---
//
// Node packages are pushed .ts/.so payloads addressed by URI
// (e.g. "source/timer:v1"). They live in the "node_packages" table,
// separate from the "registry" table which only stores built-in
// descriptors.

// PushNodeRequest uploads a node package. Content is the raw .ts or
// .so bytes; ContentType is one of "shared-library" | "typescript" |
// "python".
type PushNodeRequest struct {
	URI         string `json:"uri"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Manifest    string `json:"manifest"`
	Content     []byte `json:"content"`
	ContentType string `json:"contentType"`
}

// PullNodeRequest fetches the full package (with content) by URI.
type PullNodeRequest struct {
	URI string `json:"uri"`
}

// NodeInfoRequest fetches package metadata (no content) by URI.
type NodeInfoRequest struct {
	URI string `json:"uri"`
}

// SearchNodesRequest lists packages by optional keyword/category filter.
// Used by both registry.node.list (keyword empty) and registry.node.search.
type SearchNodesRequest struct {
	Keyword  string `json:"keyword"`
	Category string `json:"category"`
}

// NodePackageResponse is the full package body (content included).
type NodePackageResponse struct {
	URI         string `json:"uri"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Manifest    string `json:"manifest"`
	Content     []byte `json:"content"`
	ContentType string `json:"contentType"`
	Checksum    string `json:"checksum"`
	CreatedAt   string `json:"createdAt"`
	Downloads   int64  `json:"downloads"`
}

// NodeInfoResponse is package metadata without the (potentially large)
// content blob.
type NodeInfoResponse struct {
	URI         string `json:"uri"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Manifest    string `json:"manifest"`
	ContentType string `json:"contentType"`
	Checksum    string `json:"checksum"`
	CreatedAt   string `json:"createdAt"`
	Downloads   int64  `json:"downloads"`
}

// NodePackageListResponse wraps a slice of NodeInfoResponse.
type NodePackageListResponse struct {
	Nodes []NodeInfoResponse `json:"nodes"`
}
