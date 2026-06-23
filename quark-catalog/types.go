// Package main — request/response types for NATS communication.
//
// All communication between the Java platform and the Go catalog service
// uses JSON over NATS request-reply. Each request has a subject (e.g.
// "catalog.system.get") and a JSON body. The catalog responds with a JSON
// body on the reply-to subject.
package main

// --- Generic response ---

type ErrorResponse struct {
	Error   string `json:"error,omitempty"`
	Success bool   `json:"success"`
}

// --- System ---

type SaveSystemRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	State     string `json:"state"`
	Health    string `json:"health"`
	Version   int64  `json:"version"`
}

type GetSystemRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type ListSystemsRequest struct {
	Namespace string `json:"namespace"`
}

type DeleteSystemRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type UpdateSystemStateRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Health    string `json:"health"`
	Version   int64  `json:"version"`
}

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

type SystemListResponse struct {
	Systems []SystemResponse `json:"systems"`
}

// --- Node ---

type SaveNodeRequest struct {
	Namespace       string                 `json:"namespace"`
	SystemName      string                 `json:"systemName"`
	Name            string                 `json:"name"`
	URI             string                 `json:"uri"`
	Category        string                 `json:"category"`
	State           string                 `json:"state"`
	Health          string                 `json:"health"`
	Version         int64                  `json:"version"`
	Listens         []string               `json:"listens"`
	Events          []string               `json:"events"`
	Config          map[string]interface{} `json:"config,omitempty"`
	Labels          map[string]string      `json:"labels,omitempty"`
	Annotations     map[string]string      `json:"annotations,omitempty"`
	OnFailureRetry  string                 `json:"onFailureRetry,omitempty"`
	OnFailureRouteTo string                `json:"onFailureRouteTo,omitempty"`
	Timeout         string                 `json:"timeout,omitempty"`
}

type SaveNodesRequest struct {
	Nodes []SaveNodeRequest `json:"nodes"`
}

type ListNodesRequest struct {
	Namespace  string `json:"namespace"`
	SystemName string `json:"systemName"`
}

type DeleteNodesRequest struct {
	Namespace  string `json:"namespace"`
	SystemName string `json:"systemName"`
}

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
	Config           map[string]interface{} `json:"config,omitempty"`
	Labels           map[string]string      `json:"labels,omitempty"`
	Annotations      map[string]string      `json:"annotations,omitempty"`
	OnFailureRetry   string                 `json:"onFailureRetry,omitempty"`
	OnFailureRouteTo string                 `json:"onFailureRouteTo,omitempty"`
	Timeout          string                 `json:"timeout,omitempty"`
	CreatedAt        string                 `json:"createdAt"`
	UpdatedAt        string                 `json:"updatedAt"`
}

type NodeListResponse struct {
	Nodes []NodeResponse `json:"nodes"`
}

// --- Event ---

type AppendEventRequest struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	NodeName  string                 `json:"nodeName"`
	SystemName string                `json:"systemName"`
	Namespace string                 `json:"namespace"`
	Timestamp string                 `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

type AppendEventsRequest struct {
	Events []AppendEventRequest `json:"events"`
}

type QueryEventsRequest struct {
	Namespace  string   `json:"namespace"`
	SystemName string   `json:"systemName"`
	NodeName   string   `json:"nodeName"`
	Kinds      []string `json:"kinds"`
	Limit      int      `json:"limit"`
}

type CountEventsRequest struct {
	Namespace  string   `json:"namespace"`
	SystemName string   `json:"systemName"`
	NodeName   string   `json:"nodeName"`
	Kinds      []string `json:"kinds"`
}

type EventResponse struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"`
	NodeName   string                 `json:"nodeName"`
	SystemName string                 `json:"systemName"`
	Namespace  string                 `json:"namespace"`
	Timestamp  string                 `json:"timestamp"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

type EventListResponse struct {
	Events []EventResponse `json:"events"`
}

type CountResponse struct {
	Count int64 `json:"count"`
}

// --- Source ---

type SaveSourceRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Source    string `json:"source"`
}

type GetSourceRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type SourceEntry struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type SourceListResponse struct {
	Sources []SourceEntry `json:"sources"`
}

type SourceResponse struct {
	Source string `json:"source"`
}

// --- Registry (node packages) ---

type PushNodeRequest struct {
	URI         string `json:"uri"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Manifest    string `json:"manifest"`
	Content     []byte `json:"content"`
	ContentType string `json:"contentType"` // "shared-library" | "typescript" | "python"
}

type PullNodeRequest struct {
	URI string `json:"uri"`
}

type NodeInfoRequest struct {
	URI string `json:"uri"`
}

type SearchNodesRequest struct {
	Keyword  string `json:"keyword"`
	Category string `json:"category"`
}

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

type NodeListResponseReg struct {
	Nodes []NodeInfoResponse `json:"nodes"`
}

type ExistsResponse struct {
	Exists bool `json:"exists"`
}

// --- Registry Record (built-in node registration) ---

type SaveRegistryRequest struct {
	URI         string `json:"uri"`
	Pattern     string `json:"pattern"`
	Category    string `json:"category"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
}

type FindRegistryRequest struct {
	URI string `json:"uri"`
}

type RegistryResponse struct {
	URI         string `json:"uri"`
	Pattern     string `json:"pattern"`
	Category    string `json:"category"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
}

type RegistryListResponse struct {
	Records []RegistryResponse `json:"records"`
}
