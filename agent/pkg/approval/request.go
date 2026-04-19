// Package approval provides human-in-the-loop approval gates for tool execution.
// In assistive mode, tool calls are blocked until a human approves or denies them.
package approval

import (
	"time"
)

// Status represents the state of an approval request.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusDenied   Status = "denied"
	StatusExpired  Status = "expired"
)

// Request represents a pending approval request for a tool call.
type Request struct {
	// ID is the unique identifier for this request.
	ID string `json:"id"`

	// ToolName is the name of the tool being called.
	ToolName string `json:"tool_name"`

	// Arguments is the JSON-encoded arguments for the tool call.
	Arguments string `json:"arguments"`

	// SessionID is the session that initiated the tool call.
	SessionID string `json:"session_id"`

	// Status is the current approval status.
	Status Status `json:"status"`

	// Reason is an optional explanation for approval or denial.
	Reason string `json:"reason,omitempty"`

	// CreatedAt is when the request was created.
	CreatedAt time.Time `json:"created_at"`

	// ResolvedAt is when the request was approved/denied/expired.
	ResolvedAt time.Time `json:"resolved_at,omitempty"`

	// ExpiresAt is when the request will automatically expire.
	ExpiresAt time.Time `json:"expires_at"`
}

// Response is the result of resolving an approval request.
type Response struct {
	// Approved is true if the request was approved.
	Approved bool

	// Reason is an optional explanation.
	Reason string
}

// NewRequest creates a new pending approval request.
func NewRequest(id, toolName, arguments, sessionID string, timeout time.Duration) *Request {
	now := time.Now()
	return &Request{
		ID:        id,
		ToolName:  toolName,
		Arguments: arguments,
		SessionID: sessionID,
		Status:    StatusPending,
		CreatedAt: now,
		ExpiresAt: now.Add(timeout),
	}
}

// IsResolved returns true if the request has been resolved (approved, denied, or expired).
func (r *Request) IsResolved() bool {
	return r.Status != StatusPending
}

// Approve marks the request as approved.
func (r *Request) Approve(reason string) {
	r.Status = StatusApproved
	r.Reason = reason
	r.ResolvedAt = time.Now()
}

// Deny marks the request as denied.
func (r *Request) Deny(reason string) {
	r.Status = StatusDenied
	r.Reason = reason
	r.ResolvedAt = time.Now()
}

// Expire marks the request as expired.
func (r *Request) Expire() {
	r.Status = StatusExpired
	r.ResolvedAt = time.Now()
}

// CheckExpiry checks if the request has expired and updates status if so.
func (r *Request) CheckExpiry() bool {
	if r.Status == StatusPending && time.Now().After(r.ExpiresAt) {
		r.Expire()
		return true
	}
	return false
}
