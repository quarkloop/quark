// Package store — NATS-based implementation of all 5 repository
// interfaces.
//
// NatsCatalogClient sends JSON requests to catalog.* NATS subjects and
// decodes JSON responses into domain types. It implements:
//   - SystemRepository
//   - NodeRepository
//   - EventStore
//   - SourceRepository
//   - RegistryRepository
//
// Same wire format as the Java NatsCatalogClient — the Catalog (Go)
// doesn't know or care whether the caller is Java or Go.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/quarkloop/quark/server/internal/domain"
)

// natsRequestTimeout is the default timeout for catalog.* requests.
// 5s matches the Java side and the Catalog's own processing budget.
const natsRequestTimeout = 5 * time.Second

// NatsCatalogClient implements all 5 repository interfaces via NATS
// request-reply to the Catalog service.
type NatsCatalogClient struct {
	nc *nats.Conn
}

// NewNatsCatalogClient constructs a NatsCatalogClient over the given
// NATS connection. The connection must already be established.
func NewNatsCatalogClient(nc *nats.Conn) *NatsCatalogClient {
	return &NatsCatalogClient{nc: nc}
}

// natsRequest sends a JSON request to subject, waits up to 5s for a
// reply, and returns the reply payload. Errors are wrapped with the
// subject name for diagnostics.
func (c *NatsCatalogClient) natsRequest(ctx context.Context, subject string, req any) ([]byte, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request for %s: %w", subject, err)
	}
	reply, err := c.nc.RequestWithContext(ctx, subject, payload)
	if err != nil {
		return nil, fmt.Errorf("nats request %s: %w", subject, err)
	}
	return reply.Data, nil
}

// decodeError checks if a JSON response body represents a Catalog
// error envelope ({ "success": false, "error": "..." }) and returns
// the error message. Returns "" for success responses.
func decodeError(body []byte) string {
	var env struct {
		Error   string `json:"error"`
		Success bool   `json:"success"`
	}
	if json.Unmarshal(body, &env) == nil && !env.Success && env.Error != "" {
		return env.Error
	}
	return ""
}

// =========================================================================
// SystemRepository
// =========================================================================

type saveSystemReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	State     string `json:"state"`
	Health    string `json:"health"`
	Version   int64  `json:"version"`
}

func (c *NatsCatalogClient) SaveSystem(ctx context.Context, rec *domain.SystemRecord) error {
	req := saveSystemReq{
		Namespace: rec.Namespace,
		Name:      rec.Name,
		Source:    rec.Source,
		State:     rec.State,
		Health:    rec.Health,
		Version:   rec.Version,
	}
	_, err := c.natsRequest(ctx, "catalog.system.save", req)
	return err
}

type getSystemReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (c *NatsCatalogClient) GetSystem(ctx context.Context, namespace, name string) (*domain.SystemRecord, error) {
	body, err := c.natsRequest(ctx, "catalog.system.get", getSystemReq{Namespace: namespace, Name: name})
	if err != nil {
		return nil, err
	}
	if msg := decodeError(body); msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	var rec domain.SystemRecord
	if err := json.Unmarshal(body, &rec); err != nil {
		return nil, fmt.Errorf("decode system response: %w", err)
	}
	return &rec, nil
}

type listSystemsReq struct {
	Namespace string `json:"namespace"`
}

type systemListResp struct {
	Systems []domain.SystemRecord `json:"systems"`
}

func (c *NatsCatalogClient) ListSystems(ctx context.Context, namespace string) ([]*domain.SystemRecord, error) {
	body, err := c.natsRequest(ctx, "catalog.system.list", listSystemsReq{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	var resp systemListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode system list: %w", err)
	}
	out := make([]*domain.SystemRecord, 0, len(resp.Systems))
	for i := range resp.Systems {
		out = append(out, &resp.Systems[i])
	}
	return out, nil
}

func (c *NatsCatalogClient) ListAllSystems(ctx context.Context) ([]*domain.SystemRecord, error) {
	return c.ListSystems(ctx, "")
}

type deleteSystemReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (c *NatsCatalogClient) DeleteSystem(ctx context.Context, namespace, name string) error {
	_, err := c.natsRequest(ctx, "catalog.system.delete", deleteSystemReq{Namespace: namespace, Name: name})
	return err
}

type updateSystemStateReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Health    string `json:"health"`
	Version   int64  `json:"version"`
}

func (c *NatsCatalogClient) UpdateSystemState(ctx context.Context, namespace, name, state, health string, version int64) error {
	_, err := c.natsRequest(ctx, "catalog.system.updateState", updateSystemStateReq{
		Namespace: namespace, Name: name, State: state, Health: health, Version: version,
	})
	return err
}

// =========================================================================
// NodeRepository
// =========================================================================

type saveNodeReq struct {
	Namespace        string            `json:"namespace"`
	SystemName       string            `json:"systemName"`
	Name             string            `json:"name"`
	URI              string            `json:"uri"`
	State            string            `json:"state"`
	Health           string            `json:"health"`
	Version          int64             `json:"version"`
	Listens          []string          `json:"listens"`
	Events           []string          `json:"events"`
	Config           map[string]any    `json:"config,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	Annotations      map[string]string `json:"annotations,omitempty"`
	OnFailureRetry   string            `json:"onFailureRetry,omitempty"`
	OnFailureRouteTo string            `json:"onFailureRouteTo,omitempty"`
	Timeout          string            `json:"timeout,omitempty"`
}

func nodeToSaveReq(n *domain.NodeRecord) saveNodeReq {
	return saveNodeReq{
		Namespace:        n.Namespace,
		SystemName:       n.SystemName,
		Name:             n.Name,
		URI:              n.URI,
		State:            n.State,
		Health:           n.Health,
		Version:          n.Version,
		Listens:          n.Listens,
		Events:           n.Events,
		Config:           n.Config,
		Labels:           n.Labels,
		Annotations:      n.Annotations,
		OnFailureRetry:   n.OnFailureRetry,
		OnFailureRouteTo: n.OnFailureRouteTo,
		Timeout:          n.Timeout,
	}
}

func (c *NatsCatalogClient) SaveNode(ctx context.Context, rec *domain.NodeRecord) error {
	_, err := c.natsRequest(ctx, "catalog.node.save", nodeToSaveReq(rec))
	return err
}

type saveNodesReq struct {
	Nodes []saveNodeReq `json:"nodes"`
}

func (c *NatsCatalogClient) SaveNodes(ctx context.Context, recs []*domain.NodeRecord) error {
	reqs := make([]saveNodeReq, 0, len(recs))
	for _, n := range recs {
		reqs = append(reqs, nodeToSaveReq(n))
	}
	_, err := c.natsRequest(ctx, "catalog.node.saveAll", saveNodesReq{Nodes: reqs})
	return err
}

type listNodesReq struct {
	Namespace  string `json:"namespace"`
	SystemName string `json:"systemName"`
}

type nodeListResp struct {
	Nodes []domain.NodeRecord `json:"nodes"`
}

func (c *NatsCatalogClient) ListNodesBySystem(ctx context.Context, namespace, system string) ([]*domain.NodeRecord, error) {
	body, err := c.natsRequest(ctx, "catalog.node.list", listNodesReq{Namespace: namespace, SystemName: system})
	if err != nil {
		return nil, err
	}
	var resp nodeListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode node list: %w", err)
	}
	out := make([]*domain.NodeRecord, 0, len(resp.Nodes))
	for i := range resp.Nodes {
		out = append(out, &resp.Nodes[i])
	}
	return out, nil
}

func (c *NatsCatalogClient) ListNodesByNamespace(ctx context.Context, namespace string) ([]*domain.NodeRecord, error) {
	body, err := c.natsRequest(ctx, "catalog.node.list", listNodesReq{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	var resp nodeListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode node list: %w", err)
	}
	out := make([]*domain.NodeRecord, 0, len(resp.Nodes))
	for i := range resp.Nodes {
		out = append(out, &resp.Nodes[i])
	}
	return out, nil
}

func (c *NatsCatalogClient) FindNode(ctx context.Context, namespace, system, name string) (*domain.NodeRecord, error) {
	nodes, err := c.ListNodesBySystem(ctx, namespace, system)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.Name == name {
			return n, nil
		}
	}
	return nil, fmt.Errorf("node not found: %s/%s/%s", namespace, system, name)
}

type deleteNodesReq struct {
	Namespace  string `json:"namespace"`
	SystemName string `json:"systemName"`
}

func (c *NatsCatalogClient) DeleteNodesBySystem(ctx context.Context, namespace, system string) error {
	_, err := c.natsRequest(ctx, "catalog.node.delete", deleteNodesReq{Namespace: namespace, SystemName: system})
	return err
}

func (c *NatsCatalogClient) UpdateNodeState(ctx context.Context, namespace, system, name, state, health string, version int64, errMsg string) error {
	// Catalog doesn't have a per-node update-state subject; fetch full node,
	// mutate state/health/error/version, then SaveNode. Same pattern as
	// the Java NatsCatalogClient.
	rec, err := c.FindNode(ctx, namespace, system, name)
	if err != nil {
		return err
	}
	rec.State = state
	rec.Health = health
	rec.Version = version
	rec.ErrorMessage = errMsg
	rec.UpdatedAt = domain.Now()
	return c.SaveNode(ctx, rec)
}

// =========================================================================
// EventStore
// =========================================================================

type appendEventReq struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	NodeName   string         `json:"nodeName"`
	SystemName string         `json:"systemName"`
	Namespace  string         `json:"namespace"`
	Timestamp  string         `json:"timestamp"`
	Payload    map[string]any `json:"payload,omitempty"`
}

func (c *NatsCatalogClient) AppendEvent(ctx context.Context, e *domain.NodeEvent) error {
	req := appendEventReq{
		ID: e.ID, Kind: e.Kind, NodeName: e.NodeName,
		SystemName: e.SystemName, Namespace: e.Namespace,
		Timestamp: e.TimestampString(), Payload: e.Payload,
	}
	_, err := c.natsRequest(ctx, "catalog.event.append", req)
	return err
}

type appendEventsReq struct {
	Events []appendEventReq `json:"events"`
}

func (c *NatsCatalogClient) AppendEvents(ctx context.Context, events []*domain.NodeEvent) error {
	if len(events) == 0 {
		return nil
	}
	reqs := make([]appendEventReq, 0, len(events))
	for _, e := range events {
		reqs = append(reqs, appendEventReq{
			ID: e.ID, Kind: e.Kind, NodeName: e.NodeName,
			SystemName: e.SystemName, Namespace: e.Namespace,
			Timestamp: e.TimestampString(), Payload: e.Payload,
		})
	}
	_, err := c.natsRequest(ctx, "catalog.event.appendBatch", appendEventsReq{Events: reqs})
	return err
}

type queryEventsReq struct {
	Namespace  string   `json:"namespace"`
	SystemName string   `json:"systemName"`
	NodeName   string   `json:"nodeName"`
	Kinds      []string `json:"kinds"`
	Limit      int      `json:"limit"`
}

type eventListResp struct {
	Events []domain.NodeEvent `json:"events"`
}

func (c *NatsCatalogClient) QueryEvents(ctx context.Context, filter *EventFilter) ([]*domain.NodeEvent, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	req := queryEventsReq{
		Namespace:  filter.Namespace,
		SystemName: filter.SystemName,
		NodeName:   filter.NodeName,
		Kinds:      filter.Kinds,
		Limit:      limit,
	}
	body, err := c.natsRequest(ctx, "catalog.event.query", req)
	if err != nil {
		return nil, err
	}
	var resp eventListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode event list: %w", err)
	}
	out := make([]*domain.NodeEvent, 0, len(resp.Events))
	for i := range resp.Events {
		out = append(out, &resp.Events[i])
	}
	return out, nil
}

type countEventsReq struct {
	Namespace  string   `json:"namespace"`
	SystemName string   `json:"systemName"`
	NodeName   string   `json:"nodeName"`
	Kinds      []string `json:"kinds"`
}

type countResp struct {
	Count int64 `json:"count"`
}

func (c *NatsCatalogClient) CountEvents(ctx context.Context, filter *EventFilter) (int64, error) {
	req := countEventsReq{
		Namespace:  filter.Namespace,
		SystemName: filter.SystemName,
		NodeName:   filter.NodeName,
		Kinds:      filter.Kinds,
	}
	body, err := c.natsRequest(ctx, "catalog.event.count", req)
	if err != nil {
		return 0, err
	}
	var resp countResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("decode count: %w", err)
	}
	return resp.Count, nil
}

// =========================================================================
// SourceRepository
// =========================================================================

type saveSourceReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Source    string `json:"source"`
}

func (c *NatsCatalogClient) SaveSource(ctx context.Context, namespace, name, source string) error {
	_, err := c.natsRequest(ctx, "catalog.source.save", saveSourceReq{Namespace: namespace, Name: name, Source: source})
	return err
}

type getSourceReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type sourceResp struct {
	Source string `json:"source"`
}

func (c *NatsCatalogClient) GetSource(ctx context.Context, namespace, name string) (string, error) {
	body, err := c.natsRequest(ctx, "catalog.source.get", getSourceReq{Namespace: namespace, Name: name})
	if err != nil {
		return "", err
	}
	if msg := decodeError(body); msg != "" {
		return "", fmt.Errorf("%s", msg)
	}
	var resp sourceResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decode source response: %w", err)
	}
	return resp.Source, nil
}

type sourceListResp struct {
	Sources []domain.SourceEntry `json:"sources"`
}

func (c *NatsCatalogClient) ListSources(ctx context.Context) ([]*domain.SourceEntry, error) {
	body, err := c.natsRequest(ctx, "catalog.source.list", struct{}{})
	if err != nil {
		return nil, err
	}
	var resp sourceListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode source list: %w", err)
	}
	out := make([]*domain.SourceEntry, 0, len(resp.Sources))
	for i := range resp.Sources {
		out = append(out, &resp.Sources[i])
	}
	return out, nil
}

// =========================================================================
// RegistryRepository
// =========================================================================

type saveRegistryReq struct {
	URI         string `json:"uri"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
}

func (c *NatsCatalogClient) SaveRegistry(ctx context.Context, rec *domain.RegistryRecord) error {
	_, err := c.natsRequest(ctx, "catalog.registry.save", saveRegistryReq{
		URI: rec.URI, Pattern: rec.Pattern, Description: rec.Description,
	})
	return err
}

type findRegistryReq struct {
	URI string `json:"uri"`
}

func (c *NatsCatalogClient) FindRegistry(ctx context.Context, uri string) (*domain.RegistryRecord, error) {
	body, err := c.natsRequest(ctx, "catalog.registry.find", findRegistryReq{URI: uri})
	if err != nil {
		return nil, err
	}
	if msg := decodeError(body); msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	var rec domain.RegistryRecord
	if err := json.Unmarshal(body, &rec); err != nil {
		return nil, fmt.Errorf("decode registry response: %w", err)
	}
	return &rec, nil
}

type registryListResp struct {
	Records []domain.RegistryRecord `json:"records"`
}

func (c *NatsCatalogClient) ListRegistry(ctx context.Context) ([]*domain.RegistryRecord, error) {
	body, err := c.natsRequest(ctx, "catalog.registry.list", struct{}{})
	if err != nil {
		return nil, err
	}
	var resp registryListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode registry list: %w", err)
	}
	out := make([]*domain.RegistryRecord, 0, len(resp.Records))
	for i := range resp.Records {
		out = append(out, &resp.Records[i])
	}
	return out, nil
}

func (c *NatsCatalogClient) SearchRegistry(ctx context.Context, keyword string) ([]*domain.RegistryRecord, error) {
	all, err := c.ListRegistry(ctx)
	if err != nil {
		return nil, err
	}
	// Filter in-process — the Catalog doesn't expose a search-by-keyword
	// subject for registry records; the Java side does the same.
	var out []*domain.RegistryRecord
	for _, r := range all {
		if contains(r.URI, keyword) || contains(r.Description, keyword) {
			out = append(out, r)
		}
	}
	return out, nil
}

type existsRegistryReq struct {
	URI string `json:"uri"`
}

type existsResp struct {
	Exists bool `json:"exists"`
}

func (c *NatsCatalogClient) ExistsRegistry(ctx context.Context, uri string) (bool, error) {
	body, err := c.natsRequest(ctx, "catalog.registry.exists", existsRegistryReq{URI: uri})
	if err != nil {
		return false, err
	}
	var resp existsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("decode exists response: %w", err)
	}
	return resp.Exists, nil
}

// contains is a case-sensitive substring check. Used by SearchRegistry.
func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Compile-time assertions that NatsCatalogClient implements every
// repository interface. If one of these fails to compile, the
// corresponding method signature has drifted from the interface.
var (
	_ SystemRepository  = (*NatsCatalogClient)(nil)
	_ NodeRepository    = (*NatsCatalogClient)(nil)
	_ EventStore        = (*NatsCatalogClient)(nil)
	_ SourceRepository  = (*NatsCatalogClient)(nil)
	_ RegistryRepository = (*NatsCatalogClient)(nil)
)
