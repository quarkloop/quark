// Package llmctx manages the ordered message window fed to an LLM agent.
//
// File layout:
//
//	errors.go      – error codes and ContextError
//	types.go       – all value types (IDs, weights, content, counts, window, timestamp)
//	message.go     – Message, ContextStats, TokenComputer and Compactor interfaces
//	agentctx.go    – AgentContext (domain object only, no serialisation)
//	builder.go     – AgentContextBuilder
//	serializer.go  – ToFlatString, ToJSON (serialisation separated from domain)
//	adapter.go     – LLMAdapter interface + OpenAI, Anthropic, Generic implementations
//	tokenizer.go   – TokenComputer implementations
//	compactor.go   – Compactor strategy implementations
//	repository.go  – persistence interfaces
package llmctx

import "fmt"

// ---------------------------------------------------------------------------
// ContextErrorCode
// ---------------------------------------------------------------------------

// ContextErrorCode is an enumeration of every error condition in this package.
// Callers switch on Code for programmatic branching without string parsing.
type ContextErrorCode string

const (
	ErrCodeMessageNotFound     ContextErrorCode = "MESSAGE_NOT_FOUND"
	ErrCodeSystemPromptLocked  ContextErrorCode = "SYSTEM_PROMPT_LOCKED"
	ErrCodeNoCompactor         ContextErrorCode = "NO_COMPACTOR"
	ErrCodeCompactionFailed    ContextErrorCode = "COMPACTION_FAILED"
	ErrCodeInvalidMessage      ContextErrorCode = "INVALID_MESSAGE"
	ErrCodeTokenComputeFailed  ContextErrorCode = "TOKEN_COMPUTE_FAILED"
	ErrCodeSerializationFailed ContextErrorCode = "SERIALIZATION_FAILED"
	ErrCodeRepositoryFailed    ContextErrorCode = "REPOSITORY_FAILED"
	ErrCodeInvalidConfig       ContextErrorCode = "INVALID_CONFIG"
	ErrCodeBudgetExceeded      ContextErrorCode = "BUDGET_EXCEEDED"
)

// ---------------------------------------------------------------------------
// ContextError
// ---------------------------------------------------------------------------

// ContextError is the single structured error type for the entire package.
// Every function that returns an error returns *ContextError so callers can
// use errors.As to inspect Code and Cause without string matching.
type ContextError struct {
	Code    ContextErrorCode `json:"code"`
	Message string           `json:"message"`
	Cause   error            `json:"-"`
}

func (e *ContextError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap satisfies errors.Unwrap so errors.Is/As traversal works on the chain.
func (e *ContextError) Unwrap() error { return e.Cause }

// newErr is the internal constructor used throughout the package.
func newErr(code ContextErrorCode, msg string, cause error) *ContextError {
	return &ContextError{Code: code, Message: msg, Cause: cause}
}
