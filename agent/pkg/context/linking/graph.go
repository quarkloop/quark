package linking

import "fmt"

// =============================================================================
// LinkedMessage — a message node in the graph
// =============================================================================

// LinkedMessage is the minimal interface that graph operations require.
// The parent llmctx package's Message type satisfies this interface.
type LinkedMessage interface {
	// MessageID returns the message's unique identifier as a string.
	MessageID() string
	// Links returns the structural relationships attached to this message.
	Links() MessageLinks
}

// =============================================================================
// LinkGraph
// =============================================================================

// LinkGraph is a directed containment + reference graph built from a flat
// message slice.  It is computed once per compaction cycle and discarded.
//
// Use cases:
//   - GraphCompactor: score and evict messages as structural units.
//   - Adapter rendering: inline children into their parent's representation.
//   - Orphan detection: find ToolResult messages whose ToolCall was evicted.
type LinkGraph struct {
	// nodes maps message ID → node
	nodes map[string]*graphNode
	// roots is the ordered list of messages with no parent
	roots []string
}

type graphNode struct {
	id       string
	msg      LinkedMessage
	children []*graphNode
	parent   *graphNode
}

// BuildGraph constructs a LinkGraph from an ordered message slice.
// Messages without links become root nodes.
// References are recorded but do not affect the tree structure.
func BuildGraph(messages []LinkedMessage) *LinkGraph {
	g := &LinkGraph{nodes: make(map[string]*graphNode, len(messages))}

	// First pass: create all nodes.
	for _, m := range messages {
		g.nodes[m.MessageID()] = &graphNode{id: m.MessageID(), msg: m}
	}

	// Second pass: wire parent→child edges.
	for _, m := range messages {
		links := m.Links()
		if !links.HasParent() {
			g.roots = append(g.roots, m.MessageID())
			continue
		}
		parentNode, ok := g.nodes[links.ParentID]
		if !ok {
			// Parent evicted or not in window — treat as root.
			g.roots = append(g.roots, m.MessageID())
			continue
		}
		childNode := g.nodes[m.MessageID()]
		childNode.parent = parentNode
		parentNode.children = append(parentNode.children, childNode)
	}
	return g
}

// EvictionUnit returns all message IDs that must be evicted together when id
// is selected for eviction.
//
// The unit is: the message itself + all its descendants (children, grandchildren, …).
// If the message has a parent, the parent is NOT included — evicting a child
// does not force eviction of the parent (but it may leave an orphaned parent,
// which the orphan checker can detect separately).
func (g *LinkGraph) EvictionUnit(id string) []string {
	node, ok := g.nodes[id]
	if !ok {
		return []string{id}
	}
	var unit []string
	collectSubtree(node, &unit)
	return unit
}

func collectSubtree(n *graphNode, out *[]string) {
	*out = append(*out, n.id)
	for _, child := range n.children {
		collectSubtree(child, out)
	}
}

// ContainmentTokens returns the total token count for all messages in the
// eviction unit rooted at id.  The tokener function maps message ID → token count.
func (g *LinkGraph) ContainmentTokens(id string, tokener func(string) int) int {
	unit := g.EvictionUnit(id)
	total := 0
	for _, uid := range unit {
		total += tokener(uid)
	}
	return total
}

// Orphans returns the IDs of messages whose parent is not present in the graph.
// These are typically ToolResult messages whose ToolCall was already evicted.
func (g *LinkGraph) Orphans() []string {
	var orphans []string
	for id, node := range g.nodes {
		parentID := node.msg.Links().ParentID
		if parentID == "" {
			continue
		}
		if _, exists := g.nodes[parentID]; !exists {
			orphans = append(orphans, id)
		}
	}
	return orphans
}

// Children returns the direct children of id in the containment tree.
func (g *LinkGraph) Children(id string) []string {
	node, ok := g.nodes[id]
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(node.children))
	for _, c := range node.children {
		ids = append(ids, c.id)
	}
	return ids
}

// Parent returns the parent ID of id, or ("", false) if it has no parent.
func (g *LinkGraph) Parent(id string) (string, bool) {
	node, ok := g.nodes[id]
	if !ok || node.parent == nil {
		return "", false
	}
	return node.parent.id, true
}

// Roots returns the ordered list of root message IDs (messages with no parent).
func (g *LinkGraph) Roots() []string {
	out := make([]string, len(g.roots))
	copy(out, g.roots)
	return out
}

// Has reports whether id is present in the graph.
func (g *LinkGraph) Has(id string) bool {
	_, ok := g.nodes[id]
	return ok
}

// PositionOf returns the zero-based position of id within the roots slice,
// or -1 when id is not a root (it has a parent) or is not in the graph.
// Used by GraphCompactor to compute position-based decay scores.
func (g *LinkGraph) PositionOf(id string) int {
	for i, rid := range g.roots {
		if rid == id {
			return i
		}
	}
	return -1
}

// Size returns the total number of messages in the graph.
func (g *LinkGraph) Size() int { return len(g.nodes) }

// Validate checks the graph for consistency and returns a list of problems.
// Useful in tests and developer tooling.
func (g *LinkGraph) Validate() []string {
	var problems []string
	for id, node := range g.nodes {
		parentID := node.msg.Links().ParentID
		if parentID != "" {
			if _, ok := g.nodes[parentID]; !ok {
				problems = append(problems, fmt.Sprintf(
					"message %q has parent %q but parent is not in graph", id, parentID))
			}
		}
		for _, childID := range node.msg.Links().ChildIDs {
			if _, ok := g.nodes[childID]; !ok {
				problems = append(problems, fmt.Sprintf(
					"message %q lists child %q but child is not in graph", id, childID))
			}
		}
	}
	return problems
}
