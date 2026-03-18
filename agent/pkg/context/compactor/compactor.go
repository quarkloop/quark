// Package compactor provides the Compactor interface and built-in implementations
// for reducing an agent context's message list to fit within a token budget.
//
// # Interface
//
// Compactor is the core interface. Every implementation accepts a []Message
// (the compactor-local view interface) and a ContextWindow, and returns a
// reduced slice without mutating the input.
//
// # Built-in Implementations
//
//   - WeightBasedCompactor   — evicts by ascending weight, then by age
//   - FIFOCompactor          — evicts oldest non-system messages first
//   - SlidingWindowCompactor — hard cap on message count
//   - ScoreCompactor         — evicts by caller-supplied relevance score
//   - PipelineCompactor      — chains strategies; short-circuits when budget met
//   - ThresholdCompactor     — activates inner compactor at a usage % threshold
//
// # Dry-run / Preview
//
// CompactorWithPreview is an optional interface for native preview support.
// PreviewCompact is the universal fallback for any Compactor.
//
// # Scorer factories
//
// RecencyScorer, WeightScorer, TypePriorityScorer, CombinedScorer are
// predefined ScorerFunc builders for use with ScoreCompactor.
package compactor

import (
	"fmt"
	"sort"
	"time"

	"github.com/quarkloop/agent/pkg/context/message"
	"github.com/quarkloop/agent/pkg/context/tokenizer"
)

// =============================================================================
// Message — the interface compactors use to inspect messages
// =============================================================================

// Message is the read-only view of a context message as seen by compactors.
// The root *llmctx.Message type satisfies this interface.
//
// Compactors never mutate messages; they only read the fields needed to make
// eviction decisions and to build CompactionPreview records.
type Message interface {
	// IDString returns the string representation of the message ID.
	IDString() string
	// Type returns the message's payload kind.
	Type() message.MessageType
	// AuthorString returns the string representation of the message author role.
	AuthorString() string
	// WeightValue returns the raw int32 eviction-priority weight.
	// Higher value = more protected from eviction.
	WeightValue() int32
	// TokenCount returns the pre-computed token count for this message.
	TokenCount() tokenizer.TokenCount
	// Content returns the flat text representation used by token scorers.
	Content() tokenizer.MessageContent
	// EffectiveWeight returns the weight modulated by any decay function.
	// position is the message's index in the slice; total is len(slice).
	EffectiveWeight(position, total int, now time.Time) float64
}

// SystemPromptType is the canonical message type for the system prompt.
// Compactors never evict the system prompt.
const SystemPromptType = message.SystemPromptType

// =============================================================================
// Compactor interface
// =============================================================================

// Compactor reduces the message list so that the total token count fits within
// a ContextWindow.
//
// Contract:
//   - Return a new slice; do not mutate the input.
//   - Preserve the system prompt (SystemPromptType) unless unavoidable.
//   - The returned slice must fit within window (or window is unbounded).
type Compactor interface {
	Compact(messages []Message, window tokenizer.ContextWindow) ([]Message, error)
}

// =============================================================================
// CompactorWithPreview
// =============================================================================

// CompactorWithPreview is implemented by compactors that support dry-run
// previews natively.
type CompactorWithPreview interface {
	Compactor
	// Preview returns what Compact would do without mutating the input.
	Preview(messages []Message, window tokenizer.ContextWindow) Preview
}

// =============================================================================
// Preview types
// =============================================================================

// EvictedMessage is a lightweight record of one message that would be removed.
type EvictedMessage struct {
	ID       string                `json:"id"`
	Type     message.MessageType   `json:"type"`
	Author   string                `json:"author"`
	Weight   int32                 `json:"weight"`
	Tokens   tokenizer.TokenCount  `json:"tokens"`
	Position int                   `json:"position"`
}

// Preview is the immutable result of a dry-run compaction.
type Preview struct {
	WouldCompact    bool                 `json:"would_compact"`
	Evicted         []EvictedMessage     `json:"evicted"`
	RetainedCount   int                  `json:"retained_count"`
	TokensBefore    tokenizer.TokenCount `json:"tokens_before"`
	TokensAfter     tokenizer.TokenCount `json:"tokens_after"`
	TokensReclaimed tokenizer.TokenCount `json:"tokens_reclaimed"`
	WouldFit        bool                 `json:"would_fit"`
}

// EvictionCount returns the number of messages that would be evicted.
func (p Preview) EvictionCount() int { return len(p.Evicted) }

// CompressionRatio returns TokensAfter / TokensBefore as a [0,1] ratio.
func (p Preview) CompressionRatio() float64 {
	if p.TokensBefore.IsZero() {
		return 1.0
	}
	return float64(p.TokensAfter.Value()) / float64(p.TokensBefore.Value())
}

// =============================================================================
// PreviewCompact — universal entry point
// =============================================================================

// PreviewCompact performs a dry-run using any Compactor.
// If c implements CompactorWithPreview, its native Preview is called.
// Otherwise Compact is called on a shallow clone and the result is diffed.
func PreviewCompact(c Compactor, messages []Message, window tokenizer.ContextWindow) Preview {
	if cwp, ok := c.(CompactorWithPreview); ok {
		return cwp.Preview(messages, window)
	}
	return previewViaClone(c, messages, window)
}

// =============================================================================
// Internal helpers
// =============================================================================

func totalTokens(messages []Message) tokenizer.TokenCount {
	var total tokenizer.TokenCount
	for _, m := range messages {
		total = total.Add(m.TokenCount())
	}
	return total
}

func findSystemPromptIndex(messages []Message) int {
	for i, m := range messages {
		if m.Type() == SystemPromptType {
			return i
		}
	}
	return -1
}

func cloneMessages(messages []Message) []Message {
	out := make([]Message, len(messages))
	copy(out, messages)
	return out
}

func filterMessages(messages []Message, keep func(i int, m Message) bool) []Message {
	out := messages[:0]
	for i, m := range messages {
		if keep(i, m) {
			out = append(out, m)
		}
	}
	return out
}

func makeEvicted(m Message, pos int) EvictedMessage {
	return EvictedMessage{
		ID:       m.IDString(),
		Type:     m.Type(),
		Author:   m.AuthorString(),
		Weight:   m.WeightValue(),
		Tokens:   m.TokenCount(),
		Position: pos,
	}
}

func noopPreview(msgCount int, tokens tokenizer.TokenCount) Preview {
	return Preview{
		WouldCompact:  false,
		RetainedCount: msgCount,
		TokensBefore:  tokens,
		TokensAfter:   tokens,
		WouldFit:      true,
	}
}

func makePreview(
	evicted []EvictedMessage,
	originalCount int,
	tokensBefore, tokensAfter tokenizer.TokenCount,
	window tokenizer.ContextWindow,
) Preview {
	return Preview{
		WouldCompact:    len(evicted) > 0,
		Evicted:         evicted,
		RetainedCount:   originalCount - len(evicted),
		TokensBefore:    tokensBefore,
		TokensAfter:     tokensAfter,
		TokensReclaimed: tokensBefore.Sub(tokensAfter),
		WouldFit:        !tokensAfter.ExceedsWindow(window),
	}
}

func previewViaClone(c Compactor, messages []Message, window tokenizer.ContextWindow) Preview {
	tokensBefore := totalTokens(messages)
	if window.IsUnbound() || !tokensBefore.ExceedsWindow(window) {
		return noopPreview(len(messages), tokensBefore)
	}
	clone := cloneMessages(messages)
	result, err := c.Compact(clone, window)
	if err != nil {
		return Preview{
			WouldCompact:  true,
			RetainedCount: len(messages),
			TokensBefore:  tokensBefore,
			TokensAfter:   tokensBefore,
		}
	}
	return buildPreviewFromDiff(messages, result, tokensBefore, window)
}

func buildPreviewFromDiff(
	original, compacted []Message,
	tokensBefore tokenizer.TokenCount,
	window tokenizer.ContextWindow,
) Preview {
	retained := make(map[string]bool, len(compacted))
	for _, m := range compacted {
		retained[m.IDString()] = true
	}
	var evicted []EvictedMessage
	for i, m := range original {
		if !retained[m.IDString()] {
			evicted = append(evicted, makeEvicted(m, i))
		}
	}
	tokensAfter := totalTokens(compacted)
	reclaimed := tokensBefore.Sub(tokensAfter)
	return Preview{
		WouldCompact:    len(evicted) > 0,
		Evicted:         evicted,
		RetainedCount:   len(compacted),
		TokensBefore:    tokensBefore,
		TokensAfter:     tokensAfter,
		TokensReclaimed: reclaimed,
		WouldFit:        !tokensAfter.ExceedsWindow(window),
	}
}

// =============================================================================
// ScorerFunc
// =============================================================================

// ScorerFunc assigns an eviction score to a message.
// Lower score = more evictable. Should return a value in [0.0, 1.0].
type ScorerFunc func(m Message) float64

// =============================================================================
// WeightBasedCompactor
// =============================================================================

// WeightBasedCompactor evicts messages in ascending weight order (lowest first),
// breaking ties by position (oldest first). The system prompt is never evicted.
type WeightBasedCompactor struct{}

// NewWeightBasedCompactor returns a WeightBasedCompactor.
func NewWeightBasedCompactor() *WeightBasedCompactor {
	return &WeightBasedCompactor{}
}

// Compact implements Compactor.
func (WeightBasedCompactor) Compact(messages []Message, window tokenizer.ContextWindow) ([]Message, error) {
	if window.IsUnbound() || !totalTokens(messages).ExceedsWindow(window) {
		return messages, nil
	}
	sysIdx := findSystemPromptIndex(messages)
	result := cloneMessages(messages)

	type candidate struct{ idx, weight int32 }
	candidates := make([]candidate, 0, len(result))
	for i, m := range result {
		if i == sysIdx {
			continue
		}
		candidates = append(candidates, candidate{idx: int32(i), weight: m.WeightValue()})
	}
	sort.Slice(candidates, func(a, b int) bool {
		if candidates[a].weight != candidates[b].weight {
			return candidates[a].weight < candidates[b].weight
		}
		return candidates[a].idx < candidates[b].idx
	})

	evict := make(map[int]bool, len(candidates))
	total := totalTokens(result)
	for _, c := range candidates {
		if !total.ExceedsWindow(window) {
			break
		}
		total = total.Sub(result[c.idx].TokenCount())
		evict[int(c.idx)] = true
	}

	out := filterMessages(result, func(i int, _ Message) bool { return !evict[i] })
	return out, nil
}

// Preview implements CompactorWithPreview.
func (WeightBasedCompactor) Preview(messages []Message, window tokenizer.ContextWindow) Preview {
	tokensBefore := totalTokens(messages)
	if window.IsUnbound() || !tokensBefore.ExceedsWindow(window) {
		return noopPreview(len(messages), tokensBefore)
	}

	sysIdx := findSystemPromptIndex(messages)
	type cand struct{ idx, weight int32 }
	candidates := make([]cand, 0, len(messages))
	for i, m := range messages {
		if i != sysIdx {
			candidates = append(candidates, cand{int32(i), m.WeightValue()})
		}
	}
	sort.Slice(candidates, func(a, b int) bool {
		if candidates[a].weight != candidates[b].weight {
			return candidates[a].weight < candidates[b].weight
		}
		return candidates[a].idx < candidates[b].idx
	})

	var evicted []EvictedMessage
	total := tokensBefore
	for _, c := range candidates {
		if !total.ExceedsWindow(window) {
			break
		}
		m := messages[c.idx]
		total = total.Sub(m.TokenCount())
		evicted = append(evicted, makeEvicted(m, int(c.idx)))
	}
	return makePreview(evicted, len(messages), tokensBefore, total, window)
}

// =============================================================================
// FIFOCompactor
// =============================================================================

// FIFOCompactor evicts the oldest non-system messages first.
type FIFOCompactor struct{}

// NewFIFOCompactor returns a FIFOCompactor.
func NewFIFOCompactor() *FIFOCompactor { return &FIFOCompactor{} }

// Compact implements Compactor.
func (FIFOCompactor) Compact(messages []Message, window tokenizer.ContextWindow) ([]Message, error) {
	if window.IsUnbound() || !totalTokens(messages).ExceedsWindow(window) {
		return messages, nil
	}
	sysIdx := findSystemPromptIndex(messages)
	result := cloneMessages(messages)

	evict := make(map[int]bool)
	total := totalTokens(result)
	for i, m := range result {
		if !total.ExceedsWindow(window) {
			break
		}
		if i == sysIdx {
			continue
		}
		total = total.Sub(m.TokenCount())
		evict[i] = true
	}

	out := filterMessages(result, func(i int, _ Message) bool { return !evict[i] })
	return out, nil
}

// Preview implements CompactorWithPreview.
func (FIFOCompactor) Preview(messages []Message, window tokenizer.ContextWindow) Preview {
	tokensBefore := totalTokens(messages)
	if window.IsUnbound() || !tokensBefore.ExceedsWindow(window) {
		return noopPreview(len(messages), tokensBefore)
	}

	sysIdx := findSystemPromptIndex(messages)
	var evicted []EvictedMessage
	total := tokensBefore
	for i, m := range messages {
		if !total.ExceedsWindow(window) {
			break
		}
		if i == sysIdx {
			continue
		}
		total = total.Sub(m.TokenCount())
		evicted = append(evicted, makeEvicted(m, i))
	}
	return makePreview(evicted, len(messages), tokensBefore, total, window)
}

// =============================================================================
// SlidingWindowCompactor
// =============================================================================

// SlidingWindowCompactor retains only the N most-recent non-system messages.
type SlidingWindowCompactor struct {
	maxMessages int
}

// NewSlidingWindowCompactor validates maxMessages and returns a compactor.
func NewSlidingWindowCompactor(maxMessages int) (*SlidingWindowCompactor, error) {
	if maxMessages <= 0 {
		return nil, fmt.Errorf("compactor: SlidingWindowCompactor maxMessages must be > 0, got %d", maxMessages)
	}
	return &SlidingWindowCompactor{maxMessages: maxMessages}, nil
}

// Compact implements Compactor.
func (s *SlidingWindowCompactor) Compact(messages []Message, _ tokenizer.ContextWindow) ([]Message, error) {
	sysIdx := findSystemPromptIndex(messages)

	var sysMsg Message
	rest := make([]Message, 0, len(messages))
	for i, m := range messages {
		if i == sysIdx {
			sysMsg = m
		} else {
			rest = append(rest, m)
		}
	}

	if len(rest) > s.maxMessages {
		rest = rest[len(rest)-s.maxMessages:]
	}

	if sysMsg == nil {
		return rest, nil
	}
	out := make([]Message, 0, len(rest)+1)
	return append(append(out, sysMsg), rest...), nil
}

// Preview implements CompactorWithPreview.
func (s *SlidingWindowCompactor) Preview(messages []Message, _ tokenizer.ContextWindow) Preview {
	tokensBefore := totalTokens(messages)
	sysIdx := findSystemPromptIndex(messages)

	type indexed struct {
		m   Message
		pos int
	}
	nonSystem := make([]indexed, 0, len(messages))
	for i, m := range messages {
		if i != sysIdx {
			nonSystem = append(nonSystem, indexed{m, i})
		}
	}
	if len(nonSystem) <= s.maxMessages {
		return noopPreview(len(messages), tokensBefore)
	}

	trimCount := len(nonSystem) - s.maxMessages
	evicted := make([]EvictedMessage, 0, trimCount)
	total := tokensBefore
	for i := 0; i < trimCount; i++ {
		m := nonSystem[i].m
		total = total.Sub(m.TokenCount())
		evicted = append(evicted, makeEvicted(m, nonSystem[i].pos))
	}
	return Preview{
		WouldCompact:    true,
		Evicted:         evicted,
		RetainedCount:   len(messages) - len(evicted),
		TokensBefore:    tokensBefore,
		TokensAfter:     total,
		TokensReclaimed: tokensBefore.Sub(total),
		WouldFit:        true,
	}
}

// MaxMessages returns the configured cap.
func (s *SlidingWindowCompactor) MaxMessages() int { return s.maxMessages }

// =============================================================================
// ScoreCompactor
// =============================================================================

// ScoreCompactor evicts the lowest-scored messages until the context fits.
type ScoreCompactor struct {
	scorer ScorerFunc
}

// NewScoreCompactor validates scorer and returns a ScoreCompactor.
func NewScoreCompactor(scorer ScorerFunc) (*ScoreCompactor, error) {
	if scorer == nil {
		return nil, fmt.Errorf("compactor: ScoreCompactor scorer must not be nil")
	}
	return &ScoreCompactor{scorer: scorer}, nil
}

// Compact implements Compactor.
func (sc *ScoreCompactor) Compact(messages []Message, window tokenizer.ContextWindow) ([]Message, error) {
	if window.IsUnbound() || !totalTokens(messages).ExceedsWindow(window) {
		return messages, nil
	}
	sysIdx := findSystemPromptIndex(messages)

	type scored struct {
		idx   int
		score float64
	}
	candidates := make([]scored, 0, len(messages))
	for i, m := range messages {
		if i == sysIdx {
			continue
		}
		candidates = append(candidates, scored{idx: i, score: sc.scorer(m)})
	}
	sort.Slice(candidates, func(a, b int) bool {
		return candidates[a].score < candidates[b].score
	})

	evict := make(map[int]bool, len(candidates))
	total := totalTokens(messages)
	for _, c := range candidates {
		if !total.ExceedsWindow(window) {
			break
		}
		total = total.Sub(messages[c.idx].TokenCount())
		evict[c.idx] = true
	}

	out := make([]Message, 0, len(messages)-len(evict))
	for i, m := range messages {
		if !evict[i] {
			out = append(out, m)
		}
	}
	return out, nil
}

// Preview implements CompactorWithPreview.
func (sc *ScoreCompactor) Preview(messages []Message, window tokenizer.ContextWindow) Preview {
	tokensBefore := totalTokens(messages)
	if window.IsUnbound() || !tokensBefore.ExceedsWindow(window) {
		return noopPreview(len(messages), tokensBefore)
	}

	sysIdx := findSystemPromptIndex(messages)
	type scored struct {
		idx   int
		score float64
	}
	candidates := make([]scored, 0, len(messages))
	for i, m := range messages {
		if i != sysIdx {
			candidates = append(candidates, scored{i, sc.scorer(m)})
		}
	}
	sort.Slice(candidates, func(a, b int) bool {
		return candidates[a].score < candidates[b].score
	})

	var evicted []EvictedMessage
	total := tokensBefore
	for _, c := range candidates {
		if !total.ExceedsWindow(window) {
			break
		}
		m := messages[c.idx]
		total = total.Sub(m.TokenCount())
		evicted = append(evicted, makeEvicted(m, c.idx))
	}
	return makePreview(evicted, len(messages), tokensBefore, total, window)
}

// =============================================================================
// PipelineCompactor
// =============================================================================

// PipelineCompactor chains Compactors in sequence.
// Processing short-circuits as soon as the context fits within the window.
type PipelineCompactor struct {
	stages []Compactor
}

// NewPipelineCompactor requires at least one stage and returns a PipelineCompactor.
func NewPipelineCompactor(stages ...Compactor) (*PipelineCompactor, error) {
	if len(stages) == 0 {
		return nil, fmt.Errorf("compactor: PipelineCompactor requires at least one stage")
	}
	for i, s := range stages {
		if s == nil {
			return nil, fmt.Errorf("compactor: PipelineCompactor stage at index %d is nil", i)
		}
	}
	return &PipelineCompactor{stages: stages}, nil
}

// Compact implements Compactor.
func (p *PipelineCompactor) Compact(messages []Message, window tokenizer.ContextWindow) ([]Message, error) {
	current := messages
	for i, stage := range p.stages {
		if !totalTokens(current).ExceedsWindow(window) {
			break
		}
		result, err := stage.Compact(current, window)
		if err != nil {
			return nil, fmt.Errorf("compactor: PipelineCompactor stage %d failed: %w", i, err)
		}
		current = result
	}
	return current, nil
}

// Preview implements CompactorWithPreview.
func (p *PipelineCompactor) Preview(messages []Message, window tokenizer.ContextWindow) Preview {
	tokensBefore := totalTokens(messages)
	if window.IsUnbound() || !tokensBefore.ExceedsWindow(window) {
		return noopPreview(len(messages), tokensBefore)
	}

	var allEvicted []EvictedMessage
	current := cloneMessages(messages)
	for _, stage := range p.stages {
		if !totalTokens(current).ExceedsWindow(window) {
			break
		}
		sp := PreviewCompact(stage, current, window)
		allEvicted = append(allEvicted, sp.Evicted...)
		evictSet := make(map[string]bool, len(sp.Evicted))
		for _, e := range sp.Evicted {
			evictSet[e.ID] = true
		}
		next := current[:0]
		for _, m := range current {
			if !evictSet[m.IDString()] {
				next = append(next, m)
			}
		}
		current = next
	}

	tokensAfter := totalTokens(current)
	return Preview{
		WouldCompact:    len(allEvicted) > 0,
		Evicted:         allEvicted,
		RetainedCount:   len(current),
		TokensBefore:    tokensBefore,
		TokensAfter:     tokensAfter,
		TokensReclaimed: tokensBefore.Sub(tokensAfter),
		WouldFit:        !tokensAfter.ExceedsWindow(window),
	}
}

// =============================================================================
// ThresholdCompactor
// =============================================================================

// ThresholdCompactor activates its inner Compactor only when token usage
// reaches a configured percentage of the context window.
type ThresholdCompactor struct {
	inner Compactor
	pct   int // 1–100
}

// NewThresholdCompactor validates pct (must be 1–100) and inner.
func NewThresholdCompactor(inner Compactor, pct int) (*ThresholdCompactor, error) {
	if inner == nil {
		return nil, fmt.Errorf("compactor: ThresholdCompactor inner must not be nil")
	}
	if pct < 1 || pct > 100 {
		return nil, fmt.Errorf("compactor: ThresholdCompactor pct must be 1–100, got %d", pct)
	}
	return &ThresholdCompactor{inner: inner, pct: pct}, nil
}

// Compact implements Compactor.
func (t *ThresholdCompactor) Compact(messages []Message, window tokenizer.ContextWindow) ([]Message, error) {
	if window.IsUnbound() {
		return messages, nil
	}
	threshold := int32(float64(window.Value()) * float64(t.pct) / 100.0)
	if totalTokens(messages).Value() < threshold {
		return messages, nil
	}
	return t.inner.Compact(messages, window)
}

// Preview implements CompactorWithPreview.
func (t *ThresholdCompactor) Preview(messages []Message, window tokenizer.ContextWindow) Preview {
	tokensBefore := totalTokens(messages)
	if !window.IsUnbound() {
		threshold := int32(float64(window.Value()) * float64(t.pct) / 100.0)
		if tokensBefore.Value() < threshold {
			return noopPreview(len(messages), tokensBefore)
		}
	}
	return PreviewCompact(t.inner, messages, window)
}

// ThresholdPct returns the configured activation percentage.
func (t *ThresholdCompactor) ThresholdPct() int { return t.pct }

// =============================================================================
// Predefined ScorerFunc factories
// =============================================================================

// RecencyScorer assigns higher scores to more-recent messages.
// Higher score = less evictable; oldest messages evicted first.
func RecencyScorer(messages []Message) ScorerFunc {
	n := float64(len(messages))
	if n == 0 {
		return func(Message) float64 { return 0 }
	}
	posMap := make(map[string]int, len(messages))
	for i, m := range messages {
		posMap[m.IDString()] = i
	}
	return func(m Message) float64 {
		pos := posMap[m.IDString()]
		return float64(pos) / n
	}
}

// WeightScorer maps WeightValue to a normalised [0,1] score.
// Higher weight → higher score → less evictable.
func WeightScorer(maxWeight int32) ScorerFunc {
	if maxWeight <= 0 {
		maxWeight = 3 // matches HighestWeight sentinel
	}
	return func(m Message) float64 {
		return float64(m.WeightValue()) / float64(maxWeight)
	}
}

// TypePriorityScorer assigns scores by MessageType using a caller-supplied map.
// Types absent from the map receive a neutral score of 0.5.
func TypePriorityScorer(priorities map[message.MessageType]float64) ScorerFunc {
	return func(m Message) float64 {
		if score, ok := priorities[m.Type()]; ok {
			return score
		}
		return 0.5
	}
}

// CombinedScorer blends multiple ScorerFuncs using equal weights.
func CombinedScorer(scorers ...ScorerFunc) ScorerFunc {
	if len(scorers) == 0 {
		return func(Message) float64 { return 0.5 }
	}
	return func(m Message) float64 {
		var sum float64
		for _, s := range scorers {
			sum += s(m)
		}
		return sum / float64(len(scorers))
	}
}
