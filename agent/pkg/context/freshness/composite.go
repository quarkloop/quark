package freshness

import (
	"fmt"
	"strings"
	"time"
)

// CompositeMode controls how multiple policies are combined.
type CompositeMode int

const (
	// CompositeAny marks the message stale when ANY policy says IsStale.
	// This is the "fail-fast" mode: the strictest policy wins.
	CompositeAny CompositeMode = iota

	// CompositeAll marks the message stale only when ALL policies agree.
	// This is the "consensus" mode: useful for belt-and-suspenders scenarios.
	CompositeAll
)

// CompositeFreshnessPolicy combines multiple FreshnessPolicy implementations
// using either Any or All semantics.
//
// Refresh: the first policy whose Refresh returns a non-empty string is used.
//
// Example — stale when TTL expires OR location changes:
//
//	policy := freshness.NewCompositePolicy(freshness.CompositeAny,
//	    freshness.NewTTLPolicy(10 * time.Minute),
//	    freshness.NewLocationPolicy("Berlin, DE"),
//	)
type CompositeFreshnessPolicy struct {
	mode     CompositeMode
	policies []FreshnessPolicy
}

// NewCompositePolicy validates and returns a CompositeFreshnessPolicy.
// Panics if policies is empty or any element is nil.
func NewCompositePolicy(mode CompositeMode, policies ...FreshnessPolicy) *CompositeFreshnessPolicy {
	if len(policies) == 0 {
		panic("freshness: CompositeFreshnessPolicy requires at least one policy")
	}
	for i, p := range policies {
		if p == nil {
			panic(fmt.Sprintf("freshness: CompositeFreshnessPolicy: policy at index %d is nil", i))
		}
	}
	return &CompositeFreshnessPolicy{mode: mode, policies: policies}
}

// IsStale evaluates all child policies according to the configured mode.
func (p *CompositeFreshnessPolicy) IsStale(createdAt time.Time, ctx ValidationContext) bool {
	switch p.mode {
	case CompositeAll:
		for _, pol := range p.policies {
			if !pol.IsStale(createdAt, ctx) {
				return false
			}
		}
		return true
	default: // CompositeAny
		for _, pol := range p.policies {
			if pol.IsStale(createdAt, ctx) {
				return true
			}
		}
		return false
	}
}

// Refresh tries each policy in order and returns the first non-empty result.
func (p *CompositeFreshnessPolicy) Refresh(ctx ValidationContext) (string, error) {
	for _, pol := range p.policies {
		content, err := pol.Refresh(ctx)
		if err != nil {
			return "", err
		}
		if content != "" {
			return content, nil
		}
	}
	return "", nil
}

// Description returns a summary of the composite policy.
func (p *CompositeFreshnessPolicy) Description() string {
	mode := "Any"
	if p.mode == CompositeAll {
		mode = "All"
	}
	descs := make([]string, 0, len(p.policies))
	for _, pol := range p.policies {
		descs = append(descs, pol.Description())
	}
	return fmt.Sprintf("Composite%s(%s)", mode, strings.Join(descs, ", "))
}
