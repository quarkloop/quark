package model

type NamespaceSummary struct {
        Namespace      string `json:"namespace"`
        SystemCount    int    `json:"systemCount"`
        NodeCount      int    `json:"nodeCount"`
        HealthyNodes   int64  `json:"healthyNodes"`
        UnhealthyNodes int64  `json:"unhealthyNodes"`
}

type NamespaceDetail struct {
        Namespace      string         `json:"namespace"`
        SystemCount    int            `json:"systemCount"`
        NodeCount      int            `json:"nodeCount"`
        HealthyNodes   int64          `json:"healthyNodes"`
        UnhealthyNodes int64          `json:"unhealthyNodes"`
        Metrics        NamespaceMetrics `json:"metrics"`
        Systems        []NamespaceSystem `json:"systems"`
}

type NamespaceMetrics struct {
        CPU    NamespaceCPU    `json:"cpu"`
        Memory NamespaceMemory `json:"memory"`
}

type NamespaceCPU struct {
        SystemLoad          float64 `json:"systemLoad"`
        AvailableProcessors int     `json:"availableProcessors"`
}

type NamespaceMemory struct {
        HeapUsed        int64 `json:"heapUsed"`
        HeapMax         int64 `json:"heapMax"`
        HeapCommitted   int64 `json:"heapCommitted"`
        NonHeapUsed     int64 `json:"nonHeapUsed"`
        NonHeapCommitted int64 `json:"nonHeapCommitted"`
}

type NamespaceSystem struct {
        Name     string `json:"name"`
        NodeCount int   `json:"nodeCount"`
        State    string `json:"state"`
        Health   string `json:"health"`
}
