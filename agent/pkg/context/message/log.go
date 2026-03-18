package message

import (
	"fmt"
	"sort"
	"strings"
)

// LogLevel is the severity level of a log message.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogPayload stores structured audit or debug log output in the context history.
//
// Log messages are surfaced only to developers and never injected into the LLM.
type LogPayload struct {
	// Level is the severity.
	Level LogLevel `json:"level"`
	// Message is the human-readable description.
	Message string `json:"message"`
	// Fields carries structured key=value pairs.
	Fields map[string]string `json:"fields,omitempty"`
}

func init() { RegisterPayloadFactory(LogType, func() Payload { return &LogPayload{} }) }

func (p LogPayload) Kind() MessageType { return LogType }
func (p LogPayload) sealPayload()      {}

func (p LogPayload) TextRepresentation() string {
	if len(p.Fields) == 0 {
		return fmt.Sprintf("[log:%s] %s", p.Level, p.Message)
	}
	return fmt.Sprintf("[log:%s] %s {%s}", p.Level, p.Message, formatFields(p.Fields))
}

// LLMText returns "" — log messages are never injected into the LLM context.
func (p LogPayload) LLMText() string { return "" }

// UserText returns "" — log messages are not shown to users.
func (p LogPayload) UserText() string { return "" }

// DevText returns full level, message, and structured fields.
func (p LogPayload) DevText() string {
	if len(p.Fields) == 0 {
		return fmt.Sprintf("[%s] %s", p.Level, p.Message)
	}
	return fmt.Sprintf("[%s] %s\n  fields: %s", p.Level, p.Message, formatFields(p.Fields))
}

// formatFields renders a string map as sorted key=value pairs for stable output.
func formatFields(fields map[string]string) string {
	pairs := make([]string, 0, len(fields))
	for k, v := range fields {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	return strings.Join(pairs, " ")
}
