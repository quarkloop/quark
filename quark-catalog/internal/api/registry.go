package api

// --- Registry record request/response types ---
//
// The "registry" table holds built-in node descriptors (uri, pattern,
// category, active flag, description). These are different from the
// "node_packages" table which stores pushed .ts/.so payloads — see
// packages.go for those types.

// SaveRegistryRequest upserts a registry record.
type SaveRegistryRequest struct {
	URI         string `json:"uri"`
	Pattern     string `json:"pattern"`
	Category    string `json:"category"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
}

// FindRegistryRequest fetches a single registry record by URI.
type FindRegistryRequest struct {
	URI string `json:"uri"`
}

// RegistryResponse is the JSON shape returned for a single registry row.
type RegistryResponse struct {
	URI         string `json:"uri"`
	Pattern     string `json:"pattern"`
	Category    string `json:"category"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
}

// RegistryListResponse wraps a slice of RegistryResponse.
type RegistryListResponse struct {
	Records []RegistryResponse `json:"records"`
}

// ExistsResponse is the body returned by existence-check endpoints
// (registry.exists, registry.node.exists).
type ExistsResponse struct {
	Exists bool `json:"exists"`
}
