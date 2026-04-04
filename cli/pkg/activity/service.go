// Package activitysvc defines the activity service interface and a local
// JSONL-backed implementation.
package activity

import (
	"context"
	"errors"

	agentapi "github.com/quarkloop/agent-api"
)

// ErrStreamNotSupported is returned when streaming is attempted in local mode.
var ErrStreamNotSupported = errors.New("activity streaming requires a running agent (use --agent-url)")

// QueryOptions controls filtering for activity queries.
type QueryOptions struct {
	Type  string // filter by event type
	Since string // RFC3339 timestamp
	Limit int    // max entries (0 = default 50)
}

// Service defines operations for managing the activity log.
type Service interface {
	Append(ctx context.Context, record agentapi.ActivityRecord) error
	Query(ctx context.Context, opts QueryOptions) ([]agentapi.ActivityRecord, error)
	Stream(ctx context.Context, fn func(agentapi.ActivityRecord)) error
	Close() error
}
