// Package deploy orchestrates system deployment.
//
// The Go control plane's DeployService is intentionally simpler than
// the Java version:
//
//   - It does NOT parse .quark.ts. The source is treated as an opaque
//     string from the server's perspective.
//   - It persists the source verbatim to the Catalog.
//   - It does a minimal "structural sniff" (regex) to extract the
//     system name and runtime mode (shared/isolated) — this is needed
//     to know which runtimeId to route the deploy command to. Full
//     parsing happens in the runtime via SimpleSystemParser +
//     GraalJsSystemParser.
//   - It sends the deploy command via NATS request-reply and waits
//     for the StatusResponse, which carries the parsed node list.
//   - It persists NodeRecords to the Catalog from the response (the
//     data plane can't write to the Catalog directly).
//
// Same wire format as the Java DeployService — the runtime's
// DataPlaneCommandHandler doesn't know whether the caller is Java or Go.
package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	"github.com/quarkloop/quark/server/internal/dataplane"
	"github.com/quarkloop/quark/server/internal/domain"
	"github.com/quarkloop/quark/server/internal/store"
)

// ErrParse is returned when the .quark.ts source is missing the
// required `name` field or has a structurally invalid header.
// (We don't fully parse — just sniff name + runtime.)
var ErrParse = errors.New("parse failed")

// DeployService is the application-layer orchestrator for system
// deploy/undeploy.
//
// Layer: http/handler → deploy.DeployService → store.*Repository → nats
type DeployService struct {
	log         *zap.Logger
	nc          *nats.Conn
	sysRepo     store.SystemRepository
	nodeRepo    store.NodeRepository
	srcRepo     store.SourceRepository
	procMgr     *dataplane.ProcessManager
}

// NewDeployService constructs a DeployService.
func NewDeployService(
	log *zap.Logger,
	nc *nats.Conn,
	sysRepo store.SystemRepository,
	nodeRepo store.NodeRepository,
	srcRepo store.SourceRepository,
	procMgr *dataplane.ProcessManager,
) *DeployService {
	return &DeployService{
		log: log, nc: nc,
		sysRepo: sysRepo, nodeRepo: nodeRepo, srcRepo: srcRepo,
		procMgr: procMgr,
	}
}

// Deploy deploys a system from .quark.ts source.
//
// Flow:
//   1. Sniff name + runtime mode from source (regex, not full parse).
//   2. Apply namespace override (the CLI's -n flag wins).
//   3. Persist the system record + source to the Catalog.
//   4. Ensure the appropriate data-plane process is running.
//   5. Send the deploy command via NATS request-reply (5 retries × 3s).
//   6. Persist NodeRecords from the response.
//
// Returns the name of the deployed system + the parsed node info
// (so the HTTP handler can construct a DeploySystemResponse).
func (s *DeployService) Deploy(ctx context.Context, source, namespaceOverride string) (result *DeployResult, err error) {
	// 1. Sniff name + runtime mode
	sniff, err := sniffSystemMeta(source)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParse, err)
	}

	// 2. Apply namespace override
	ns := sniff.Namespace
	if namespaceOverride != "" && namespaceOverride != ns {
		s.log.Info("overriding namespace",
			zap.String("from", ns), zap.String("to", namespaceOverride))
		ns = namespaceOverride
	}

	// 3. Persist system record + source
	now := domain.Now()
	sysRec := &domain.SystemRecord{
		Namespace: ns,
		Name:      sniff.Name,
		Source:    source,
		State:     domain.SystemStateCreating,
		Health:    domain.HealthUnknown,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.sysRepo.SaveSystem(ctx, sysRec); err != nil {
		s.log.Warn("persist system record failed — deploy will continue",
			zap.String("namespace", ns), zap.String("system", sniff.Name), zap.Error(err))
	}
	if err := s.sysRepo.UpdateSystemState(ctx, ns, sniff.Name, domain.SystemStateActive, domain.HealthHealthy, 1); err != nil {
		s.log.Warn("update system state failed",
			zap.String("namespace", ns), zap.String("system", sniff.Name), zap.Error(err))
	}
	if err := s.srcRepo.SaveSource(ctx, ns, sniff.Name, source); err != nil {
		s.log.Warn("save source failed",
			zap.String("namespace", ns), zap.String("system", sniff.Name), zap.Error(err))
	}

	// 4. Ensure data-plane process is running
	runtimeId := dataplane.RuntimeID(ns, sniff.IsIsolated)
	if _, err := s.procMgr.EnsureProcess(ctx, runtimeId); err != nil {
		return nil, fmt.Errorf("ensure data-plane process %s: %w", runtimeId, err)
	}

	// 5. Send deploy command via NATS request-reply
	subject := dataplane.DeploySubject(runtimeId)
	cmd := DeployCommand{Namespace: ns, SystemName: sniff.Name, Source: source}
	payload, _ := json.Marshal(cmd)

	var resp *StatusResponse
	for attempt := 1; attempt <= 5; attempt++ {
		reply, rerr := s.nc.RequestWithContext(ctx, subject, payload)
		if rerr == nil && reply != nil {
			if uerr := json.Unmarshal(reply.Data, &resp); uerr != nil {
				return nil, fmt.Errorf("decode status response: %w", uerr)
			}
			break
		}
		s.log.Debug("deploy attempt got no response, retrying",
			zap.Int("attempt", attempt),
			zap.String("runtimeId", runtimeId),
			zap.Error(rerr))
		time.Sleep(time.Second)
	}
	if resp == nil {
		return nil, fmt.Errorf("data-plane did not respond to deploy for %s/%s after 5 attempts (15s total)",
			ns, sniff.Name)
	}
	if !resp.Success {
		return nil, fmt.Errorf("data-plane deploy failed for %s/%s: %s",
			ns, sniff.Name, resp.Error)
	}

	// 6. Persist NodeRecords from the response
	if len(resp.Nodes) > 0 {
		recs := make([]*domain.NodeRecord, 0, len(resp.Nodes))
		for _, ni := range resp.Nodes {
			recs = append(recs, &domain.NodeRecord{
				Namespace:  ns,
				SystemName: sniff.Name,
				Name:       ni.Name,
				URI:        ni.URI,
				State:      ni.State,
				Health:     ni.Health,
				Version:    1,
				Listens:    ni.Listens,
				Events:     ni.Events,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
		if err := s.nodeRepo.SaveNodes(ctx, recs); err != nil {
			s.log.Warn("persist node records failed",
				zap.String("namespace", ns), zap.String("system", sniff.Name), zap.Error(err))
		}
	}

	s.log.Info("deployed system",
		zap.String("namespace", ns),
		zap.String("system", sniff.Name),
		zap.String("runtimeId", runtimeId),
		zap.Int("nodes", len(resp.Nodes)))

	return &DeployResult{
		Name:      sniff.Name,
		Namespace: ns,
		Nodes:     resp.Nodes,
		State:     domain.SystemStateActive,
		Health:    domain.HealthHealthy,
	}, nil
}

// Undeploy removes a system.
//
// Flow:
//   1. Determine runtimeId (isolated vs shared) by checking which
//      process is alive for this namespace.
//   2. Send undeploy command via NATS request-reply (15s timeout).
//   3. Delete system + node records from the Catalog.
//   4. If isolated namespace has no remaining systems, stop its process.
func (s *DeployService) Undeploy(ctx context.Context, namespace, systemName string) error {
	isolatedId := dataplane.RuntimeID(namespace, true)
	sharedId := dataplane.RuntimeID(namespace, false)
	runtimeId := sharedId
	if s.procMgr.IsProcessAlive(isolatedId) {
		runtimeId = isolatedId
	}

	// Send undeploy command
	subject := dataplane.UndeploySubject(runtimeId)
	cmd := UndeployCommand{Namespace: namespace, SystemName: systemName}
	payload, _ := json.Marshal(cmd)
	reply, err := s.nc.RequestWithContext(ctx, subject, payload)
	if err != nil {
		s.log.Warn("undeploy NATS request failed",
			zap.String("namespace", namespace), zap.String("system", systemName), zap.Error(err))
	} else if reply != nil {
		var resp StatusResponse
		if uerr := json.Unmarshal(reply.Data, &resp); uerr == nil && !resp.Success {
			s.log.Warn("data-plane undeploy failed",
				zap.String("namespace", namespace), zap.String("system", systemName),
				zap.String("error", resp.Error))
		}
	}

	// Delete system + node records
	if err := s.nodeRepo.DeleteNodesBySystem(ctx, namespace, systemName); err != nil {
		s.log.Warn("delete node records failed",
			zap.String("namespace", namespace), zap.String("system", systemName), zap.Error(err))
	}
	if err := s.sysRepo.DeleteSystem(ctx, namespace, systemName); err != nil {
		s.log.Warn("delete system record failed",
			zap.String("namespace", namespace), zap.String("system", systemName), zap.Error(err))
	}

	// If isolated and no systems remain, stop the dedicated process
	if runtimeId == isolatedId {
		remaining, _ := s.sysRepo.ListSystems(ctx, namespace)
		if len(remaining) == 0 {
			s.procMgr.StopProcess(isolatedId)
			s.log.Info("stopped isolated data-plane process (no systems remaining)",
				zap.String("namespace", namespace))
		}
	}

	s.log.Info("undeployed system",
		zap.String("namespace", namespace), zap.String("system", systemName))
	return nil
}

// Apply is the declarative reconcile path (PUT /systems/:name).
// Since the Go server doesn't parse, it can't compute a real diff —
// it just redeploys. The Java side does the same when systems aren't
// found in RuntimeContext.
func (s *DeployService) Apply(ctx context.Context, source, namespaceOverride string) (*ApplyResult, error) {
	deployed, err := s.Deploy(ctx, source, namespaceOverride)
	if err != nil {
		return nil, err
	}
	// We can't tell if the system was newly created or updated because
	// we don't keep an in-memory registry. Always report "created=true".
	return &ApplyResult{
		Name:      deployed.Name,
		Namespace: deployed.Namespace,
		Created:   true,
		Changed:   true,
		Changes:   []ApplyChange{},
	}, nil
}

// RecoverFromDisk re-deploys every system whose source is persisted in
// the Catalog. Called on server startup.
//
// Same logic as the Java DeployService.recoverFromDisk — but uses
// the Go store interfaces and DeployService.Deploy.
func (s *DeployService) RecoverFromDisk(ctx context.Context) []string {
	entries, err := s.srcRepo.ListSources(ctx)
	if err != nil {
		s.log.Warn("list sources for recovery failed", zap.Error(err))
		return nil
	}
	var recovered []string
	for _, e := range entries {
		source, err := s.srcRepo.GetSource(ctx, e.Namespace, e.Name)
		if err != nil {
			s.log.Warn("get source for recovery failed",
				zap.String("namespace", e.Namespace), zap.String("system", e.Name), zap.Error(err))
			continue
		}
		if _, err := s.Deploy(ctx, source, e.Namespace); err != nil {
			s.log.Warn("recover deploy failed",
				zap.String("namespace", e.Namespace), zap.String("system", e.Name), zap.Error(err))
			continue
		}
		recovered = append(recovered, e.Namespace+"/"+e.Name)
		s.log.Info("recovered system",
			zap.String("namespace", e.Namespace), zap.String("system", e.Name))
	}
	return recovered
}

// --- DTOs ---

// DeployCommand is the JSON payload sent over NATS to
// quark.control.<runtimeId>.deploy. Same shape as the Java
// DataPlaneCommandHandler.DeployCommand.
type DeployCommand struct {
	Namespace  string `json:"namespace"`
	SystemName string `json:"systemName"`
	Source     string `json:"source"`
}

// UndeployCommand is the JSON payload sent over NATS to
// quark.control.<runtimeId>.undeploy.
type UndeployCommand struct {
	Namespace  string `json:"namespace"`
	SystemName string `json:"systemName"`
}

// StatusResponse is the JSON response from the data plane after a
// deploy/undeploy command. Same shape as the Java
// DataPlaneCommandHandler.StatusResponse.
type StatusResponse struct {
	Success     bool       `json:"success"`
	SystemName  string     `json:"systemName"`
	Namespace   string     `json:"namespace"`
	Error       string     `json:"error,omitempty"`
	Nodes       []NodeInfo `json:"nodes,omitempty"`
}

// NodeInfo is the per-node info reported back by the data plane after
// deploy. Contains enough data for the control plane to persist a
// NodeRecord.
type NodeInfo struct {
	Name   string   `json:"name"`
	URI    string   `json:"uri"`
	State  string   `json:"state"`
	Health string   `json:"health"`
	Listens []string `json:"listens"`
	Events  []string `json:"events"`
}

// DeployResult is the in-process return value of Deploy(). The HTTP
// handler converts this to a DeploySystemResponse DTO.
type DeployResult struct {
	Name      string
	Namespace string
	Nodes     []NodeInfo
	State     string
	Health    string
}

// ApplyResult is the in-process return value of Apply().
type ApplyResult struct {
	Name      string
	Namespace string
	Created   bool
	Changed   bool
	Changes   []ApplyChange
}

// ApplyChange describes a single diff entry (unused in Go server but
// kept for DTO compatibility with the Java server's wire format).
type ApplyChange struct {
	Type    string `json:"type"`
	Node    string `json:"node"`
	Details string `json:"details"`
}

// --- Source sniffer ---
//
// The Go server does NOT do full TypeScript parsing. It uses simple
// regex to extract two pieces of metadata:
//   - name: "monitor"
//   - runtime: "isolated"  (optional; defaults to "shared")
//
// These are needed to (a) know the system name for persistence and
// (b) know which runtimeId to route the deploy command to.
//
// Full parsing (extracting nodes, configs, listens, events, etc.)
// happens in the runtime via SimpleSystemParser + GraalJsSystemParser.

var (
	nameFieldRe    = regexp.MustCompile(`(?m)^\s*name\s*:\s*["']([^"']+)["']`)
	runtimeFieldRe = regexp.MustCompile(`(?m)^\s*runtime\s*:\s*["']([^"']+)["']`)
	nsFieldRe      = regexp.MustCompile(`(?m)^\s*namespace\s*:\s*["']([^"']+)["']`)
)

// systemMeta is the minimal info extracted from a .quark.ts file.
type systemMeta struct {
	Name       string
	Namespace  string
	IsIsolated bool
}

// sniffSystemMeta extracts name + namespace + runtime mode from source.
//
// Limitations: this regex works on the simple-streaming example. For
// more complex .quark.ts files (e.g. with computed property names or
// conditional logic), the runtime's full parser is authoritative —
// the Go server's sniff is only used for routing.
func sniffSystemMeta(source string) (*systemMeta, error) {
	if source == "" {
		return nil, errors.New("source is empty")
	}

	nameMatch := nameFieldRe.FindStringSubmatch(source)
	if len(nameMatch) < 2 || nameMatch[1] == "" {
		return nil, errors.New("missing required field: name")
	}

	nsMatch := nsFieldRe.FindStringSubmatch(source)
	ns := ""
	if len(nsMatch) >= 2 {
		ns = nsMatch[1]
	}

	isolated := false
	if rtMatch := runtimeFieldRe.FindStringSubmatch(source); len(rtMatch) >= 2 {
		if rtMatch[1] == "isolated" {
			isolated = true
		}
	}

	return &systemMeta{
		Name:       nameMatch[1],
		Namespace:  ns,
		IsIsolated: isolated,
	}, nil
}
