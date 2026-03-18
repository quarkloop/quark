package freshness

import (
	"fmt"
	"time"
)

// RequestCountFreshnessPolicy marks a message stale after a fixed number of
// LLM requests have been sent since the message was appended.
//
// This policy is useful for data that is meaningful for only N reasoning steps
// (e.g. an injected search result that should be refreshed every 3 turns).
//
// Example — stale after 5 requests:
//
//	policy := freshness.NewRequestCountPolicy(5)
type RequestCountFreshnessPolicy struct {
	// maxRequests is the number of requests after which the message is stale.
	maxRequests int64
	// createdAtRequest is the RequestCount when the message was appended.
	// Set by the scanner when it first processes the message.
	createdAtRequest int64
	// refreshFn is an optional self-healing function.
	refreshFn func(ctx ValidationContext) (string, error)
}

// NewRequestCountPolicy returns a RequestCountFreshnessPolicy.
// maxRequests must be > 0.
func NewRequestCountPolicy(maxRequests int64) *RequestCountFreshnessPolicy {
	if maxRequests <= 0 {
		panic(fmt.Sprintf("freshness: maxRequests must be > 0, got %d", maxRequests))
	}
	return &RequestCountFreshnessPolicy{maxRequests: maxRequests}
}

// WithRefresh adds a self-healing function to the policy.
func (p *RequestCountFreshnessPolicy) WithRefresh(fn func(ctx ValidationContext) (string, error)) *RequestCountFreshnessPolicy {
	p.refreshFn = fn
	return p
}

// WithCreatedAtRequest records the request count at the time the message was
// appended.  Called by the scanner the first time it sees this policy.
func (p *RequestCountFreshnessPolicy) WithCreatedAtRequest(n int64) *RequestCountFreshnessPolicy {
	p.createdAtRequest = n
	return p
}

// IsStale returns true when the current RequestCount exceeds the threshold.
// createdAt is ignored; this policy is purely request-count-based.
func (p *RequestCountFreshnessPolicy) IsStale(_ time.Time, ctx ValidationContext) bool {
	return ctx.RequestCount-p.createdAtRequest > p.maxRequests
}

// Refresh calls the registered refresh function, or returns ("", nil).
func (p *RequestCountFreshnessPolicy) Refresh(ctx ValidationContext) (string, error) {
	if p.refreshFn == nil {
		return "", nil
	}
	return p.refreshFn(ctx)
}

// Description returns a human-readable summary.
func (p *RequestCountFreshnessPolicy) Description() string {
	return fmt.Sprintf("RequestCount(max=%d)", p.maxRequests)
}

// MaxRequests returns the configured request limit.
func (p *RequestCountFreshnessPolicy) MaxRequests() int64 { return p.maxRequests }
