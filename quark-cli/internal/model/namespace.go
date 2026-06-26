package model

type NamespaceSummary struct {
	Namespace      string `json:"namespace"`
	SystemCount    int    `json:"systemCount"`
	NodeCount      int    `json:"nodeCount"`
	HealthyNodes   int64  `json:"healthyNodes"`
	UnhealthyNodes int64  `json:"unhealthyNodes"`
}

type NamespaceDetail struct {
	Namespace      string           `json:"namespace"`
	SystemCount    int              `json:"systemCount"`
	NodeCount      int              `json:"nodeCount"`
	HealthyNodes   int64            `json:"healthyNodes"`
	UnhealthyNodes int64            `json:"unhealthyNodes"`
	Metrics        NamespaceMetrics `json:"metrics"`
	Systems        []NamespaceSystem `json:"systems"`
}

type NamespaceMetrics struct {
	CPU        NamespaceCPU        `json:"cpu"`
	Throughput NamespaceThroughput `json:"throughput"`
	Memory     NamespaceMemory     `json:"memory"`
}

type NamespaceCPU struct {
	NamespacePercent    float64 `json:"namespacePercent"`
	AvailableProcessors int     `json:"availableProcessors"`
}

type NamespaceThroughput struct {
	MessagesPublishedPerSec float64 `json:"messagesPublishedPerSec"`
	MessagesReceivedPerSec  float64 `json:"messagesReceivedPerSec"`
	ErrorsPerSec            float64 `json:"errorsPerSec"`
	TotalPublished          int64   `json:"totalPublished"`
	TotalReceived           int64   `json:"totalReceived"`
	TotalErrors             int64   `json:"totalErrors"`
	CPUTimeNanos            int64   `json:"cpuTimeNanos"`
}

type NamespaceMemory struct {
	HeapUsed         int64 `json:"heapUsed"`
	HeapMax          int64 `json:"heapMax"`
	HeapCommitted    int64 `json:"heapCommitted"`
	NonHeapUsed      int64 `json:"nonHeapUsed"`
	NonHeapCommitted int64 `json:"nonHeapCommitted"`
}

type NamespaceSystem struct {
	Name      string `json:"name"`
	NodeCount int    `json:"nodeCount"`
	State     string `json:"state"`
	Health    string `json:"health"`
}
