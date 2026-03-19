// Package linking defines structural relationships between Messages.
//
// # Concept
//
// Messages in an AgentContext are stored as a flat ordered list, but they
// carry implicit relationships that the compactor and adapter layer need to
// understand:
//
//   - Containment: a ToolResult is a child of its ToolCall.  Evicting the
//     ToolCall without evicting the ToolResult leaves an orphaned result that
//     confuses the model.
//
// # Usage
//
// Attach a MessageLinks value to a Message via the parent llmctx package's
// Message.WithLinks helper.  The GraphCompactor (linking/graph.go) reads these
// links and scores/evicts messages as units rather than individuals.
//
// # Link semantics
//
//   - ParentID:   this message is contained within the parent message.
//     The pair forms an eviction unit — never evict one without
//     the other.
//   - ChildIDs:   messages this one contains (inverse of ParentID).
//     Maintained automatically when ChildIDs are set on a parent.
package linking

// =============================================================================
// MessageLinks
// =============================================================================

// MessageLinks carries all structural relationships for a single Message.
//
// Zero value is valid and means the message has no structural links.
// All ID fields are plain strings to keep this package dependency-free.
type MessageLinks struct {
	// ParentID is the ID of the message that contains this one.
	// If set, this message and its parent form a single eviction unit.
	ParentID string `json:"parent_id,omitempty"`

	// ChildIDs lists messages contained within this one.
	// These are the inverse of ParentID entries on the children.
	ChildIDs []string `json:"child_ids,omitempty"`
}

// IsZero reports whether this MessageLinks carries any relationships.
func (l MessageLinks) IsZero() bool {
	return l.ParentID == "" && len(l.ChildIDs) == 0
}

// HasParent reports whether the message has a containment parent.
func (l MessageLinks) HasParent() bool { return l.ParentID != "" }

// WithParent returns a copy of l with ParentID set to id.
// This is the symmetric counterpart to AddChild.
func (l MessageLinks) WithParent(id string) MessageLinks {
	out := l
	out.ParentID = id
	return out
}

// ClearParent returns a copy of l with ParentID cleared.
func (l MessageLinks) ClearParent() MessageLinks {
	out := l
	out.ParentID = ""
	return out
}

// Deduplicates: adding an existing ID is a no-op.
func (l MessageLinks) AddChild(id string) MessageLinks {
	for _, existing := range l.ChildIDs {
		if existing == id {
			return l
		}
	}
	out := l
	out.ChildIDs = append(append([]string(nil), l.ChildIDs...), id)
	return out
}

// RemoveChild returns a copy of l with id removed from ChildIDs.
func (l MessageLinks) RemoveChild(id string) MessageLinks {
	out := l
	out.ChildIDs = filterStrings(l.ChildIDs, func(s string) bool { return s != id })
	return out
}

func filterStrings(ss []string, keep func(string) bool) []string {
	out := ss[:0:0]
	for _, s := range ss {
		if keep(s) {
			out = append(out, s)
		}
	}
	return out
}
