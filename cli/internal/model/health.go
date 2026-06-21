package model

// HealthSummary is the response for platform-wide and per-namespace health.
type HealthSummary struct {
        TotalSystems       int    `json:"totalSystems"`
        TotalNodes     int    `json:"totalNodes"`
        HealthyNodes   int    `json:"healthyNodes"`
        DegradedNodes  int    `json:"degradedNodes"`
        UnhealthyNodes int    `json:"unhealthyNodes"`
        UnknownNodes   int    `json:"unknownNodes"`
        Overall            string `json:"overall"`
}

// SystemHealth is the response for GET /health/systems/{name}.
type SystemHealth struct {
        SystemName  string            `json:"systemName"`
        Namespace     string            `json:"namespace"`
        Overall       string            `json:"overall"`
        NodeCount int               `json:"nodeCount"`
        PerNode   map[string]string `json:"perNode"`
}

// NodeHealth is the response for GET /health/nodes/{name}.
type NodeHealth struct {
        NodeName string  `json:"nodeName"`
        State        string  `json:"state"`
        Health       string  `json:"health"`
        ErrorMessage string  `json:"errorMessage,omitempty"`
        Version      int64   `json:"version"`
        RecentEvents []Event `json:"recentEvents"`
}
