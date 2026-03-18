// Package freshness defines the FreshnessPolicy interface and all standard
// implementations for controlling how long a Message's data remains valid.
//
// # Concept
//
// A message's *content* may be permanently correct (a contract date, a name),
// but its *contextual truth value* can expire.  "The user is in Berlin" was
// true at 10am; at 2pm they flew to Munich.  A status message saying "the
// database is healthy" is stale after 5 minutes.
//
// FreshnessPolicy separates two concerns:
//   - staleness detection: IsStale(ctx) → bool
//   - optional self-healing: Refresh(ctx) → updated content string
//
// # Usage
//
// Attach a policy to a Message via message.go's WithFreshnessPolicy.
// Before building each LLM request, call AgentContext.RefreshStaleMessages
// (freshness/scanner.go) to scan every message and replace or flag stale ones.
//
// # Implementations
//
//	TTLFreshnessPolicy          — stale after a fixed duration
//	RequestCountFreshnessPolicy — stale after N requests
//	ImmutableFreshnessPolicy    — never stale (for contracts, resumes)
//	LocationSensitiveFreshnessPolicy — stale when location changes
//	ExternalFreshnessPolicy     — delegates to a caller-supplied function
//	CompositeFreshnessPolicy    — combines multiple policies (Any / All)
package freshness

import "time"

// =============================================================================
// ValidationContext
// =============================================================================

// ValidationContext carries all ambient state that FreshnessPolicy
// implementations may consult to determine staleness.
//
// Only the fields relevant to a given policy need to be populated; missing
// fields never cause a panic — policies treat zero/nil values as "unknown".
type ValidationContext struct {
	// Now is the current wall-clock time.  Required.
	Now time.Time

	// RequestCount is the number of LLM requests sent since the context was
	// created.  Used by RequestCountFreshnessPolicy.
	RequestCount int64

	// Location is an optional geolocation tag for the current session,
	// e.g. "Berlin, DE" or a lat/lng pair formatted as a string.
	// Used by LocationSensitiveFreshnessPolicy.
	Location string

	// SessionID is an opaque identifier for the current conversation session.
	// Policies can use it to detect session boundaries.
	SessionID string

	// Extra carries any application-specific key/value pairs that custom
	// FreshnessPolicy implementations may need.
	Extra map[string]string
}

// Age returns how long ago ts occurred relative to ctx.Now.
// Returns 0 when ts is after ctx.Now (clock skew guard).
func (ctx ValidationContext) Age(ts time.Time) time.Duration {
	if ctx.Now.Before(ts) {
		return 0
	}
	return ctx.Now.Sub(ts)
}

// =============================================================================
// FreshnessPolicy interface
// =============================================================================

// FreshnessPolicy controls how long a Message's data stays valid.
//
// Implementations must be safe for concurrent use from multiple goroutines.
//
// The optional Refresh method enables self-healing: when IsStale returns true,
// the scanner calls Refresh and replaces the message content with the returned
// string before the LLM call.  If Refresh is not implemented (returns "", nil)
// the message is marked stale but not updated.
type FreshnessPolicy interface {
	// IsStale reports whether the message data should be considered outdated.
	//
	// createdAt is the Timestamp when the message was first appended.
	// ctx carries ambient session state.
	IsStale(createdAt time.Time, ctx ValidationContext) bool

	// Refresh produces fresh content to replace the stale message body.
	// Return ("", nil) if this policy does not support self-healing.
	// The caller replaces only the payload's text content; it does not
	// create a new Message or change the ID.
	Refresh(ctx ValidationContext) (newContent string, err error)

	// Description returns a human-readable summary of the policy for logging.
	Description() string
}

// =============================================================================
// NeverStale sentinel
// =============================================================================

// NeverStale is a convenience value that can be compared against in tests or
// used as a sentinel.  Functionally identical to ImmutableFreshnessPolicy{}.
var NeverStale FreshnessPolicy = ImmutableFreshnessPolicy{}
