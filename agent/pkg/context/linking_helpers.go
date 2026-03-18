package llmctx

import (
	"context"

	"github.com/quarkloop/agent/pkg/context/linking"
	msg "github.com/quarkloop/agent/pkg/context/message"
)

// =============================================================================
// linking_helpers.go  —  Convenient link-building helpers for AgentContext
//
// Concept:
//   Messages in the context are a flat list, but some have natural containment
//   relationships.  A ToolResult is semantically part of its ToolCall — they
//   form one logical exchange.  An error message relates to the message that
//   caused it.  A reasoning step was triggered by a user message.
//
//   When messages are structurally linked, the GraphCompactor can:
//     - Evict them as a single unit (never orphan a ToolResult)
//     - Score them collectively (old tool exchanges decay together)
//     - Report orphaned messages after eviction
//
// Usage — manual linking:
//
//	toolCall = toolCall.WithParentMessage(nil) // root: no parent
//	toolResult = toolResult.WithParentMessage(toolCall)
//
// Usage — convenience factory:
//
//	toolCall, toolResult, err := llmctx.NewLinkedToolExchange(
//	    callID, authorID, callPayload,
//	    resultID, toolAuthorID, resultPayload, tc,
//	)
//	ac.AppendMessage(toolCall)
//	ac.AppendMessage(toolResult)
// =============================================================================

// WithParentMessage returns a copy of m with its ParentID set to parent's ID.
// The parent message is NOT modified; callers must also add m's ID to the
// parent's ChildIDs using WithChildMessage if bidirectional linking is desired.
//
// Pass nil to clear the parent link.
func (m *Message) WithParentMessage(parent *Message) *Message {
	if parent == nil {
		c := *m
		l := m.links
		l.ParentID = ""
		c.links = l
		return &c
	}
	return m.WithLinks(m.links.WithParent(parent.ID().String()))
}

// WithChildMessage returns a copy of m with childID added to its ChildIDs list.
// Use this on the parent side after calling child.WithParentMessage(parent).
func (m *Message) WithChildMessage(child *Message) *Message {
	return m.WithLinks(m.links.AddChild(child.ID().String()))
}

// LinkPair returns copies of parent and child with bidirectional containment
// links established.
//
// This is the preferred way to link any two messages because it maintains the
// invariant that parent.ChildIDs contains child.ID and child.ParentID == parent.ID.
func LinkPair(parent, child *Message) (linkedParent, linkedChild *Message) {
	linkedChild = child.WithParentMessage(parent)
	linkedParent = parent.WithChildMessage(child)
	return linkedParent, linkedChild
}

// =============================================================================
// NewLinkedToolExchange — factory for linked ToolCall+ToolResult pairs
// =============================================================================

// NewLinkedToolExchange creates a ToolCall and ToolResult with bidirectional
// containment links pre-established.
//
// The two messages together form one eviction unit: the GraphCompactor never
// evicts one without the other.
//
// Example:
//
//	call, result, err := llmctx.NewLinkedToolExchange(
//	    callID, agentAuthorID, llmctx.ToolCallPayload{...},
//	    resultID, toolAuthorID, llmctx.ToolResultPayload{...},
//	    tc,
//	)
func NewLinkedToolExchange(
	callID MessageID, callAuthorID AuthorID, callPayload msg.ToolCallPayload,
	resultID MessageID, resultAuthorID AuthorID, resultPayload msg.ToolResultPayload,
	tc TokenComputer,
) (toolCall, toolResult *Message, err error) {
	toolCall, err = NewToolCallMessage(callID, callAuthorID, callPayload, tc)
	if err != nil {
		return nil, nil, err
	}
	toolResult, err = NewToolResultMessage(resultID, resultAuthorID, resultPayload, tc)
	if err != nil {
		return nil, nil, err
	}
	toolCall, toolResult = LinkPair(toolCall, toolResult)
	return toolCall, toolResult, nil
}

// =============================================================================
// OrphanReport — detect structurally inconsistent messages
// =============================================================================

// OrphanReport describes messages whose structural parent is no longer in the context.
type OrphanReport struct {
	// OrphanIDs lists message IDs whose parent has been evicted or removed.
	OrphanIDs []MessageID
}

// HasOrphans reports whether any orphaned messages were found.
func (r OrphanReport) HasOrphans() bool { return len(r.OrphanIDs) > 0 }

// DetectOrphans scans the current context for messages whose ParentID
// references a message that is no longer present.
//
// These are typically ToolResult messages whose ToolCall was evicted.
// Callers can decide to remove them, log them, or ignore them.
func (ac *AgentContext) DetectOrphans() OrphanReport {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	linked := make([]linking.LinkedMessage, len(ac.messages))
	for i, m := range ac.messages {
		linked[i] = m
	}
	graph := linking.BuildGraph(linked)
	orphanStrs := graph.Orphans()

	ids := make([]MessageID, 0, len(orphanStrs))
	for _, s := range orphanStrs {
		id, err := NewMessageID(s)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return OrphanReport{OrphanIDs: ids}
}

// RemoveOrphans removes all messages identified as orphans.
// Returns the number of messages removed.
func (ac *AgentContext) RemoveOrphans() (int, error) {
	report := ac.DetectOrphans()
	ctx := context.Background()
	removed := 0
	for _, id := range report.OrphanIDs {
		if err := ac.RemoveMessageByID(ctx, id); err != nil {
			if ce, ok := err.(*ContextError); ok && ce.Code == ErrCodeMessageNotFound {
				continue
			}
			return removed, err
		}
		removed++
	}
	return removed, nil
}
