package model

import "time"

// DeploySystemRequest is the body of POST /systems/deploy.
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

// SystemSummary is one row in GET /systems.
type SystemSummary struct {
        Name            string    `json:"name"`
        Namespace       string    `json:"namespace"`
        NodeCount       int       `json:"nodeCount"`
        State           string    `json:"state"`
        Health          string    `json:"health"`
        HealthyNodes    int64     `json:"healthyNodes"`
        DegradedNodes   int64     `json:"degradedNodes"`
        UnhealthyNodes  int64     `json:"unhealthyNodes"`
        ConnectionCount int64     `json:"connectionCount"`
        // CreatedAt / UpdatedAt are kept for backward compatibility with
        // older servers that emit them. The current Java server does not.
        CreatedAt time.Time `json:"createdAt,omitempty"`
        UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

// NodeState is one element in SystemDetail.Nodes.
type NodeState struct {
        Name         string    `json:"name"`
        URI          string    `json:"uri"`
        Category     string    `json:"category"`
        State        string    `json:"state"`
        Health       string    `json:"health"`
        CreatedAt    time.Time `json:"createdAt,omitempty"`
        UpdatedAt    time.Time `json:"updatedAt,omitempty"`
        Version      int64     `json:"version"`
        ErrorMessage string    `json:"errorMessage,omitempty"`
}

// SystemDetail is the response for GET /systems/{name}.
type SystemDetail struct {
        Name      string      `json:"name"`
        Namespace string      `json:"namespace"`
        State     string      `json:"state"`
        Health    string      `json:"health"`
        CreatedAt time.Time   `json:"createdAt,omitempty"`
        UpdatedAt time.Time   `json:"updatedAt,omitempty"`
        Version   int64       `json:"version"`
        Nodes     []NodeState `json:"nodes"`
}
