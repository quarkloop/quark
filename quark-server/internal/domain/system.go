// Package domain contains the Go control plane's domain types.
//
// These types mirror the Java records in runtime/quark-core under
// com.quarkloop.quark.runtime.domain.* — but only the subset the
// control plane actually touches. The duplication is intentional:
// the stable contract between Go server and Java runtime is the JSON
// wire format over NATS, not shared source code.
//
// Field tags are tuned for compatibility with both the CLI's JSON
// expectations (see cli/internal/model/*.go) and the Catalog's NATS
// request/response shapes (see quark-catalog/internal/api/*.go).
package domain

import "time"

// SystemRecord is the persisted row for a deployed system. Mirrors
// quark-catalog/internal/api.SystemResponse.
type SystemRecord struct {
        Namespace string `json:"namespace"`
        Name      string `json:"name"`
        Source    string `json:"source"`
        State     string `json:"state"`
        Health    string `json:"health"`
        Version   int64  `json:"version"`
        CreatedAt string `json:"createdAt"`
        UpdatedAt string `json:"updatedAt"`
}

// NodeRecord is the persisted row for a deployed node. Mirrors
// quark-catalog/internal/api.NodeResponse.
type NodeRecord struct {
        Namespace        string                 `json:"namespace"`
        SystemName       string                 `json:"systemName"`
        Name             string                 `json:"name"`
        URI              string                 `json:"uri"`
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

// NodeEvent is an immutable record of a significant node lifecycle
// occurrence. Mirrors quark-catalog/internal/api.EventResponse.
//
// The Timestamp field is typed as `any` (not `string`) because the
// Java data plane serializes java.time.Instant as epoch-millis (a
// JSON number) by default, while the Catalog stores it as an RFC3339
// string. The event receiver normalizes to string before persisting.
type NodeEvent struct {
        ID         string         `json:"id"`
        Kind       string         `json:"kind"`
        NodeName   string         `json:"nodeName"`
        SystemName string         `json:"systemName"`
        Namespace  string         `json:"namespace"`
        Timestamp  any            `json:"timestamp"`
        Payload    map[string]any `json:"payload,omitempty"`
}

// TimestampString returns the timestamp as a string. If the timestamp
// arrived as a JSON number, it's converted to an RFC3339Nano string.
// If already a string, returned as-is.
//
// Heuristic: Jackson's default for Instant is epoch-seconds (a number
// ~1.7e9 for 2024+). Epoch-millis would be ~1.7e12. We auto-detect.
func (e *NodeEvent) TimestampString() string {
        switch t := e.Timestamp.(type) {
        case string:
                return t
        case float64:
                if t > 1e11 {
                        // epoch-millis
                        secs := int64(t) / 1000
                        nanos := int64((t - float64(secs)*1000) * 1_000_000)
                        return time.Unix(secs, nanos).UTC().Format(time.RFC3339Nano)
                }
                // epoch-seconds (Jackson default for Instant)
                secs := int64(t)
                nanos := int64((t - float64(secs)) * 1_000_000_000)
                return time.Unix(secs, nanos).UTC().Format(time.RFC3339Nano)
        case int64:
                if t > 1e11 {
                        return time.Unix(t/1000, (t%1000)*1_000_000).UTC().Format(time.RFC3339Nano)
                }
                return time.Unix(t, 0).UTC().Format(time.RFC3339Nano)
        }
        return ""
}

// RegistryRecord is a built-in node descriptor (uri + pattern + description).
// Mirrors quark-catalog/internal/api.RegistryResponse.
type RegistryRecord struct {
        URI         string `json:"uri"`
        Pattern     string `json:"pattern"`
        Description string `json:"description"`
}

// SourceEntry is a (namespace, name) pair — the listing shape returned
// by SourceRepository.ListSources.
type SourceEntry struct {
        Namespace string `json:"namespace"`
        Name      string `json:"name"`
}

// SystemState enumeration (mirrors the Java enum names).
const (
        SystemStateCreating = "CREATING"
        SystemStateActive   = "ACTIVE"
        SystemStateDeleting = "DELETING"
        SystemStateError    = "ERROR"

        HealthUnknown = "UNKNOWN"
        HealthHealthy = "HEALTHY"
)

// NodeState enumeration — used by lifecycle transitions.
const (
        NodeStateActive    = "ACTIVE"
        NodeStatePaused    = "PAUSED"
        NodeStateDraining  = "DRAINING"
        NodeStateArchived  = "ARCHIVED"
        NodeStateError     = "ERROR"
        NodeStateRecovering = "RECOVERING"
        NodeStateDeleted   = "DELETED"
)

// NodeEventKind enumeration — used by event filtering.
const (
        EventNodeCreated           = "NODE_CREATED"
        EventNodeUpdated           = "NODE_UPDATED"
        EventNodeStateChanged      = "NODE_STATE_CHANGED"
        EventNodeDataReceived      = "NODE_DATA_RECEIVED"
        EventNodeDataProduced      = "NODE_DATA_PRODUCED"
        EventNodeExecutionStarted  = "NODE_EXECUTION_STARTED"
        EventNodeExecutionCompleted = "NODE_EXECUTION_COMPLETED"
        EventNodeExecutionFailed   = "NODE_EXECUTION_FAILED"
        EventNodeQueryReceived     = "NODE_QUERY_RECEIVED"
        EventNodeQueryResponded    = "NODE_QUERY_RESPONDED"
        EventNodePolicyEvaluated   = "NODE_POLICY_EVALUATED"
        EventNodePolicyViolated    = "NODE_POLICY_VIOLATED"
)

// Now returns the current time as an RFC3339Nano string. Centralised
// here so the rest of the codebase doesn't sprinkle time.Now().Format(...)
// calls everywhere — and so tests can stub it if needed.
func Now() string {
        return time.Now().UTC().Format(time.RFC3339Nano)
}
