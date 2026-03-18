package freshness

import (
	"fmt"
	"time"
)

// LocationSensitiveFreshnessPolicy marks a message stale when the session
// location changes from the one recorded at append time.
//
// Useful for location-contextual messages like "nearby restaurants", "local
// weather", or "timezone-aware reminders".
//
// Example:
//
//	policy := freshness.NewLocationPolicy("Berlin, DE")
type LocationSensitiveFreshnessPolicy struct {
	// originalLocation is the location at the time the message was appended.
	originalLocation string
	// refreshFn is called when the location has changed, to produce updated content.
	refreshFn func(newLocation string, ctx ValidationContext) (string, error)
}

// NewLocationPolicy returns a policy anchored to originalLocation.
func NewLocationPolicy(originalLocation string) *LocationSensitiveFreshnessPolicy {
	return &LocationSensitiveFreshnessPolicy{originalLocation: originalLocation}
}

// WithRefresh adds a self-healing function.  It receives the new location and
// the full ValidationContext.
func (p *LocationSensitiveFreshnessPolicy) WithRefresh(
	fn func(newLocation string, ctx ValidationContext) (string, error),
) *LocationSensitiveFreshnessPolicy {
	p.refreshFn = fn
	return p
}

// IsStale returns true when the current location differs from the original.
// If ctx.Location is empty the policy returns false (unknown = not stale).
func (p *LocationSensitiveFreshnessPolicy) IsStale(_ time.Time, ctx ValidationContext) bool {
	if ctx.Location == "" || p.originalLocation == "" {
		return false
	}
	return ctx.Location != p.originalLocation
}

// Refresh calls the registered function with the new location, or returns ("", nil).
func (p *LocationSensitiveFreshnessPolicy) Refresh(ctx ValidationContext) (string, error) {
	if p.refreshFn == nil {
		return "", nil
	}
	return p.refreshFn(ctx.Location, ctx)
}

// Description returns a human-readable summary.
func (p *LocationSensitiveFreshnessPolicy) Description() string {
	return fmt.Sprintf("Location(original=%q)", p.originalLocation)
}

// OriginalLocation returns the location at which the message was created.
func (p *LocationSensitiveFreshnessPolicy) OriginalLocation() string {
	return p.originalLocation
}
