package api

// --- System request/response types ---

// SaveSystemRequest upserts a system row.
type SaveSystemRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	State     string `json:"state"`
	Health    string `json:"health"`
	Version   int64  `json:"version"`
}

// GetSystemRequest fetches a single system by (namespace, name).
type GetSystemRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// ListSystemsRequest lists systems. An empty Namespace returns all
// systems across every namespace.
type ListSystemsRequest struct {
	Namespace string `json:"namespace"`
}

// DeleteSystemRequest removes a system row.
type DeleteSystemRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// UpdateSystemStateRequest updates the state/health/version columns
// of an existing system row.
type UpdateSystemStateRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Health    string `json:"health"`
	Version   int64  `json:"version"`
}

// SystemResponse is the JSON shape returned for a single system row.
type SystemResponse struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	State     string `json:"state"`
	Health    string `json:"health"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// SystemListResponse wraps a slice of SystemResponse.
type SystemListResponse struct {
	Systems []SystemResponse `json:"systems"`
}
