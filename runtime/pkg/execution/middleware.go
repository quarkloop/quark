package execution

import (
	"context"
	"fmt"

	"github.com/quarkloop/runtime/pkg/approval"
	"github.com/quarkloop/runtime/pkg/loop"
)

// ToolCallMessage is a message type for tool execution requests.
// This interface allows the middleware to intercept tool calls for approval.
type ToolCallMessage interface {
	loop.Message
	ToolName() string
	ToolArguments() string
	SessionID() string
}

// ApprovalMiddleware creates middleware that requires approval for tool calls in assistive mode.
// It intercepts ToolCallMessage types and blocks until approval is received.
func ApprovalMiddleware(gate *approval.Gate) loop.Middleware {
	return func(next loop.HandlerFunc) loop.HandlerFunc {
		return func(ctx context.Context, msg loop.Message) error {
			// Only intercept tool call messages
			toolMsg, ok := msg.(ToolCallMessage)
			if !ok {
				return next(ctx, msg)
			}

			// Request approval and block until received
			err := gate.RequestApproval(
				ctx,
				toolMsg.ToolName(),
				toolMsg.ToolArguments(),
				toolMsg.SessionID(),
			)

			if err != nil {
				if err == approval.ErrDenied {
					return fmt.Errorf("tool call denied: %s", toolMsg.ToolName())
				}
				if err == approval.ErrExpired {
					return fmt.Errorf("approval request expired for: %s", toolMsg.ToolName())
				}
				return err
			}

			// Approved — continue to handler
			return next(ctx, msg)
		}
	}
}

// ModeMiddleware creates middleware that enforces execution mode behavior.
// For autonomous mode, it passes through. For other modes, it applies mode-specific logic.
func ModeMiddleware(mode Mode) loop.Middleware {
	return func(next loop.HandlerFunc) loop.HandlerFunc {
		return func(ctx context.Context, msg loop.Message) error {
			// Inject mode into context for handlers to access
			ctx = context.WithValue(ctx, modeContextKey{}, mode)
			return next(ctx, msg)
		}
	}
}

// modeContextKey is the context key for execution mode.
type modeContextKey struct{}

// GetMode retrieves the execution mode from context.
func GetMode(ctx context.Context) Mode {
	if m, ok := ctx.Value(modeContextKey{}).(Mode); ok {
		return m
	}
	return ModeAutonomous
}
