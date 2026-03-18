package llmctx

// =============================================================================
// graph_compactor.go  —  Root-package bridge for GraphCompactor
//
// M2: The canonical GraphCompactor implementation lives in
// llmctx/compactor (compactor/graph_compactor.go).
//
// This file:
//   1. Provides NewGraphCompactor as the public constructor.
//      Returns a CompactorWithPreview wrapping the sub-package implementation.
//   2. Re-exports DecayAwareScorer and EvaluateMessageDecay as root helpers.
// =============================================================================

import (
	"time"

	comp "github.com/quarkloop/agent/pkg/context/compactor"
	"github.com/quarkloop/agent/pkg/context/freshness"
)

// NewGraphCompactor returns a CompactorWithPreview that evicts messages as
// structurally coherent units, weighted by per-message decay functions.
//
// inner is applied as a fallback if graph-aware eviction alone cannot satisfy
// the window. Pass nil to use only graph-aware scoring.
func NewGraphCompactor(inner Compactor) CompactorWithPreview {
	var innerComp comp.Compactor
	if inner != nil {
		innerComp = &rootToCompAdapter{inner}
	}
	gc := comp.NewGraphCompactor(innerComp)
	return &rootCompactorAdapter{inner: gc}
}

// DecayAwareScorer returns a ScorerFunc that scores messages by their
// decay-modulated effective weight. Use it to upgrade a ScoreCompactor:
//
//	scorer := llmctx.DecayAwareScorer(messages, time.Now().UTC())
//	sc, _ := llmctx.NewScoreCompactor(scorer)
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
