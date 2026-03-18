package compactor

// =============================================================================
// graph_compactor.go  —  Decay-aware, structurally-linked compaction
//
// M2: Migrated from the root llmctx package into llmctx/compactor.
//
// The GraphCompactor is the most sophisticated built-in Compactor. It extends
// the token-based eviction logic with two orthogonal improvements:
//
// 1. Structural awareness (linking):
//    A ToolCall and its ToolResult are an eviction unit — never split.
//    The compactor uses the GraphMessage interface to inspect structural links
//    and scores whole subtrees, evicting them atomically.
//
// 2. Importance decay (freshness):
//    Each message's effective score is:
//        effectiveScore = baseWeight × decayMultiplier(age, position, total)
//    Messages lose importance over time, making them progressively more
//    evictable without changing their static weight.
//
// The two features compose cleanly: a ToolCall/ToolResult pair has a combined
// decay score (the average of both), so old tool exchanges decay together.
// =============================================================================

import (
	"sort"
	"time"

	"github.com/quarkloop/agent/pkg/context/linking"
	"github.com/quarkloop/agent/pkg/context/tokenizer"
)

// GraphMessage extends Message with the structural link information needed
// by the GraphCompactor. The root *llmctx.Message type satisfies this interface.
type GraphMessage interface {
	Message
	// MessageID returns the stable unique identifier string (same as IDString()).
	// Required for linking.LinkedMessage compatibility.
	MessageID() string
	// Links returns the structural relationships attached to this message.
	Links() linking.MessageLinks
}

// GraphCompactor evicts messages as structurally coherent units while
// weighting eviction priority by per-message decay functions.
//
// Zero value is invalid; use NewGraphCompactor.
type GraphCompactor struct {
	// inner is the fallback strategy applied after the graph-aware pass.
	// When nil, units are scored purely by effective weight.
	inner Compactor

	// nowFn returns the current time.  Defaults to time.Now().UTC().
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
// Messages must implement GraphMessage; non-GraphMessage entries are passed
// through the fallback inner compactor without graph-aware scoring.
func (gc *GraphCompactor) Compact(messages []Message, window tokenizer.ContextWindow) ([]Message, error) {
	if window.IsUnbound() || !totalTokens(messages).ExceedsWindow(window) {
		return messages, nil
	}

	result := cloneMessages(messages)
	result = gc.evictByGraph(result, window)

	// If we still exceed the window after graph-aware eviction, fall back to inner.
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
func (gc *GraphCompactor) Preview(messages []Message, window tokenizer.ContextWindow) Preview {
	tokensBefore := totalTokens(messages)
	if window.IsUnbound() || !tokensBefore.ExceedsWindow(window) {
		return noopPreview(len(messages), tokensBefore)
	}

	clone := cloneMessages(messages)
	compacted := gc.evictByGraph(clone, window)

	// If still over budget and we have an inner compactor, continue previewing.
	if gc.inner != nil && totalTokens(compacted).ExceedsWindow(window) {
		innerPreview := PreviewCompact(gc.inner, compacted, window)
		// Build merged eviction set from both passes.
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
		return Preview{
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

func (gc *GraphCompactor) evictByGraph(messages []Message, window tokenizer.ContextWindow) []Message {
	now := gc.nowFn()
	sysIdx := findSystemPromptIndex(messages)

	// Lift messages to GraphMessage if they support it; others get no link info.
	graphMsgs := toGraphMessages(messages)

	// Convert to []linking.LinkedMessage for the graph builder.
	linked := make([]linking.LinkedMessage, len(graphMsgs))
	for i, gm := range graphMsgs {
		linked[i] = gm
	}
	graph := linking.BuildGraph(linked)

	type rootScore struct {
		id     string
		score  float64
		tokens tokenizer.TokenCount
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
		var unitTokens tokenizer.TokenCount
		for _, uid := range unit {
			unitMsg := findByIDString(messages, uid)
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

	// Sort ascending by score (lowest = most evictable).
	sort.Slice(roots, func(a, b int) bool {
		return roots[a].score < roots[b].score
	})

	// Evict units until we fit.
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

	return filterMessages(messages, func(_ int, m Message) bool {
		return !evictSet[m.IDString()]
	})
}

// =============================================================================
// graphMessageAdapter — wraps a Message with empty link info when not GraphMessage
// =============================================================================

type graphMessageAdapter struct {
	Message
}

func (a graphMessageAdapter) MessageID() string             { return a.IDString() }
func (a graphMessageAdapter) Links() linking.MessageLinks   { return linking.MessageLinks{} }

func toGraphMessages(messages []Message) []GraphMessage {
	out := make([]GraphMessage, len(messages))
	for i, m := range messages {
		if gm, ok := m.(GraphMessage); ok {
			out[i] = gm
		} else {
			out[i] = graphMessageAdapter{m}
		}
	}
	return out
}

// findByIDString is a linear search helper used in the graph compactor's
// per-unit score computation where the slice is already small.
func findByIDString(messages []Message, id string) Message {
	for _, m := range messages {
		if m.IDString() == id {
			return m
		}
	}
	return nil
}

// =============================================================================
// DecayAwareScorer — adds decay scoring to any ScorerFunc-based compactor
// =============================================================================

// DecayAwareScorer returns a ScorerFunc that scores messages by their decay-
// modulated effective weight. Use it to upgrade a ScoreCompactor with decay
// awareness:
//
//	scorer := compactor.DecayAwareScorer(messages, time.Now().UTC(), maxWeight)
//	sc, _ := compactor.NewScoreCompactor(scorer)
func DecayAwareScorer(messages []Message, now time.Time, maxWeight float64) ScorerFunc {
	total := len(messages)
	posMap := make(map[string]int, total)
	for i, m := range messages {
		posMap[m.IDString()] = i
	}
	if maxWeight <= 0 {
		maxWeight = 1
	}
	return func(m Message) float64 {
		pos, ok := posMap[m.IDString()]
		if !ok {
			pos = 0
		}
		return m.EffectiveWeight(pos, total, now) / maxWeight
	}
}
