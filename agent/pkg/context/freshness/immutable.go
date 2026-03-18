package freshness

import "time"

// ImmutableFreshnessPolicy marks a message as permanently valid.
//
// Use this for content that is true by definition and must never be treated
// as stale: contract dates, names in a resume, constitutional facts, etc.
//
// Example:
//
//	policy := freshness.ImmutableFreshnessPolicy{}
type ImmutableFreshnessPolicy struct{}

// IsStale always returns false.
func (ImmutableFreshnessPolicy) IsStale(_ time.Time, _ ValidationContext) bool { return false }

// Refresh returns ("", nil) — immutable messages are never updated.
func (ImmutableFreshnessPolicy) Refresh(_ ValidationContext) (string, error) { return "", nil }

// Description returns a human-readable summary.
func (ImmutableFreshnessPolicy) Description() string { return "Immutable" }
