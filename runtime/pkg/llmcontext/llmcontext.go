// Package llmcontext manages LLM context windows with strict separation
// between Work Context (autonomous execution) and Session Context (chat).
//
// Key invariants:
//   - Session history stays in session only
//   - Work history stays in work context only
//   - Work status can be injected as read-only summary into sessions
//   - Compaction is triggered lazily at 80% of context window
package llmcontext

import (
	"log/slog"
	"sync"

	"github.com/quarkloop/pkg/plugin"
)

const (
	// compactThreshold is the fraction of the context window at which
	// compaction kicks in (0.8 = 80%).
	compactThreshold = 0.8

	// charsPerToken is a rough estimate for token counting.
	charsPerToken = 4

	// maxMessages is a hard cap on message count.
	maxMessages = 200
)

// Context manages a single isolated LLM context window.
// Use WorkContext or SessionContext for type-safe separation.
type Context struct {
	mu            sync.RWMutex
	messages      []plugin.Message
	contextWindow int // token limit (0 = unlimited)
}

// New creates a new context with the given token limit.
func New(contextWindow int) *Context {
	return &Context{
		messages:      make([]plugin.Message, 0, 32),
		contextWindow: contextWindow,
	}
}

// Add appends a message to the context.
func (c *Context) Add(role, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = append(c.messages, plugin.Message{
		Role:    role,
		Content: content,
	})

	c.compactIfNeeded()
}

// AddWithToolCalls appends an assistant message with tool calls.
func (c *Context) AddWithToolCalls(content string, toolCalls []plugin.ToolCall) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = append(c.messages, plugin.Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	})

	c.compactIfNeeded()
}

// AddToolResult appends a tool result message.
func (c *Context) AddToolResult(toolCallID, result string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = append(c.messages, plugin.Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
	})

	c.compactIfNeeded()
}

// Messages returns a copy of all messages.
func (c *Context) Messages() []plugin.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]plugin.Message, len(c.messages))
	copy(out, c.messages)
	return out
}

// Len returns the number of messages.
func (c *Context) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.messages)
}

// Clear removes all messages.
func (c *Context) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = c.messages[:0]
}

// SetContextWindow updates the token limit.
func (c *Context) SetContextWindow(tokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.contextWindow = tokens
}

// compactIfNeeded drops oldest messages if limits are exceeded.
// Caller must hold the write lock.
func (c *Context) compactIfNeeded() {
	n := len(c.messages)
	if n == 0 {
		return
	}

	// Calculate character lengths
	lengths := make([]int, n)
	for i, m := range c.messages {
		lengths[i] = len(m.Content)
	}

	startIdx := CompactIndex(lengths, c.contextWindow)
	if startIdx > 0 {
		c.messages = c.messages[startIdx:]
	}
}

// WorkContext is an isolated context for autonomous work execution.
// Work context MUST NOT receive session messages.
type WorkContext struct {
	ctx *Context
}

// NewWorkContext creates a new work context.
func NewWorkContext(contextWindow int) *WorkContext {
	return &WorkContext{ctx: New(contextWindow)}
}

// Add appends a message to the work context.
func (w *WorkContext) Add(role, content string) {
	w.ctx.Add(role, content)
}

// Messages returns the work history.
func (w *WorkContext) Messages() []plugin.Message {
	return w.ctx.Messages()
}

// Clear resets the work context.
func (w *WorkContext) Clear() {
	w.ctx.Clear()
}

// Len returns the number of messages.
func (w *WorkContext) Len() int {
	return w.ctx.Len()
}

// SessionContext is an isolated context for a single chat session.
// Session context MUST NOT leak to work context.
type SessionContext struct {
	ctx       *Context
	sessionID string
}

// NewSessionContext creates a new session context.
func NewSessionContext(sessionID string, contextWindow int) *SessionContext {
	return &SessionContext{
		ctx:       New(contextWindow),
		sessionID: sessionID,
	}
}

// SessionID returns the session identifier.
func (s *SessionContext) SessionID() string {
	return s.sessionID
}

// Add appends a message to the session context.
func (s *SessionContext) Add(role, content string) {
	s.ctx.Add(role, content)
}

// Messages returns the session history.
func (s *SessionContext) Messages() []plugin.Message {
	return s.ctx.Messages()
}

// Clear resets the session context.
func (s *SessionContext) Clear() {
	s.ctx.Clear()
}

// Len returns the number of messages.
func (s *SessionContext) Len() int {
	return s.ctx.Len()
}

// BuildWithWorkSummary creates a message list for LLM call that includes
// a read-only work status summary WITHOUT including full work history.
// This is the ONLY approved way to inject work context into a session.
func (s *SessionContext) BuildWithWorkSummary(systemPrompt, workSummary string) []plugin.Message {
	msgs := s.ctx.Messages()

	// Prepend system message with work status
	system := systemPrompt
	if workSummary != "" {
		system += "\n\n## Current Work Status\n" + workSummary
	}

	result := make([]plugin.Message, 0, len(msgs)+1)
	result = append(result, plugin.Message{
		Role:    "system",
		Content: system,
	})
	result = append(result, msgs...)

	return result
}

// CompactIndex returns the start index for compaction.
// contents is a slice of per-message character lengths.
// contextWindow is the model's token limit.
func CompactIndex(contents []int, contextWindow int) int {
	n := len(contents)
	if n == 0 {
		return 0
	}

	charBudget := 0
	if contextWindow > 0 {
		charBudget = int(float64(contextWindow) * charsPerToken * compactThreshold)
	}

	total := 0
	for _, c := range contents {
		total += c
	}

	withinChars := charBudget == 0 || total <= charBudget
	withinMsgs := n <= maxMessages
	if withinChars && withinMsgs {
		return 0
	}

		slog.Info("compaction triggered",
		"messages", n, "chars", total, "budget", charBudget)

	startIdx := n - 1
	for i, c := range contents {
		remaining := n - i
		withinChars := charBudget == 0 || total <= charBudget
		withinMsgs := remaining <= maxMessages
		if withinChars && withinMsgs {
			startIdx = i
			break
		}
		total -= c
	}

	slog.Info("messages dropped", "dropped", startIdx, "keeping", n-startIdx)
	return startIdx
}
