package api

// --- Node package registry request/response types ---
//
// Node packages are pushed .ts/.jar payloads addressed by URI
// (e.g. "quark/time/schedule/timer:v1"). They live in the
// "node_packages" table, separate from the "registry" table which
// only stores built-in descriptors.

// PushNodeRequest uploads a node package. Content is the raw .ts or
// .jar bytes (packaged as a zip containing manifest.json + the build
// output); ContentType is one of "shared-library" | "typescript" |
// "python".
type PushNodeRequest struct {
	URI         string `json:"uri"`
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

// SearchNodesRequest lists packages by optional keyword filter.
type SearchNodesRequest struct {
	Keyword string `json:"keyword"`
}

// NodePackageResponse is the full package body (content included).
type NodePackageResponse struct {
	URI         string `json:"uri"`
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
