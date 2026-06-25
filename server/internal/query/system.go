// Package query is the read-side of the Go control plane.
//
// All query services are read-only — they hit the Catalog via the
// store.*Repository interfaces. There is no in-memory cache; every
// call goes straight to NATS. The Catalog's own response time is
// ~1ms (local SQLite), so caching is unnecessary for the load levels
// this server is designed for.
package query

import (
	"context"
	"sort"

	"github.com/quarkloop/quark/server/internal/domain"
	"github.com/quarkloop/quark/server/internal/store"
)

// SystemQueryService backs GET /api/v1/namespaces/:ns/systems and
// GET /api/v1/namespaces/:ns/systems/:name.
type SystemQueryService struct {
	sysRepo  store.SystemRepository
	nodeRepo store.NodeRepository
}

// NewSystemQueryService constructs a SystemQueryService.
func NewSystemQueryService(sysRepo store.SystemRepository, nodeRepo store.NodeRepository) *SystemQueryService {
	return &SystemQueryService{sysRepo: sysRepo, nodeRepo: nodeRepo}
}

// SystemSummary is the listing row shape. Mirrors cli/internal/model.SystemSummary.
type SystemSummary struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	NodeCount       int    `json:"nodeCount"`
	State           string `json:"state"`
	Health          string `json:"health"`
	HealthyNodes    int64  `json:"healthyNodes"`
	DegradedNodes   int64  `json:"degradedNodes"`
	UnhealthyNodes  int64  `json:"unhealthyNodes"`
	ConnectionCount int64  `json:"connectionCount"`
}

// ListSystems returns all systems in a namespace, sorted by name.
func (s *SystemQueryService) ListSystems(ctx context.Context, namespace string) ([]*SystemSummary, error) {
	sysRecs, err := s.sysRepo.ListSystems(ctx, namespace)
	if err != nil {
		return nil, err
	}
	out := make([]*SystemSummary, 0, len(sysRecs))
	for _, sr := range sysRecs {
		nodes, _ := s.nodeRepo.ListNodesBySystem(ctx, namespace, sr.Name)
		var healthy int64
		for _, n := range nodes {
			if n.Health == domain.HealthHealthy {
				healthy++
			}
		}
		out = append(out, &SystemSummary{
			Name:            sr.Name,
			Namespace:       sr.Namespace,
			NodeCount:       len(nodes),
			State:           sr.State,
			Health:          sr.Health,
			HealthyNodes:    healthy,
			UnhealthyNodes:  int64(len(nodes)) - healthy,
			ConnectionCount: int64(len(nodes)),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// SystemDetail is the single-system response shape. Mirrors
// cli/internal/model.SystemDetail.
type SystemDetail struct {
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	State     string         `json:"state"`
	Health    string         `json:"health"`
	Version   int64          `json:"version"`
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
	Nodes     []*NodeSummary `json:"nodes"`
}

// GetSystem returns details for one system, including its nodes.
func (s *SystemQueryService) GetSystem(ctx context.Context, namespace, name string) (*SystemDetail, error) {
	sr, err := s.sysRepo.GetSystem(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	nodeSvc := NewNodeQueryService(s.nodeRepo)
	nodes, _ := nodeSvc.ListNodes(ctx, namespace, name)
	return &SystemDetail{
		Name:      sr.Name,
		Namespace: sr.Namespace,
		State:     sr.State,
		Health:    sr.Health,
		Version:   sr.Version,
		CreatedAt: sr.CreatedAt,
		UpdatedAt: sr.UpdatedAt,
		Nodes:     nodes,
	}, nil
}

// --- Node queries ---

// NodeQueryService backs GET /api/v1/namespaces/:ns/systems/:sys/nodes
// and GET /api/v1/namespaces/:ns/systems/:sys/nodes/:name.
type NodeQueryService struct {
	nodeRepo store.NodeRepository
}

// NewNodeQueryService constructs a NodeQueryService.
func NewNodeQueryService(nodeRepo store.NodeRepository) *NodeQueryService {
	return &NodeQueryService{nodeRepo: nodeRepo}
}

// NodeSummary is the listing row shape. Mirrors cli/internal/model.NodeSummary.
type NodeSummary struct {
	Name       string `json:"name"`
	SystemName string `json:"systemName"`
	Namespace  string `json:"namespace"`
	URI        string `json:"uri"`
	State      string `json:"state"`
	Health     string `json:"health"`
	Version    int64  `json:"version"`
}

// ListNodes returns all nodes in a system, sorted by name.
func (s *NodeQueryService) ListNodes(ctx context.Context, namespace, system string) ([]*NodeSummary, error) {
	recs, err := s.nodeRepo.ListNodesBySystem(ctx, namespace, system)
	if err != nil {
		return nil, err
	}
	out := make([]*NodeSummary, 0, len(recs))
	for _, n := range recs {
		out = append(out, &NodeSummary{
			Name:       n.Name,
			SystemName: n.SystemName,
			Namespace:  n.Namespace,
			URI:        n.URI,
			State:      n.State,
			Health:     n.Health,
			Version:    n.Version,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// NodeDetail is the single-node response shape. Mirrors
// cli/internal/model.NodeDetail.
type NodeDetail struct {
	Name         string                 `json:"name"`
	SystemName   string                 `json:"systemName"`
	Namespace    string                 `json:"namespace"`
	URI          string                 `json:"uri"`
	State        string                 `json:"state"`
	Health       string                 `json:"health"`
	Version      int64                  `json:"version"`
	ErrorMessage string                 `json:"errorMessage,omitempty"`
	CreatedAt    string                 `json:"createdAt"`
	UpdatedAt    string                 `json:"updatedAt"`
	Config       map[string]any         `json:"config"`
	Labels       map[string]string      `json:"labels"`
	Annotations  map[string]string      `json:"annotations"`
	Listens      []string               `json:"listens"`
	Events       []string               `json:"events"`
}

// GetNode returns details for one node.
func (s *NodeQueryService) GetNode(ctx context.Context, namespace, system, name string) (*NodeDetail, error) {
	rec, err := s.nodeRepo.FindNode(ctx, namespace, system, name)
	if err != nil {
		return nil, err
	}
	return &NodeDetail{
		Name:         rec.Name,
		SystemName:   rec.SystemName,
		Namespace:    rec.Namespace,
		URI:          rec.URI,
		State:        rec.State,
		Health:       rec.Health,
		Version:      rec.Version,
		ErrorMessage: rec.ErrorMessage,
		CreatedAt:    rec.CreatedAt,
		UpdatedAt:    rec.UpdatedAt,
		Config:       rec.Config,
		Labels:       rec.Labels,
		Annotations:  rec.Annotations,
		Listens:      rec.Listens,
		Events:       rec.Events,
	}, nil
}

// --- Namespace queries ---

// NamespaceQueryService backs GET /api/v1/namespaces and
// GET /api/v1/namespaces/:ns. It does not have its own repository —
// it composes from SystemRepository + NodeRepository + the metrics
// collector.
type NamespaceQueryService struct {
	sysRepo    store.SystemRepository
	nodeRepo   store.NodeRepository
	metrics    MetricsProvider
}

// MetricsProvider is the subset of *metrics.Collector that
// NamespaceQueryService uses. Declared here so tests can stub it.
type MetricsProvider interface {
	GetRate(namespace string) *RateSnapshot
}

// RateSnapshot is a copy of metrics.Rate (declared here to avoid
// an import cycle: query -> metrics, but we want query to be a leaf).
type RateSnapshot struct {
	MessagesPublished int64
	MessagesReceived  int64
	Errors            int64
	CPUTimeNanos      int64
	PublishRate       float64
	ReceiveRate       float64
	ErrorRate         float64
	CPUPercent        float64
}

// NewNamespaceQueryService constructs a NamespaceQueryService.
func NewNamespaceQueryService(sysRepo store.SystemRepository, nodeRepo store.NodeRepository, metrics MetricsProvider) *NamespaceQueryService {
	return &NamespaceQueryService{sysRepo: sysRepo, nodeRepo: nodeRepo, metrics: metrics}
}

// NamespaceSummary is the listing row shape. Mirrors
// cli/internal/model.NamespaceSummary.
type NamespaceSummary struct {
	Namespace      string `json:"namespace"`
	SystemCount    int    `json:"systemCount"`
	NodeCount      int    `json:"nodeCount"`
	HealthyNodes   int64  `json:"healthyNodes"`
	UnhealthyNodes int64  `json:"unhealthyNodes"`
}

// ListNamespaces returns all active namespaces (those that have at
// least one system in the Catalog).
func (s *NamespaceQueryService) ListNamespaces(ctx context.Context) ([]*NamespaceSummary, error) {
	sysRecs, err := s.sysRepo.ListAllSystems(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]*NamespaceSummary)
	for _, sr := range sysRecs {
		ns := sr.Namespace
		if _, ok := seen[ns]; !ok {
			seen[ns] = &NamespaceSummary{Namespace: ns}
		}
		seen[ns].SystemCount++
		nodes, _ := s.nodeRepo.ListNodesBySystem(ctx, ns, sr.Name)
		seen[ns].NodeCount += len(nodes)
		for _, n := range nodes {
			if n.Health == domain.HealthHealthy {
				seen[ns].HealthyNodes++
			} else {
				seen[ns].UnhealthyNodes++
			}
		}
	}
	out := make([]*NamespaceSummary, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Namespace < out[j].Namespace })
	return out, nil
}

// NamespaceDetail is the single-namespace response shape. Mirrors
// cli/internal/model.NamespaceDetail (without the JVM memory metrics,
// which the Go server doesn't have).
type NamespaceDetail struct {
	Namespace      string             `json:"namespace"`
	SystemCount    int                `json:"systemCount"`
	NodeCount      int                `json:"nodeCount"`
	HealthyNodes   int64              `json:"healthyNodes"`
	UnhealthyNodes int64              `json:"unhealthyNodes"`
	Metrics        NamespaceMetrics   `json:"metrics"`
	Systems        []NamespaceSystem  `json:"systems"`
}

// NamespaceMetrics is the metrics block. The CPU and throughput
// fields come from the metrics collector; memory fields are zero
// (the Go server has no JVM heap to report).
type NamespaceMetrics struct {
	CPU        NamespaceCPU        `json:"cpu"`
	Throughput NamespaceThroughput `json:"throughput"`
	Memory     NamespaceMemory     `json:"memory"`
}

// NamespaceCPU mirrors cli/internal/model.NamespaceCPU.
type NamespaceCPU struct {
	NamespacePercent    float64 `json:"namespacePercent"`
	AvailableProcessors int     `json:"availableProcessors"`
}

// NamespaceThroughput mirrors cli/internal/model.NamespaceThroughput.
type NamespaceThroughput struct {
	MessagesPublishedPerSec float64 `json:"messagesPublishedPerSec"`
	MessagesReceivedPerSec  float64 `json:"messagesReceivedPerSec"`
	ErrorsPerSec            float64 `json:"errorsPerSec"`
	TotalPublished          int64   `json:"totalPublished"`
	TotalReceived           int64   `json:"totalReceived"`
	TotalErrors             int64   `json:"totalErrors"`
	CPUTimeNanos            int64   `json:"cpuTimeNanos"`
}

// NamespaceMemory is zeroed in the Go server (no JVM heap).
type NamespaceMemory struct {
	HeapUsed         int64 `json:"heapUsed"`
	HeapMax          int64 `json:"heapMax"`
	HeapCommitted    int64 `json:"heapCommitted"`
	NonHeapUsed      int64 `json:"nonHeapUsed"`
	NonHeapCommitted int64 `json:"nonHeapCommitted"`
}

// NamespaceSystem is one entry in NamespaceDetail.Systems.
type NamespaceSystem struct {
	Name      string `json:"name"`
	NodeCount int    `json:"nodeCount"`
	State     string `json:"state"`
	Health    string `json:"health"`
}

// GetNamespace returns details for one namespace.
func (s *NamespaceQueryService) GetNamespace(ctx context.Context, namespace string) (*NamespaceDetail, error) {
	sysRecs, err := s.sysRepo.ListSystems(ctx, namespace)
	if err != nil {
		return nil, err
	}
	if len(sysRecs) == 0 {
		return nil, ErrNotFound
	}

	allNodes, _ := s.nodeRepo.ListNodesByNamespace(ctx, namespace)
	var healthy, unhealthy int64
	for _, n := range allNodes {
		if n.Health == domain.HealthHealthy {
			healthy++
		} else {
			unhealthy++
		}
	}

	systems := make([]NamespaceSystem, 0, len(sysRecs))
	for _, sr := range sysRecs {
		nodes, _ := s.nodeRepo.ListNodesBySystem(ctx, namespace, sr.Name)
		systems = append(systems, NamespaceSystem{
			Name:      sr.Name,
			NodeCount: len(nodes),
			State:     sr.State,
			Health:    sr.Health,
		})
	}

	var rate *RateSnapshot
	if s.metrics != nil {
		rate = s.metrics.GetRate(namespace)
	}
	detail := &NamespaceDetail{
		Namespace:      namespace,
		SystemCount:    len(sysRecs),
		NodeCount:      len(allNodes),
		HealthyNodes:   healthy,
		UnhealthyNodes: unhealthy,
		Systems:        systems,
		Metrics: NamespaceMetrics{
			CPU: NamespaceCPU{
				AvailableProcessors: availableProcessors(),
			},
		},
	}
	if rate != nil {
		detail.Metrics.CPU.NamespacePercent = rate.CPUPercent
		detail.Metrics.Throughput = NamespaceThroughput{
			MessagesPublishedPerSec: rate.PublishRate,
			MessagesReceivedPerSec:  rate.ReceiveRate,
			ErrorsPerSec:            rate.ErrorRate,
			TotalPublished:          rate.MessagesPublished,
			TotalReceived:           rate.MessagesReceived,
			TotalErrors:             rate.Errors,
			CPUTimeNanos:            rate.CPUTimeNanos,
		}
	}
	return detail, nil
}
