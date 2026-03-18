package llmctx

import (
	"fmt"

	comp "github.com/quarkloop/agent/pkg/context/compactor"
)

// =============================================================================
// compactor.go  —  Root-package compactor bridge
//
// The canonical compactor implementations live in llmctx/compactor.
// This file:
//   1. Keeps the root Compactor interface (taking []*Message) for backward
//      compatibility with AgentContext.
//   2. Re-exports the compactor type names as aliases.
//   3. Provides adapted constructors that wrap llmctx/compactor implementations
//      inside a root-level Compactor.
//
// The bridge works via toCompactorMessages() which converts []*Message to
// []comp.Message (both *Message satisfies comp.Message interface).
// =============================================================================

// ---------------------------------------------------------------------------
// Root Compactor interface — kept for AgentContext compatibility
// ---------------------------------------------------------------------------

// Compactor reduces the message list so that the total token count fits within
// a ContextWindow. Implementations are provided by llmctx/compactor.
//
// Contract:
//   - Return a new slice; do not mutate the input.
//   - Preserve the system prompt (SystemPromptType) unless unavoidable.
//   - The returned slice must fit within window (or window is unbounded).
type Compactor interface {
	Compact(messages []*Message, window ContextWindow) ([]*Message, error)
}

// ScorerFunc assigns an eviction score to a message.
// Lower score = more evictable. Should return a value in [0.0, 1.0].
type ScorerFunc func(m *Message) float64

// ---------------------------------------------------------------------------
// Type aliases from llmctx/compactor (public types)
// ---------------------------------------------------------------------------

// EvictedMessage is a lightweight record of one message that would be removed.
type EvictedMessage = comp.EvictedMessage

// CompactionPreview is the immutable result of a dry-run compaction.
// Note: field types differ slightly from the sub-package (IDs are strings).
type CompactionPreview = comp.Preview

// CompactorWithPreview is implemented by compactors that support dry-run
// previews natively.
// Note: this interface uses []*Message (root slice) via the bridge.
type CompactorWithPreview interface {
	Compactor
	Preview(messages []*Message, window ContextWindow) CompactionPreview
}

// ---------------------------------------------------------------------------
// Message conversion helpers
// ---------------------------------------------------------------------------

// toCompactorMessages converts []*Message to []comp.Message for use with
// llmctx/compactor implementations.
func toCompactorMessages(messages []*Message) []comp.Message {
	out := make([]comp.Message, len(messages))
	for i, m := range messages {
		out[i] = m
	}
	return out
}

// fromCompactorMessages recovers []*Message from a []comp.Message returned
// by a sub-package compactor. Since the elements are the original *Message
// pointers (just wrapped in the interface), a simple type assertion suffices.
func fromCompactorMessages(messages []comp.Message) ([]*Message, error) {
	out := make([]*Message, len(messages))
	for i, m := range messages {
		msg, ok := m.(*Message)
		if !ok {
			return nil, fmt.Errorf("compactor: unexpected message type %T", m)
		}
		out[i] = msg
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// rootCompactorAdapter — wraps a comp.Compactor in the root Compactor interface
// ---------------------------------------------------------------------------

// rootCompactorAdapter bridges llmctx/compactor.Compactor ↔ root Compactor.
type rootCompactorAdapter struct {
	inner comp.Compactor
}

func (a *rootCompactorAdapter) Compact(messages []*Message, window ContextWindow) ([]*Message, error) {
	compMsgs := toCompactorMessages(messages)
	result, err := a.inner.Compact(compMsgs, window)
	if err != nil {
		return nil, err
	}
	return fromCompactorMessages(result)
}

// Preview implements the optional CompactorWithPreview interface if the inner
// compactor supports it.
func (a *rootCompactorAdapter) Preview(messages []*Message, window ContextWindow) CompactionPreview {
	compMsgs := toCompactorMessages(messages)
	return comp.PreviewCompact(a.inner, compMsgs, window)
}

// ---------------------------------------------------------------------------
// Internal helpers (used by compactor_preview.go and graph_compactor.go)
// ---------------------------------------------------------------------------

func findSystemPromptIndex(messages []*Message) int {
	for i, m := range messages {
		if m.Type() == SystemPromptType {
			return i
		}
	}
	return -1
}

func totalTokens(messages []*Message) TokenCount {
	var total TokenCount
	for _, m := range messages {
		total = total.Add(m.TokenCount())
	}
	return total
}

func cloneMessages(messages []*Message) []*Message {
	out := make([]*Message, len(messages))
	copy(out, messages)
	return out
}

func filterMessages(messages []*Message, keep func(i int, m *Message) bool) []*Message {
	out := messages[:0]
	for i, m := range messages {
		if keep(i, m) {
			out = append(out, m)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Public constructors — return root Compactor wrapping sub-package implementation
// ---------------------------------------------------------------------------

// NewWeightBasedCompactor returns a Compactor that evicts by ascending weight.
func NewWeightBasedCompactor() CompactorWithPreview {
	return &rootCompactorAdapter{inner: comp.NewWeightBasedCompactor()}
}

// NewFIFOCompactor returns a Compactor that evicts the oldest messages first.
func NewFIFOCompactor() CompactorWithPreview {
	return &rootCompactorAdapter{inner: comp.NewFIFOCompactor()}
}

// NewSlidingWindowCompactor validates maxMessages and returns a count-limited Compactor.
func NewSlidingWindowCompactor(maxMessages int) (CompactorWithPreview, error) {
	inner, err := comp.NewSlidingWindowCompactor(maxMessages)
	if err != nil {
		return nil, newErr(ErrCodeInvalidConfig, err.Error(), err)
	}
	return &rootCompactorAdapter{inner: inner}, nil
}

// NewScoreCompactor validates scorer and returns a score-based Compactor.
func NewScoreCompactor(scorer ScorerFunc) (CompactorWithPreview, error) {
	if scorer == nil {
		return nil, newErr(ErrCodeInvalidConfig, "ScoreCompactor: scorer must not be nil", nil)
	}
	// Wrap the root ScorerFunc into a comp.ScorerFunc.
	innerScorer := func(m comp.Message) float64 {
		msg, ok := m.(*Message)
		if !ok {
			return 0.5
		}
		return scorer(msg)
	}
	inner, err := comp.NewScoreCompactor(innerScorer)
	if err != nil {
		return nil, newErr(ErrCodeInvalidConfig, err.Error(), err)
	}
	return &rootCompactorAdapter{inner: inner}, nil
}

// NewPipelineCompactor chains Compactors in sequence.
func NewPipelineCompactor(stages ...Compactor) (CompactorWithPreview, error) {
	if len(stages) == 0 {
		return nil, newErr(ErrCodeInvalidConfig,
			"PipelineCompactor: at least one stage is required", nil)
	}
	innerStages := make([]comp.Compactor, len(stages))
	for i, s := range stages {
		if s == nil {
			return nil, newErr(ErrCodeInvalidConfig,
				fmt.Sprintf("PipelineCompactor: stage at index %d is nil", i), nil)
		}
		innerStages[i] = &rootToCompAdapter{s}
	}
	inner, err := comp.NewPipelineCompactor(innerStages...)
	if err != nil {
		return nil, newErr(ErrCodeInvalidConfig, err.Error(), err)
	}
	return &rootCompactorAdapter{inner: inner}, nil
}

// NewThresholdCompactor activates inner at a usage % threshold.
func NewThresholdCompactor(inner Compactor, pct int) (CompactorWithPreview, error) {
	if inner == nil {
		return nil, newErr(ErrCodeInvalidConfig,
			"ThresholdCompactor: inner compactor must not be nil", nil)
	}
	innerComp, err := comp.NewThresholdCompactor(&rootToCompAdapter{inner}, pct)
	if err != nil {
		return nil, newErr(ErrCodeInvalidConfig, err.Error(), err)
	}
	return &rootCompactorAdapter{inner: innerComp}, nil
}

// ---------------------------------------------------------------------------
// rootToCompAdapter — wraps a root Compactor inside comp.Compactor
// (used when nesting PipelineCompactor stages)
// ---------------------------------------------------------------------------

type rootToCompAdapter struct {
	inner Compactor
}

func (a *rootToCompAdapter) Compact(messages []comp.Message, window ContextWindow) ([]comp.Message, error) {
	rootMsgs, err := fromCompactorMessages(messages)
	if err != nil {
		return nil, err
	}
	result, err := a.inner.Compact(rootMsgs, window)
	if err != nil {
		return nil, err
	}
	return toCompactorMessages(result), nil
}

// ---------------------------------------------------------------------------
// Scorer factories — root-level wrappers
// ---------------------------------------------------------------------------

// RecencyScorer assigns higher scores to more-recent messages (less evictable).
func RecencyScorer(messages []*Message) ScorerFunc {
	n := float64(len(messages))
	if n == 0 {
		return func(*Message) float64 { return 0 }
	}
	posMap := make(map[string]int, len(messages))
	for i, m := range messages {
		posMap[m.ID().String()] = i
	}
	return func(m *Message) float64 {
		pos := posMap[m.ID().String()]
		return float64(pos) / n
	}
}

// WeightScorer maps MessageWeight to a normalised [0,1] score.
func WeightScorer(maxWeight int32) ScorerFunc {
	if maxWeight <= 0 {
		maxWeight = HighestWeight.Value()
	}
	return func(m *Message) float64 {
		return float64(m.Weight().Value()) / float64(maxWeight)
	}
}

// TypePriorityScorer assigns scores by MessageType.
func TypePriorityScorer(priorities map[MessageType]float64) ScorerFunc {
	return func(m *Message) float64 {
		if score, ok := priorities[m.Type()]; ok {
			return score
		}
		return 0.5
	}
}

// CombinedScorer blends multiple ScorerFuncs using equal weights.
func CombinedScorer(scorers ...ScorerFunc) ScorerFunc {
	if len(scorers) == 0 {
		return func(*Message) float64 { return 0.5 }
	}
	return func(m *Message) float64 {
		var sum float64
		for _, s := range scorers {
			sum += s(m)
		}
		return sum / float64(len(scorers))
	}
}
