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
	Message    string           `json:"message"`
	SessionKey string           `json:"session_key,omitempty"`
	Stream     bool             `json:"stream,omitempty"`
	Mode       string           `json:"mode,omitempty"`
	Files      []FileAttachment `json:"files,omitempty"`
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

// SetModeRequest is the wire type for POST /mode. It sets the agent's
// working mode.
type SetModeRequest struct {
	Mode string `json:"mode"`
}

// PlanActionRequest is the wire type for POST /plan/approve and
// POST /plan/reject. The plan_id field is optional — when omitted the
// single active plan is targeted.
type PlanActionRequest struct {
	PlanID string `json:"plan_id,omitempty"`
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
	// PlanRejected indicates a plan has been rejected.
	PlanRejected PlanStatus = "rejected"
	// PlanExecuting indicates a plan is currently being executed.
	PlanExecuting PlanStatus = "executing"
	// PlanSucceeded indicates a plan completed successfully.
	PlanSucceeded PlanStatus = "succeeded"
	// PlanFailed indicates a plan terminated with errors.
	PlanFailed PlanStatus = "failed"
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

// SessionType classifies the purpose of a session.
type SessionType string

const (
	SessionTypeMain     SessionType = "main"
	SessionTypeChat     SessionType = "chat"
	SessionTypeSubAgent SessionType = "subagent"
	SessionTypeCron     SessionType = "cron"
)

// SessionRecord is the wire type for session endpoints.
type SessionRecord struct {
	Key       string      `json:"key"`
	AgentID   string      `json:"agent_id"`
	Type      SessionType `json:"type"`
	Status    string      `json:"status"`
	Title     string      `json:"title,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	EndedAt   *time.Time  `json:"ended_at,omitempty"`
}

// CreateSessionRequest is the wire type for POST /sessions.
type CreateSessionRequest struct {
	Type  SessionType `json:"type"`
	Title string      `json:"title,omitempty"`
}

// CreateSessionResponse is returned by POST /sessions.
type CreateSessionResponse struct {
	Session SessionRecord `json:"session"`
}

// ErrorResponse is the standard error envelope returned by all endpoints.
type ErrorResponse struct {
	Error string `json:"error"`
}

// BudgetResponse is the wire type for GET /sessions/{sessionKey}/budget.
type BudgetResponse struct {
	TotalBudget      int     `json:"total_budget"`
	UsedTokens       int     `json:"used_tokens"`
	AvailableTokens  int     `json:"available_tokens"`
	UsagePct         float64 `json:"usage_pct"`
	AtSoftLimit      bool    `json:"at_soft_limit"`
	AtHardLimit      bool    `json:"at_hard_limit"`
	CompactionNeeded bool    `json:"compaction_needed"`
}
