package session

import "time"

// Type classifies the purpose of a session.
type Type string

const (
	// TypeMain is the agent's persistent autonomous session.
	TypeMain Type = "main"
	// TypeChat is a user-initiated conversation thread.
	TypeChat Type = "chat"
	// TypeSubAgent is a session created for a worker step execution.
	TypeSubAgent Type = "subagent"
	// TypeCron is a session created for a scheduled task run.
	TypeCron Type = "cron"
)

// Status represents the lifecycle state of a session.
type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusDeleted   Status = "deleted"
)

// Session is a communication channel between a user (or system) and an agent.
// Each session owns its own AgentContext (LLM message window) and activity stream.
type Session struct {
	Key         string     `json:"key"`
	AgentID     string     `json:"agent_id"`
	Type        Type       `json:"type"`
	Status      Status     `json:"status"`
	Title       string     `json:"title,omitempty"`
	ParentKey   string     `json:"parent_key,omitempty"`
	SnapshotRef string     `json:"snapshot_ref,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
}
