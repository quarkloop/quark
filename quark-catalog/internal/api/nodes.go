package api

// --- Node request/response types ---

// SaveNodeRequest upserts a node row.
type SaveNodeRequest struct {
	Namespace        string                 `json:"namespace"`
	SystemName       string                 `json:"systemName"`
	Name             string                 `json:"name"`
	URI              string                 `json:"uri"`
	Category         string                 `json:"category"`
	State            string                 `json:"state"`
	Health           string                 `json:"health"`
	Version          int64                  `json:"version"`
	Listens          []string               `json:"listens"`
	Events           []string               `json:"events"`
	Config           map[string]any         `json:"config,omitempty"`
	Labels           map[string]string      `json:"labels,omitempty"`
	Annotations      map[string]string      `json:"annotations,omitempty"`
	OnFailureRetry   string                 `json:"onFailureRetry,omitempty"`
	OnFailureRouteTo string                 `json:"onFailureRouteTo,omitempty"`
	Timeout          string                 `json:"timeout,omitempty"`
}

// SaveNodesRequest batches a SaveNode operation across many nodes.
type SaveNodesRequest struct {
	Nodes []SaveNodeRequest `json:"nodes"`
}

// ListNodesRequest lists nodes. If SystemName is empty, all nodes for
// the namespace are returned; otherwise only nodes for that system.
type ListNodesRequest struct {
	Namespace  string `json:"namespace"`
	SystemName string `json:"systemName"`
}

// DeleteNodesRequest removes all nodes belonging to (namespace, system).
type DeleteNodesRequest struct {
	Namespace  string `json:"namespace"`
	SystemName string `json:"systemName"`
}

// NodeResponse is the JSON shape returned for a single node row.
type NodeResponse struct {
	Namespace        string                 `json:"namespace"`
	SystemName       string                 `json:"systemName"`
	Name             string                 `json:"name"`
	URI              string                 `json:"uri"`
	Category         string                 `json:"category"`
	State            string                 `json:"state"`
	Health           string                 `json:"health"`
	Version          int64                  `json:"version"`
	ErrorMessage     string                 `json:"errorMessage"`
	Listens          []string               `json:"listens"`
	Events           []string               `json:"events"`
	Config           map[string]any         `json:"config,omitempty"`
	Labels           map[string]string      `json:"labels,omitempty"`
	Annotations      map[string]string      `json:"annotations,omitempty"`
	OnFailureRetry   string                 `json:"onFailureRetry,omitempty"`
	OnFailureRouteTo string                 `json:"onFailureRouteTo,omitempty"`
	Timeout          string                 `json:"timeout,omitempty"`
	CreatedAt        string                 `json:"createdAt"`
	UpdatedAt        string                 `json:"updatedAt"`
}

// NodeListResponse wraps a slice of NodeResponse.
type NodeListResponse struct {
	Nodes []NodeResponse `json:"nodes"`
}
