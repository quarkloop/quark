package llmctx

// =============================================================================
// tokenizer.go  —  Root-package re-exports of llmctx/tokenizer
//
// The canonical implementations live in llmctx/tokenizer.
// These aliases let callers who import only "llmctx" use tokenizer types
// without a separate import.
// =============================================================================

import "github.com/quarkloop/agent/pkg/context/tokenizer"

// Concrete TokenComputer implementations — aliases of llmctx/tokenizer types.
// Use these directly or wrap them with NewCachedTokenComputer.
type (
	WordApproxTokenComputer  = tokenizer.WordApproxTokenComputer
	CharApproxTokenComputer  = tokenizer.CharApproxTokenComputer
	CL100KTokenComputer      = tokenizer.CL100KTokenComputer
	AnthropicTokenComputer   = tokenizer.AnthropicTokenComputer
	CachedTokenComputer      = tokenizer.CachedTokenComputer
	CompositeTokenComputer   = tokenizer.CompositeTokenComputer
	ClampedTokenComputer     = tokenizer.ClampedTokenComputer
	WeightedComputer         = tokenizer.WeightedComputer
)

// NewCachedTokenComputer wraps delegate with an LRU memoisation layer.
// maxEntries = 0 means unbounded.
func NewCachedTokenComputer(delegate TokenComputer, maxEntries int) (*CachedTokenComputer, error) {
	return tokenizer.NewCached(delegate, maxEntries)
}

// NewCompositeTokenComputer validates inputs and returns a CompositeTokenComputer.
func NewCompositeTokenComputer(computers ...WeightedComputer) (*CompositeTokenComputer, error) {
	return tokenizer.NewComposite(computers...)
}

// NewClampedTokenComputer validates bounds and constructs a ClampedTokenComputer.
func NewClampedTokenComputer(delegate TokenComputer, min, max int32) (*ClampedTokenComputer, error) {
	return tokenizer.NewClamped(delegate, min, max)
}

// DefaultTokenComputer returns a CachedTokenComputer wrapping WordApproxTokenComputer
// with a 4096-entry LRU cache.
func DefaultTokenComputer() (*CachedTokenComputer, error) {
	return tokenizer.Default()
}

// OpenAITokenComputer returns a CachedTokenComputer wrapping CL100KTokenComputer.
func OpenAITokenComputer(cacheSize int) (*CachedTokenComputer, error) {
	return tokenizer.ForOpenAI(cacheSize)
}

// AnthropicCachedTokenComputer returns a CachedTokenComputer wrapping AnthropicTokenComputer.
func AnthropicCachedTokenComputer(cacheSize int) (*CachedTokenComputer, error) {
	return tokenizer.ForAnthropic(cacheSize)
}

// ContentKey returns a stable lowercase-trimmed key for a MessageContent.
func ContentKey(c MessageContent) string {
	return tokenizer.ContentKey(c)
}
