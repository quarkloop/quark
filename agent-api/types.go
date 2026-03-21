// Package agentapi defines the shared HTTP API contracts for agent
// communication. Both the direct agent runtime and the api-server proxy
// implement the Service interface; the agent-client package consumes it.
package agentapi

import (
	"encoding/json"
	"time"
)

// DefaultBasePath is the URL prefix for the direct agent HTTP API.
// The api-server uses a per-agent prefix instead (see AgentProxyBasePath).
const DefaultBasePath = "/api/v1/agent"

// ChatRequest is the wire type for POST /chat. It carries a user message
// with optional mode override, streaming flag, and file attachments.
type ChatRequest struct {
	Message string           `json:"message"`
	Stream  bool             `json:"stream,omitempty"`
	Mode    string           `json:"mode,omitempty"`
	Files   []FileAttachment `json:"files,omitempty"`
}

// FileAttachment describes a file uploaded alongside a chat message.
// Content holds the raw bytes server-side but is never serialised to JSON.
type FileAttachment struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
	// Path is the server-side path where the file was saved.
	Path string `json:"path,omitempty"`
	// Content holds the raw file bytes. It is not serialized in JSON responses.
	Content []byte `json:"-"`
}

// ChatResponse is the wire type returned by POST /chat. It contains the
// agent's reply text, the resolved working mode, optional warnings, and
// approximate token counts for the request.
type ChatResponse struct {
	Reply        string `json:"reply"`
	Mode         string `json:"mode,omitempty"`
	Warning      string `json:"warning,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

// HealthResponse is the wire type for GET /health. It returns the agent's
// ID and a simple status string ("running"). This endpoint is intentionally
// minimal for fast liveness probes and port scanning during discovery.
type HealthResponse struct {
	AgentID string `json:"agent_id,omitempty"`
	Status  string `json:"status"`
}

// InfoResponse is the wire type for GET /info. It returns agent metadata
// including the configured LLM provider, model name, current working mode,
// and the list of registered tools. Use this for UI display and
// agent introspection; use /health for liveness checks.
type InfoResponse struct {
	AgentID  string   `json:"agent_id"`
	Provider string   `json:"provider"`
	Model    string   `json:"model"`
	Mode     string   `json:"mode"`
	Tools    []string `json:"tools"`
}

// ModeResponse is the wire type for GET /mode. It returns the agent's
// current working mode (ask, plan, masterplan, or auto).
type ModeResponse struct {
	Mode string `json:"mode"`
}

// StatsResponse is the wire type for GET /stats. It returns a free-form
// map of agent statistics including context window metrics, token counts,
// and compaction history.
type StatsResponse map[string]any

// PlanStatus represents the approval state of an execution plan.
type PlanStatus string

const (
	// PlanDraft indicates a plan awaiting user approval.
	PlanDraft PlanStatus = "draft"
	// PlanApproved indicates a plan cleared for execution.
	PlanApproved PlanStatus = "approved"
)

// StepStatus represents the execution state of a single plan step.
type StepStatus string

const (
	// StepPending indicates the step has not started yet.
	StepPending StepStatus = "pending"
	// StepRunning indicates the step is currently executing.
	StepRunning StepStatus = "running"
	// StepComplete indicates the step finished successfully.
	StepComplete StepStatus = "complete"
	// StepFailed indicates the step encountered an error.
	StepFailed StepStatus = "failed"
)

// Step is a single unit of work within an execution plan.
type Step struct {
	ID          string     `json:"id"`
	Agent       string     `json:"agent"`
	Description string     `json:"description"`
	DependsOn   []string   `json:"depends_on"`
	Status      StepStatus `json:"status"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// Plan is the wire type for GET /plan. It represents an execution plan
// with a goal, approval status, ordered steps, and completion state.
type Plan struct {
	Goal      string     `json:"goal"`
	Status    PlanStatus `json:"status"`
	Steps     []Step     `json:"steps"`
	Complete  bool       `json:"complete"`
	Summary   string     `json:"summary,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ActivityRecord is a single event in the agent's activity stream.
// The Type field identifies the event kind (e.g. "message.added",
// "tool.called"); the Data field carries event-specific JSON payload.
type ActivityRecord struct {
	ID        string          `json:"id"`
	SessionID string          `json:"session_id,omitempty"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// ErrorResponse is the standard error envelope returned by all endpoints.
type ErrorResponse struct {
	Error string `json:"error"`
}
