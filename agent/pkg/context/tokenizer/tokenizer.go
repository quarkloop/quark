// Package tokenizer provides the TokenComputer interface and all built-in
// implementations for estimating LLM token counts.
//
// It also defines the primitive value types that token counting depends on:
// MessageContent, TokenCount, and ContextWindow. These live here rather than
// in the root package so that llmctx/tokenizer is a pure leaf dependency —
// it imports only the standard library, and everything else imports it.
//
// # Implementations
//
//   - WordApproxTokenComputer  — fast English heuristic (~0.75 words/token)
//   - CharApproxTokenComputer  — language-agnostic rune-based estimate
//   - CL100KTokenComputer      — approximates OpenAI cl100k_base BPE vocab
//   - AnthropicTokenComputer   — Anthropic's documented 3.5 bytes/token rule
//   - CachedTokenComputer      — LRU memoisation decorator (R5, R18 stats)
//   - CompositeTokenComputer   — weighted blend of multiple computers
//   - ClampedTokenComputer     — enforces [min, max] bounds on any delegate
//
// # Usage
//
//	tc, err := tokenizer.Default()             // WordApprox with LRU cache
//	tc, err := tokenizer.NewCached(tokenizer.CL100KTokenComputer{}, 4096)
//	count, err := tc.Compute(tokenizer.NewContent("hello world"))
package tokenizer

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// =============================================================================
// MessageContent
// =============================================================================

// MessageContent holds the textual representation of a message.
// All access is through exported methods; the internal representation is opaque.
type MessageContent struct {
	value string
}

// NewContent wraps a raw string in a MessageContent.
func NewContent(v string) MessageContent {
	return MessageContent{value: v}
}

// String returns the raw string payload.
func (c MessageContent) String() string { return c.value }

// IsEmpty reports whether the content is empty.
func (c MessageContent) IsEmpty() bool { return c.value == "" }

// RuneCount returns the number of Unicode code points.
func (c MessageContent) RuneCount() int { return utf8.RuneCountInString(c.value) }

// ByteLen returns the byte length of the UTF-8 encoded string.
func (c MessageContent) ByteLen() int { return len(c.value) }

// WordCount returns the number of whitespace-delimited words.
func (c MessageContent) WordCount() int { return len(strings.Fields(c.value)) }

// MarshalJSON encodes the content as a JSON string.
func (c MessageContent) MarshalJSON() ([]byte, error) { return json.Marshal(c.value) }

// UnmarshalJSON decodes the content from a JSON string.
func (c *MessageContent) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &c.value)
}

// =============================================================================
// TokenCount
// =============================================================================

// TokenCount is a validated, non-negative count of LLM tokens.
type TokenCount struct {
	value int32
}

// NewTokenCount validates v and returns a TokenCount.
func NewTokenCount(v int32) (TokenCount, error) {
	if v < 0 {
		return TokenCount{}, fmt.Errorf("tokenizer: token count must be >= 0, got %d", v)
	}
	return TokenCount{value: v}, nil
}

// MustTokenCount panics if v is invalid. Intended for constants.
func MustTokenCount(v int32) TokenCount {
	tc, err := NewTokenCount(v)
	if err != nil {
		panic(err)
	}
	return tc
}

// Value returns the raw int32 token count.
func (t TokenCount) Value() int32 { return t.value }

// IsZero reports whether the count is zero.
func (t TokenCount) IsZero() bool { return t.value == 0 }

// Add returns t + o.
func (t TokenCount) Add(o TokenCount) TokenCount {
	return TokenCount{value: t.value + o.value}
}

// Sub returns t - o, flooring at zero.
func (t TokenCount) Sub(o TokenCount) TokenCount {
	if o.value > t.value {
		return TokenCount{}
	}
	return TokenCount{value: t.value - o.value}
}

// ExceedsWindow reports whether t exceeds the context window budget.
func (t TokenCount) ExceedsWindow(w ContextWindow) bool {
	return !w.IsUnbound() && t.value > w.value
}

// MarshalJSON encodes the count as a JSON number.
func (t TokenCount) MarshalJSON() ([]byte, error) { return json.Marshal(t.value) }

// UnmarshalJSON decodes the count from a JSON number.
func (t *TokenCount) UnmarshalJSON(b []byte) error { return json.Unmarshal(b, &t.value) }

// =============================================================================
// ContextWindow
// =============================================================================

// ContextWindow is the maximum token budget the LLM accepts for one request.
// A zero value means unbounded.
type ContextWindow struct {
	value int32
}

// NewContextWindow validates v and returns a ContextWindow.
func NewContextWindow(v int32) (ContextWindow, error) {
	if v < 0 {
		return ContextWindow{}, fmt.Errorf("tokenizer: context window must be >= 0, got %d", v)
	}
	return ContextWindow{value: v}, nil
}

// Value returns the raw int32 window size.
func (cw ContextWindow) Value() int32 { return cw.value }

// IsUnbound reports whether the window is unbounded (zero value).
func (cw ContextWindow) IsUnbound() bool { return cw.value == 0 }

// UsagePct returns the percentage of the window consumed by t (0.0–100.0+).
// Returns 0 when the window is unbounded.
func (cw ContextWindow) UsagePct(t TokenCount) float64 {
	if cw.IsUnbound() || cw.value == 0 {
		return 0
	}
	return float64(t.value) / float64(cw.value) * 100
}

// =============================================================================
// TokenComputer interface
// =============================================================================

// TokenComputer computes the approximate LLM token count for a MessageContent.
// Inject the same instance into every Message and AgentContext to ensure
// token counts are consistent across the system.
type TokenComputer interface {
	Compute(content MessageContent) (TokenCount, error)
}

// =============================================================================
// WordApproxTokenComputer
// =============================================================================

// WordApproxTokenComputer uses the OpenAI rule-of-thumb: ~0.75 words per token.
// Fast and dependency-free; suitable for English text.
type WordApproxTokenComputer struct{}

// Compute implements TokenComputer.
func (WordApproxTokenComputer) Compute(c MessageContent) (TokenCount, error) {
	if c.IsEmpty() {
		return TokenCount{}, nil
	}
	words := c.WordCount()
	approx := int32((float64(words)/0.75 + 0.5))
	if approx < 1 {
		approx = 1
	}
	return NewTokenCount(approx)
}

// =============================================================================
// CharApproxTokenComputer
// =============================================================================

// CharApproxTokenComputer divides the Unicode rune count by 4.
// Language-agnostic; works reasonably well for mixed-script content.
type CharApproxTokenComputer struct{}

// Compute implements TokenComputer.
func (CharApproxTokenComputer) Compute(c MessageContent) (TokenCount, error) {
	if c.IsEmpty() {
		return TokenCount{}, nil
	}
	approx := int32(c.RuneCount()/4 + 1)
	return NewTokenCount(approx)
}

// =============================================================================
// CL100KTokenComputer
// =============================================================================

// CL100KTokenComputer approximates the cl100k_base BPE vocabulary used by
// GPT-3.5-turbo, GPT-4, and text-embedding-ada-002.
//
// Heuristics:
//   - ASCII alphanumeric runs  → 1 token per 4 chars
//   - Whitespace runs          → 1 token
//   - ASCII punctuation/symbol → 1 token each
//   - Non-ASCII (CJK, emoji)   → 2 tokens each (conservative)
//
// For production accuracy use github.com/pkoukk/tiktoken-go.
type CL100KTokenComputer struct{}

// Compute implements TokenComputer.
func (CL100KTokenComputer) Compute(c MessageContent) (TokenCount, error) {
	if c.IsEmpty() {
		return TokenCount{}, nil
	}
	var tokens int32
	var asciiRun int32

	flush := func() {
		if asciiRun > 0 {
			tokens += (asciiRun + 3) / 4
			asciiRun = 0
		}
	}

	for _, r := range c.String() {
		switch {
		case r <= 127 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			asciiRun++
		case r == ' ' || r == '\t' || r == '\n':
			flush()
			tokens++
		case r <= 127:
			flush()
			tokens++
		default:
			flush()
			tokens += 2
		}
	}
	flush()
	if tokens < 1 {
		tokens = 1
	}
	return NewTokenCount(tokens)
}

// =============================================================================
// AnthropicTokenComputer
// =============================================================================

// AnthropicTokenComputer applies Anthropic's documented estimate:
// 1 token ≈ 3.5 bytes, so tokens = ceil(bytes / 3.5) = ceil(2×bytes / 7).
type AnthropicTokenComputer struct{}

// Compute implements TokenComputer.
func (AnthropicTokenComputer) Compute(c MessageContent) (TokenCount, error) {
	if c.IsEmpty() {
		return TokenCount{}, nil
	}
	byteLen := int32(c.ByteLen())
	approx := (2*byteLen + 6) / 7
	if approx < 1 {
		approx = 1
	}
	return NewTokenCount(approx)
}

// =============================================================================
// CachedTokenComputer  (R5: O(1) LRU, R18: Stats)
// =============================================================================

// lruEntry is the value stored in each LRU list element.
type lruEntry struct {
	key   string
	count TokenCount
	hits  int64
}

// CachedTokenComputer is a memoising LRU decorator over any TokenComputer.
//
// Cache key: SHA-256 of the UTF-8 content string, hex-encoded.
// MaxEntries = 0 means unbounded.
// Safe for concurrent use from multiple goroutines.
type CachedTokenComputer struct {
	delegate   TokenComputer
	mu         sync.Mutex
	cache      map[string]*list.Element
	lru        *list.List
	MaxEntries int

	// R18: aggregate counters, updated under mu.
	totalHits      int64
	totalMisses    int64
	totalEvictions int64
}

// NewCached wraps delegate with an LRU memoisation layer.
// maxEntries = 0 disables eviction (unbounded cache).
func NewCached(delegate TokenComputer, maxEntries int) (*CachedTokenComputer, error) {
	if delegate == nil {
		return nil, fmt.Errorf("tokenizer: CachedTokenComputer delegate must not be nil")
	}
	return &CachedTokenComputer{
		delegate:   delegate,
		cache:      make(map[string]*list.Element),
		lru:        list.New(),
		MaxEntries: maxEntries,
	}, nil
}

func cacheKey(c MessageContent) string {
	h := sha256.Sum256([]byte(c.String()))
	return hex.EncodeToString(h[:])
}

// Compute returns the memoised token count, delegating on a miss.
func (cc *CachedTokenComputer) Compute(c MessageContent) (TokenCount, error) {
	key := cacheKey(c)

	cc.mu.Lock()
	if elem, ok := cc.cache[key]; ok {
		cc.lru.MoveToFront(elem)
		entry := elem.Value.(*lruEntry)
		entry.hits++
		cc.totalHits++
		count := entry.count
		cc.mu.Unlock()
		return count, nil
	}
	cc.mu.Unlock()

	// Cache miss: compute outside the lock.
	count, err := cc.delegate.Compute(c)
	if err != nil {
		return TokenCount{}, fmt.Errorf("tokenizer: cached delegate compute failed: %w", err)
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Double-check: another goroutine may have filled the entry.
	if elem, ok := cc.cache[key]; ok {
		cc.lru.MoveToFront(elem)
		cc.totalHits++
		return elem.Value.(*lruEntry).count, nil
	}

	cc.totalMisses++

	// Evict LRU entry if at capacity.
	if cc.MaxEntries > 0 && cc.lru.Len() >= cc.MaxEntries {
		if back := cc.lru.Back(); back != nil {
			evicted := back.Value.(*lruEntry)
			delete(cc.cache, evicted.key)
			cc.lru.Remove(back)
			cc.totalEvictions++
		}
	}

	entry := &lruEntry{key: key, count: count}
	elem := cc.lru.PushFront(entry)
	cc.cache[key] = elem
	return count, nil
}

// Invalidate removes the cached entry for c. O(1).
func (cc *CachedTokenComputer) Invalidate(c MessageContent) {
	key := cacheKey(c)
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if elem, ok := cc.cache[key]; ok {
		cc.lru.Remove(elem)
		delete(cc.cache, key)
	}
}

// Flush clears all cached entries and resets aggregate stats counters.
func (cc *CachedTokenComputer) Flush() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.cache = make(map[string]*list.Element)
	cc.lru.Init()
	cc.totalHits = 0
	cc.totalMisses = 0
	cc.totalEvictions = 0
}

// CacheSize returns the number of currently cached entries.
func (cc *CachedTokenComputer) CacheSize() int {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return cc.lru.Len()
}

// Stats returns an immutable snapshot of cache performance metrics.
func (cc *CachedTokenComputer) Stats() Stats {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return Stats{
		Hits:       cc.totalHits,
		Misses:     cc.totalMisses,
		Evictions:  cc.totalEvictions,
		CacheSize:  cc.lru.Len(),
		MaxEntries: cc.MaxEntries,
	}
}

// ResetStats zeroes aggregate counters without clearing cached entries.
// Use this to measure performance over a fixed interval.
func (cc *CachedTokenComputer) ResetStats() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.totalHits = 0
	cc.totalMisses = 0
	cc.totalEvictions = 0
}

// =============================================================================
// Stats  (R18)
// =============================================================================

// Stats is an immutable snapshot of CachedTokenComputer performance metrics.
type Stats struct {
	// Hits is the number of Compute calls served from cache.
	Hits int64
	// Misses is the number of Compute calls that required a delegate call.
	Misses int64
	// Evictions is the number of LRU evictions due to capacity limits.
	Evictions int64
	// CacheSize is the current number of cached entries.
	CacheSize int
	// MaxEntries is the configured capacity limit (0 = unbounded).
	MaxEntries int
}

// Total returns the total number of Compute calls (Hits + Misses).
func (s Stats) Total() int64 { return s.Hits + s.Misses }

// HitRate returns the fraction of calls served from cache (0.0–1.0).
// Returns 0 when no calls have been made.
func (s Stats) HitRate() float64 {
	if total := s.Total(); total > 0 {
		return float64(s.Hits) / float64(total)
	}
	return 0
}

// HitRatePct returns the hit rate as a percentage (0–100).
func (s Stats) HitRatePct() float64 { return s.HitRate() * 100 }

// FillPct returns the cache fill percentage (0–100).
// Returns 0 when MaxEntries is 0 (unbounded).
func (s Stats) FillPct() float64 {
	if s.MaxEntries == 0 {
		return 0
	}
	return float64(s.CacheSize) / float64(s.MaxEntries) * 100
}

// =============================================================================
// CompositeTokenComputer
// =============================================================================

// WeightedComputer pairs a TokenComputer with a positive relative weight.
type WeightedComputer struct {
	Computer TokenComputer
	Weight   float64
}

// CompositeTokenComputer returns a weighted average across multiple TokenComputers.
// Useful for blending heuristics (e.g. 60% CL100K + 40% Anthropic).
type CompositeTokenComputer struct {
	computers []WeightedComputer
}

// NewComposite validates inputs and returns a CompositeTokenComputer.
func NewComposite(computers ...WeightedComputer) (*CompositeTokenComputer, error) {
	if len(computers) == 0 {
		return nil, fmt.Errorf("tokenizer: CompositeTokenComputer requires at least one computer")
	}
	for i, wc := range computers {
		if wc.Computer == nil {
			return nil, fmt.Errorf("tokenizer: computer at index %d is nil", i)
		}
		if wc.Weight <= 0 {
			return nil, fmt.Errorf("tokenizer: computer at index %d has non-positive weight %f", i, wc.Weight)
		}
	}
	return &CompositeTokenComputer{computers: computers}, nil
}

// Compute implements TokenComputer.
func (cc *CompositeTokenComputer) Compute(c MessageContent) (TokenCount, error) {
	var weightedSum, totalWeight float64
	for _, wc := range cc.computers {
		count, err := wc.Computer.Compute(c)
		if err != nil {
			return TokenCount{}, fmt.Errorf("tokenizer: composite sub-computer failed: %w", err)
		}
		weightedSum += float64(count.Value()) * wc.Weight
		totalWeight += wc.Weight
	}
	avg := int32(weightedSum/totalWeight + 0.5)
	if avg < 0 {
		avg = 0
	}
	return NewTokenCount(avg)
}

// =============================================================================
// ClampedTokenComputer
// =============================================================================

// ClampedTokenComputer enforces [Min, Max] bounds on any delegate.
// Min = 0 means no lower clamp; Max = 0 means no upper clamp.
type ClampedTokenComputer struct {
	delegate TokenComputer
	min      int32
	max      int32
}

// NewClamped validates bounds and constructs a ClampedTokenComputer.
func NewClamped(delegate TokenComputer, min, max int32) (*ClampedTokenComputer, error) {
	if delegate == nil {
		return nil, fmt.Errorf("tokenizer: ClampedTokenComputer delegate must not be nil")
	}
	if max > 0 && min > max {
		return nil, fmt.Errorf("tokenizer: ClampedTokenComputer min (%d) > max (%d)", min, max)
	}
	return &ClampedTokenComputer{delegate: delegate, min: min, max: max}, nil
}

// Compute implements TokenComputer.
func (ct *ClampedTokenComputer) Compute(c MessageContent) (TokenCount, error) {
	count, err := ct.delegate.Compute(c)
	if err != nil {
		return TokenCount{}, err
	}
	v := count.Value()
	if ct.min > 0 && v < ct.min {
		v = ct.min
	}
	if ct.max > 0 && v > ct.max {
		v = ct.max
	}
	return NewTokenCount(v)
}

// =============================================================================
// Factory helpers
// =============================================================================

// Default returns a CachedTokenComputer wrapping WordApproxTokenComputer
// with a 4096-entry LRU cache. Suitable when model-specific accuracy is
// not critical.
func Default() (*CachedTokenComputer, error) {
	return NewCached(WordApproxTokenComputer{}, 4096)
}

// ForOpenAI returns a CachedTokenComputer wrapping CL100KTokenComputer.
func ForOpenAI(cacheSize int) (*CachedTokenComputer, error) {
	return NewCached(CL100KTokenComputer{}, cacheSize)
}

// ForAnthropic returns a CachedTokenComputer wrapping AnthropicTokenComputer.
func ForAnthropic(cacheSize int) (*CachedTokenComputer, error) {
	return NewCached(AnthropicTokenComputer{}, cacheSize)
}

// ContentKey returns a stable lowercase-trimmed key for a MessageContent.
// Used by compactors and scorers that need a stable content identity.
func ContentKey(c MessageContent) string {
	return strings.ToLower(strings.TrimSpace(c.String()))
}
