package llmctx

// =============================================================================
// message_types.go  —  Root-package re-exports of llmctx/message payload types
//
// M4: The canonical payload types live in llmctx/message. This file re-exports
// them as type aliases in the root package so callers who import only "llmctx"
// can use e.g. llmctx.TextPayload without a separate import.
//
// Rule: if you need to add a new payload type, define it in llmctx/message/<n>.go
// and add a single alias line here. Do NOT define payload types in this file.
// =============================================================================

import msg "github.com/quarkloop/agent/pkg/context/message"

// ---------------------------------------------------------------------------
// Payload types
// ---------------------------------------------------------------------------

type (
	SystemPromptPayload = msg.SystemPromptPayload
	TextPayload         = msg.TextPayload
	ToolCallPayload     = msg.ToolCallPayload
	ToolResultPayload   = msg.ToolResultPayload
	MemoryPayload       = msg.MemoryPayload
	ReasoningPayload    = msg.ReasoningPayload
	LogPayload          = msg.LogPayload
	ErrorPayload        = msg.ErrorPayload
	PlanPayload         = msg.PlanPayload
)

// ---------------------------------------------------------------------------
// Payload enum types and constants
// ---------------------------------------------------------------------------

// LogLevel re-exports for llmctx.LogLevel* usage.
type LogLevel = msg.LogLevel

const (
	LogLevelDebug = msg.LogLevelDebug
	LogLevelInfo  = msg.LogLevelInfo
	LogLevelWarn  = msg.LogLevelWarn
	LogLevelError = msg.LogLevelError
)

// MemoryScope re-exports for llmctx.MemoryScope* usage.
type MemoryScope = msg.MemoryScope

const (
	MemoryScopeEphemeral = msg.MemoryScopeEphemeral
	MemoryScopeSession   = msg.MemoryScopeSession
	MemoryScopeUser      = msg.MemoryScopeUser
	MemoryScopeGlobal    = msg.MemoryScopeGlobal
)

// PlanStepStatus re-exports for llmctx.PlanStepStatus* usage.
type PlanStepStatus = msg.PlanStepStatus

const (
	PlanStepPending    = msg.PlanStepPending
	PlanStepInProgress = msg.PlanStepInProgress
	PlanStepCompleted  = msg.PlanStepCompleted
	PlanStepFailed     = msg.PlanStepFailed
	PlanStepSkipped    = msg.PlanStepSkipped
)

// PlanStep re-export.
type PlanStep = msg.PlanStep
