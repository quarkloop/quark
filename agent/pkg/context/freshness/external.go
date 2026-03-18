package freshness

import (
	"fmt"
	"time"
)

// ExternalFreshnessPolicy delegates staleness detection to a caller-supplied
// function.  This is the escape hatch for policies that require domain logic
// not covered by the built-in implementations.
//
// Example — stale when a feature flag is disabled:
//
//	policy := freshness.NewExternalPolicy(
//	    func(createdAt time.Time, ctx freshness.ValidationContext) bool {
//	        return !featureFlags.IsEnabled("weather-widget")
//	    },
//	    "feature-flag:weather-widget",
//	)
type ExternalFreshnessPolicy struct {
	staleFn     func(createdAt time.Time, ctx ValidationContext) bool
	refreshFn   func(ctx ValidationContext) (string, error)
	description string
}

// NewExternalPolicy returns a policy backed by staleFn.
//
//	staleFn      — required; returns true when the message is stale
//	description  — human-readable label shown in logs and stats
func NewExternalPolicy(
	staleFn func(createdAt time.Time, ctx ValidationContext) bool,
	description string,
) *ExternalFreshnessPolicy {
	if staleFn == nil {
		panic("freshness: ExternalFreshnessPolicy staleFn must not be nil")
	}
	return &ExternalFreshnessPolicy{
		staleFn:     staleFn,
		description: description,
	}
}

// WithRefresh adds a self-healing function.
func (p *ExternalFreshnessPolicy) WithRefresh(fn func(ctx ValidationContext) (string, error)) *ExternalFreshnessPolicy {
	p.refreshFn = fn
	return p
}

// IsStale delegates to the caller-supplied function.
func (p *ExternalFreshnessPolicy) IsStale(createdAt time.Time, ctx ValidationContext) bool {
	return p.staleFn(createdAt, ctx)
}

// Refresh calls the registered function, or returns ("", nil).
func (p *ExternalFreshnessPolicy) Refresh(ctx ValidationContext) (string, error) {
	if p.refreshFn == nil {
		return "", nil
	}
	return p.refreshFn(ctx)
}

// Description returns the label supplied at construction.
func (p *ExternalFreshnessPolicy) Description() string {
	return fmt.Sprintf("External(%s)", p.description)
}
