package freshness

import (
	"fmt"
	"time"
)

// TTLFreshnessPolicy marks a message stale when it is older than a fixed duration.
//
// Example — stale after 5 minutes:
//
//	policy := freshness.NewTTLPolicy(5 * time.Minute)
type TTLFreshnessPolicy struct {
	ttl time.Duration
	// refreshFn is an optional self-healing function.
	// When non-nil, Refresh calls it to produce updated content.
	refreshFn func(ctx ValidationContext) (string, error)
}

// NewTTLPolicy returns a TTLFreshnessPolicy with the given TTL.
// Panics if ttl ≤ 0.
func NewTTLPolicy(ttl time.Duration) *TTLFreshnessPolicy {
	if ttl <= 0 {
		panic(fmt.Sprintf("freshness: TTL must be > 0, got %v", ttl))
	}
	return &TTLFreshnessPolicy{ttl: ttl}
}

// WithRefresh adds a self-healing function to the policy.
// The function is called when IsStale returns true; its return value replaces
// the stale message content.
func (p *TTLFreshnessPolicy) WithRefresh(fn func(ctx ValidationContext) (string, error)) *TTLFreshnessPolicy {
	p.refreshFn = fn
	return p
}

// IsStale returns true when the message has been in the context longer than TTL.
func (p *TTLFreshnessPolicy) IsStale(createdAt time.Time, ctx ValidationContext) bool {
	return ctx.Age(createdAt) > p.ttl
}

// Refresh calls the registered refresh function, or returns ("", nil).
func (p *TTLFreshnessPolicy) Refresh(ctx ValidationContext) (string, error) {
	if p.refreshFn == nil {
		return "", nil
	}
	return p.refreshFn(ctx)
}

// Description returns a human-readable summary.
func (p *TTLFreshnessPolicy) Description() string {
	return fmt.Sprintf("TTL(%v)", p.ttl)
}

// TTL returns the configured duration.
func (p *TTLFreshnessPolicy) TTL() time.Duration { return p.ttl }
