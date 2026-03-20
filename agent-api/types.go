package agentapi

import (
	"encoding/json"
	"time"
)

const DefaultBasePath = "/api/v1/agent"

type ChatRequest struct {
	Message string `json:"message"`
	Stream  bool   `json:"stream,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

type ChatResponse struct {
	Reply        string `json:"reply"`
	Mode         string `json:"mode,omitempty"`
	Warning      string `json:"warning,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

type HealthResponse struct {
	AgentID string `json:"agent_id,omitempty"`
	Status  string `json:"status"`
}

type ModeResponse struct {
	Mode string `json:"mode"`
}

type StatsResponse map[string]interface{}

type PlanStatus string

const (
	PlanDraft    PlanStatus = "draft"
	PlanApproved PlanStatus = "approved"
)

type StepStatus string

const (
	StepPending  StepStatus = "pending"
	StepRunning  StepStatus = "running"
	StepComplete StepStatus = "complete"
	StepFailed   StepStatus = "failed"
)

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

type Plan struct {
	Goal      string     `json:"goal"`
	Status    PlanStatus `json:"status"`
	Steps     []Step     `json:"steps"`
	Complete  bool       `json:"complete"`
	Summary   string     `json:"summary,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type ActivityRecord struct {
	ID        string          `json:"id"`
	SessionID string          `json:"session_id,omitempty"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
