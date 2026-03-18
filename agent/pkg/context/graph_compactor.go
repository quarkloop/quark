package llmctx

// =============================================================================
// graph_compactor.go  —  Decay-aware, structurally-linked compaction
//
// The GraphCompactor is the most sophisticated built-in Compactor. It extends
// the token-based eviction logic with two orthogonal improvements:
//
// 1. Structural awareness (linking):
//    A ToolCall and its ToolResult are an eviction unit — never split.
//    The compactor uses linking.BuildGraph to inspect structural links
//    and scores whole subtrees, evicting them atomically.
//
// 2. Importance decay (freshness):
//    Each message's effective score is:
//        effectiveScore = baseWeight × decayMultiplier(age, position, total)
//    Messages lose importance over time, making them progressively more
//    evictable without changing their static weight.
// =============================================================================

import (
	"sort"
	"time"

	"github.com/quarkloop/agent/pkg/context/freshness"
	"github.com/quarkloop/agent/pkg/context/linking"
)

// GraphCompactor evicts messages as structurally coherent units while
// weighting eviction priority by per-message decay functions.
//
// Zero value is invalid; use NewGraphCompactor.
type GraphCompactor struct {
	inner Compactor
	nowFn func() time.Time
}

// NewGraphCompactor returns a GraphCompactor.
//
// inner is an optional fallback Compactor applied after the graph-aware
// eviction pass. Pass nil to use only graph-aware scoring.
func NewGraphCompactor(inner Compactor) *GraphCompactor {
	return &GraphCompactor{
		inner: inner,
		nowFn: func() time.Time { return time.Now().UTC() },
	}
}

// WithClock replaces the internal clock with fn. Useful for deterministic tests.
func (gc *GraphCompactor) WithClock(fn func() time.Time) *GraphCompactor {
	gc.nowFn = fn
	return gc
}

// Compact implements the Compactor interface.
func (gc *GraphCompactor) Compact(messages []*Message, window ContextWindow) ([]*Message, error) {
	if window.IsUnbound() || !totalTokens(messages).ExceedsWindow(window) {
		return messages, nil
	}

	result := cloneMessages(messages)
	result = gc.evictByGraph(result, window)

	if gc.inner != nil && totalTokens(result).ExceedsWindow(window) {
		var err error
		result, err = gc.inner.Compact(result, window)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Preview implements CompactorWithPreview.
func (gc *GraphCompactor) Preview(messages []*Message, window ContextWindow) CompactionPreview {
	tokensBefore := totalTokens(messages)
	if window.IsUnbound() || !tokensBefore.ExceedsWindow(window) {
		return noopPreview(len(messages), tokensBefore)
	}

	clone := cloneMessages(messages)
	compacted := gc.evictByGraph(clone, window)

	if gc.inner != nil && totalTokens(compacted).ExceedsWindow(window) {
		innerPreview := PreviewCompaction(gc.inner, compacted, window)
		compactedSet := make(map[string]bool, len(compacted))
		for _, m := range compacted {
			compactedSet[m.IDString()] = true
		}
		innerEvictSet := make(map[string]bool, len(innerPreview.Evicted))
		for _, e := range innerPreview.Evicted {
			innerEvictSet[e.ID] = true
		}

		var allEvicted []EvictedMessage
		for i, m := range messages {
			if !compactedSet[m.IDString()] || innerEvictSet[m.IDString()] {
				allEvicted = append(allEvicted, makeEvicted(m, i))
			}
		}
		tokensAfter := tokensBefore
		for _, e := range allEvicted {
			tokensAfter = tokensAfter.Sub(e.Tokens)
		}
		return CompactionPreview{
			WouldCompact:    len(allEvicted) > 0,
			Evicted:         allEvicted,
			RetainedCount:   len(messages) - len(allEvicted),
			TokensBefore:    tokensBefore,
			TokensAfter:     tokensAfter,
			TokensReclaimed: tokensBefore.Sub(tokensAfter),
			WouldFit:        !tokensAfter.ExceedsWindow(window),
		}
	}

	return buildPreviewFromDiff(messages, compacted, tokensBefore, window)
}

// =============================================================================
// evictByGraph — core eviction logic
// =============================================================================

func (gc *GraphCompactor) evictByGraph(messages []*Message, window ContextWindow) []*Message {
	now := gc.nowFn()
	sysIdx := findSystemPromptIndex(messages)

	// Convert to []linking.LinkedMessage for the graph builder.
	linked := make([]linking.LinkedMessage, len(messages))
	for i, m := range messages {
		linked[i] = m
	}
	graph := linking.BuildGraph(linked)

	type rootScore struct {
		id     string
		score  float64
		tokens TokenCount
	}

	var roots []rootScore
	for i, m := range messages {
		if i == sysIdx {
			continue
		}
		rootID := m.IDString()
		if _, hasParent := graph.Parent(rootID); hasParent {
			continue // handled as part of its parent's unit
		}
		unit := graph.EvictionUnit(rootID)
		var unitScore float64
		var unitTokens TokenCount
		for _, uid := range unit {
			unitMsg := findMessageByID(messages, uid)
			if unitMsg == nil {
				continue
			}
			pos := graph.PositionOf(uid)
			if pos < 0 {
				pos = i
			}
			unitScore += unitMsg.EffectiveWeight(pos, len(messages), now)
			unitTokens = unitTokens.Add(unitMsg.TokenCount())
		}
		if len(unit) > 0 {
			unitScore /= float64(len(unit))
		}
		roots = append(roots, rootScore{id: rootID, score: unitScore, tokens: unitTokens})
	}

	sort.Slice(roots, func(a, b int) bool {
		return roots[a].score < roots[b].score
	})

	evictSet := make(map[string]bool)
	total := totalTokens(messages)
	for _, r := range roots {
		if !total.ExceedsWindow(window) {
			break
		}
		for _, uid := range graph.EvictionUnit(r.id) {
			evictSet[uid] = true
		}
		total = total.Sub(r.tokens)
	}

	return filterMessages(messages, func(_ int, m *Message) bool {
		return !evictSet[m.IDString()]
	})
}

// =============================================================================
// Helpers
// =============================================================================

// findMessageByID is a linear search helper used in the graph compactor's
// per-unit score computation where the slice is already small.
func findMessageByID(messages []*Message, id string) *Message {
	for _, m := range messages {
		if m.IDString() == id {
			return m
		}
	}
	return nil
}

// DecayAwareScorer returns a ScorerFunc that scores messages by their
// decay-modulated effective weight.
func DecayAwareScorer(messages []*Message, now time.Time) ScorerFunc {
	total := len(messages)
	posMap := graphPositionMap(messages)
	maxW := float64(HighestWeight.Value())
	if maxW == 0 {
		maxW = 1
	}
	return func(m *Message) float64 {
		pos, ok := posMap[m.ID().String()]
		if !ok {
			pos = 0
		}
		return m.EffectiveWeight(pos, total, now) / maxW
	}
}

// EvaluateMessageDecay returns the decay multiplier for m at the given
// position and time. Returns {Score: 1.0} when no decay function is set.
func EvaluateMessageDecay(m *Message, position, total int, now time.Time) freshness.DecayResult {
	if m.decayFn == nil {
		return freshness.DecayResult{Score: 1.0}
	}
	return freshness.EvaluateDecay(m.decayFn, m.createdAt.Time(), position, total, now)
}

// graphPositionMap builds a messageID→slice-position map for O(1) lookup.
func graphPositionMap(messages []*Message) map[string]int {
	m := make(map[string]int, len(messages))
	for i, msg := range messages {
		m[msg.ID().String()] = i
	}
	return m
}
