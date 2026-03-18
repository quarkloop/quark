package llmctx

import (
	"fmt"
	"sort"

	"github.com/quarkloop/agent/pkg/context/message"
)

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
	Compact(messages []*Message, window ContextWindow) ([]*Message, error)
}

// ScorerFunc assigns an eviction score to a message.
// Lower score = more evictable. Should return a value in [0.0, 1.0].
type ScorerFunc func(m *Message) float64

// =============================================================================
// Preview types
// =============================================================================

// EvictedMessage is a lightweight record of one message that would be removed.
type EvictedMessage struct {
	ID       string      `json:"id"`
	Type     MessageType `json:"type"`
	Author   string      `json:"author"`
	Weight   int32       `json:"weight"`
	Tokens   TokenCount  `json:"tokens"`
	Position int         `json:"position"`
}

// CompactionPreview is the immutable result of a dry-run compaction.
type CompactionPreview struct {
	WouldCompact    bool           `json:"would_compact"`
	Evicted         []EvictedMessage `json:"evicted"`
	RetainedCount   int            `json:"retained_count"`
	TokensBefore    TokenCount     `json:"tokens_before"`
	TokensAfter     TokenCount     `json:"tokens_after"`
	TokensReclaimed TokenCount     `json:"tokens_reclaimed"`
	WouldFit        bool           `json:"would_fit"`
}

// EvictionCount returns the number of messages that would be evicted.
func (p CompactionPreview) EvictionCount() int { return len(p.Evicted) }

// CompressionRatio returns TokensAfter / TokensBefore as a [0,1] ratio.
func (p CompactionPreview) CompressionRatio() float64 {
	if p.TokensBefore.IsZero() {
		return 1.0
	}
	return float64(p.TokensAfter.Value()) / float64(p.TokensBefore.Value())
}

// CompactorWithPreview is implemented by compactors that support dry-run
// previews natively.
type CompactorWithPreview interface {
	Compactor
	Preview(messages []*Message, window ContextWindow) CompactionPreview
}

// =============================================================================
// Internal helpers
// =============================================================================

func totalTokens(messages []*Message) TokenCount {
	var total TokenCount
	for _, m := range messages {
		total = total.Add(m.TokenCount())
	}
	return total
}

func findSystemPromptIndex(messages []*Message) int {
	for i, m := range messages {
		if m.Type() == SystemPromptType {
			return i
		}
	}
	return -1
}

func cloneMessages(messages []*Message) []*Message {
	out := make([]*Message, len(messages))
	copy(out, messages)
	return out
}

func filterMessages(messages []*Message, keep func(i int, m *Message) bool) []*Message {
	out := make([]*Message, 0, len(messages))
	for i, m := range messages {
		if keep(i, m) {
			out = append(out, m)
		}
	}
	return out
}

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

func (WeightBasedCompactor) Compact(messages []*Message, window ContextWindow) ([]*Message, error) {
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
		candidates = append(candidates, candidate{idx: int32(i), weight: m.Weight().Value()})
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

	return filterMessages(result, func(i int, _ *Message) bool { return !evict[i] }), nil
}

func (WeightBasedCompactor) Preview(messages []*Message, window ContextWindow) CompactionPreview {
	tokensBefore := totalTokens(messages)
	if window.IsUnbound() || !tokensBefore.ExceedsWindow(window) {
		return noopPreview(len(messages), tokensBefore)
	}

	sysIdx := findSystemPromptIndex(messages)
	type cand struct{ idx, weight int32 }
	candidates := make([]cand, 0, len(messages))
	for i, m := range messages {
		if i != sysIdx {
			candidates = append(candidates, cand{int32(i), m.Weight().Value()})
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

func NewFIFOCompactor() *FIFOCompactor { return &FIFOCompactor{} }

func (FIFOCompactor) Compact(messages []*Message, window ContextWindow) ([]*Message, error) {
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

	return filterMessages(result, func(i int, _ *Message) bool { return !evict[i] }), nil
}

func (FIFOCompactor) Preview(messages []*Message, window ContextWindow) CompactionPreview {
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

func NewSlidingWindowCompactor(maxMessages int) (*SlidingWindowCompactor, error) {
	if maxMessages <= 0 {
		return nil, newErr(ErrCodeInvalidConfig,
			fmt.Sprintf("SlidingWindowCompactor maxMessages must be > 0, got %d", maxMessages), nil)
	}
	return &SlidingWindowCompactor{maxMessages: maxMessages}, nil
}

func (s *SlidingWindowCompactor) Compact(messages []*Message, _ ContextWindow) ([]*Message, error) {
	sysIdx := findSystemPromptIndex(messages)

	var sysMsg *Message
	rest := make([]*Message, 0, len(messages))
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
	out := make([]*Message, 0, len(rest)+1)
	return append(append(out, sysMsg), rest...), nil
}

func (s *SlidingWindowCompactor) Preview(messages []*Message, _ ContextWindow) CompactionPreview {
	tokensBefore := totalTokens(messages)
	sysIdx := findSystemPromptIndex(messages)

	type indexed struct {
		m   *Message
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
	return CompactionPreview{
		WouldCompact:    true,
		Evicted:         evicted,
		RetainedCount:   len(messages) - len(evicted),
		TokensBefore:    tokensBefore,
		TokensAfter:     total,
		TokensReclaimed: tokensBefore.Sub(total),
		WouldFit:        true,
	}
}

func (s *SlidingWindowCompactor) MaxMessages() int { return s.maxMessages }

// =============================================================================
// ScoreCompactor
// =============================================================================

// ScoreCompactor evicts the lowest-scored messages until the context fits.
type ScoreCompactor struct {
	scorer ScorerFunc
}

func NewScoreCompactor(scorer ScorerFunc) (*ScoreCompactor, error) {
	if scorer == nil {
		return nil, newErr(ErrCodeInvalidConfig, "ScoreCompactor: scorer must not be nil", nil)
	}
	return &ScoreCompactor{scorer: scorer}, nil
}

func (sc *ScoreCompactor) Compact(messages []*Message, window ContextWindow) ([]*Message, error) {
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

	return filterMessages(messages, func(i int, _ *Message) bool { return !evict[i] }), nil
}

func (sc *ScoreCompactor) Preview(messages []*Message, window ContextWindow) CompactionPreview {
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
type PipelineCompactor struct {
	stages []Compactor
}

func NewPipelineCompactor(stages ...Compactor) (*PipelineCompactor, error) {
	if len(stages) == 0 {
		return nil, newErr(ErrCodeInvalidConfig,
			"PipelineCompactor: at least one stage is required", nil)
	}
	for i, s := range stages {
		if s == nil {
			return nil, newErr(ErrCodeInvalidConfig,
				fmt.Sprintf("PipelineCompactor: stage at index %d is nil", i), nil)
		}
	}
	return &PipelineCompactor{stages: stages}, nil
}

func (p *PipelineCompactor) Compact(messages []*Message, window ContextWindow) ([]*Message, error) {
	current := messages
	for i, stage := range p.stages {
		if !totalTokens(current).ExceedsWindow(window) {
			break
		}
		result, err := stage.Compact(current, window)
		if err != nil {
			return nil, fmt.Errorf("PipelineCompactor stage %d failed: %w", i, err)
		}
		current = result
	}
	return current, nil
}

func (p *PipelineCompactor) Preview(messages []*Message, window ContextWindow) CompactionPreview {
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
		sp := PreviewCompaction(stage, current, window)
		allEvicted = append(allEvicted, sp.Evicted...)
		evictSet := make(map[string]bool, len(sp.Evicted))
		for _, e := range sp.Evicted {
			evictSet[e.ID] = true
		}
		next := make([]*Message, 0, len(current))
		for _, m := range current {
			if !evictSet[m.ID().String()] {
				next = append(next, m)
			}
		}
		current = next
	}

	tokensAfter := totalTokens(current)
	return CompactionPreview{
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

func NewThresholdCompactor(inner Compactor, pct int) (*ThresholdCompactor, error) {
	if inner == nil {
		return nil, newErr(ErrCodeInvalidConfig,
			"ThresholdCompactor: inner compactor must not be nil", nil)
	}
	if pct < 1 || pct > 100 {
		return nil, newErr(ErrCodeInvalidConfig,
			fmt.Sprintf("ThresholdCompactor: pct must be 1–100, got %d", pct), nil)
	}
	return &ThresholdCompactor{inner: inner, pct: pct}, nil
}

func (t *ThresholdCompactor) Compact(messages []*Message, window ContextWindow) ([]*Message, error) {
	if window.IsUnbound() {
		return messages, nil
	}
	threshold := int32(float64(window.Value()) * float64(t.pct) / 100.0)
	if totalTokens(messages).Value() < threshold {
		return messages, nil
	}
	return t.inner.Compact(messages, window)
}

func (t *ThresholdCompactor) Preview(messages []*Message, window ContextWindow) CompactionPreview {
	tokensBefore := totalTokens(messages)
	if !window.IsUnbound() {
		threshold := int32(float64(window.Value()) * float64(t.pct) / 100.0)
		if tokensBefore.Value() < threshold {
			return noopPreview(len(messages), tokensBefore)
		}
	}
	return PreviewCompaction(t.inner, messages, window)
}

func (t *ThresholdCompactor) ThresholdPct() int { return t.pct }

// =============================================================================
// Scorer factories
// =============================================================================

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
func TypePriorityScorer(priorities map[message.MessageType]float64) ScorerFunc {
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
