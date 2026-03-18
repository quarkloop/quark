package llmctx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
)

// =============================================================================
// idgen.go  —  R12: MessageID generation strategies
//
// Problem solved:
//   Every New*Message factory requires a caller-supplied MessageID. Before R12
//   the only practical pattern was a package-level integer counter — a global
//   mutable variable that is not goroutine-safe and not unique across processes.
//
// Solution:
//   IDGenerator is a single-method interface. Three implementations cover the
//   full spectrum of use cases:
//
//     SequentialIDGenerator  – deterministic counter; ideal for tests
//     UUIDIDGenerator        – cryptographically random; ideal for production
//     PrefixedIDGenerator    – wraps any generator and prepends a fixed prefix
//
//   AgentContextBuilder gains WithIDGenerator so the strategy is injected once
//   and reused everywhere a new Message needs an ID.
//
// Design notes:
//   - Uses sync/atomic (not sync.Mutex) for the sequential counter so a single
//     generator is safely shared across goroutines with zero contention.
//   - UUIDIDGenerator uses crypto/rand (not math/rand) for unpredictability.
//     The format is plain hex (32 chars) rather than hyphenated UUID v4 to
//     avoid the "encoding/uuid" import — stdlib only.
//   - PrefixedIDGenerator is composable: wrap UUID for "msg-<uuid>", or wrap
//     Sequential for "usr-0001" in tests.
// =============================================================================

// IDGenerator produces unique, valid MessageIDs on demand.
// Implementations must be safe for concurrent use from multiple goroutines.
type IDGenerator interface {
	// Next returns the next MessageID.
	// Returns an error only when the underlying entropy source fails
	// (relevant only for UUIDIDGenerator; sequential generators never err).
	Next() (MessageID, error)
}

// MustNext calls g.Next() and panics if it returns an error.
// Useful in test setup or in init() paths where error handling is noise.
func MustNext(g IDGenerator) MessageID {
	id, err := g.Next()
	if err != nil {
		panic(fmt.Sprintf("IDGenerator.Next failed: %v", err))
	}
	return id
}

// =============================================================================
// SequentialIDGenerator
// =============================================================================

// SequentialIDGenerator produces IDs of the form "prefix-NNNN" where NNNN is
// a zero-padded decimal counter that increments atomically on every call.
//
// The counter starts at 1 and wraps at MaxInt64 (effectively never in practice).
// Deterministic output makes this ideal for tests and reproducible scenarios.
//
// Example output: "msg-0001", "msg-0002", …
type SequentialIDGenerator struct {
	prefix  string
	counter atomic.Int64
	width   int // zero-pad width for the numeric suffix
}

// NewSequentialIDGenerator returns a SequentialIDGenerator starting at 1.
//
//	prefix   – prepended to every ID; may be empty
//	width    – minimum digit width for zero-padding (0 → no padding)
//
// Example:
//
//	g := NewSequentialIDGenerator("msg", 4)
//	g.Next() // → "msg-0001"
//	g.Next() // → "msg-0002"
func NewSequentialIDGenerator(prefix string, width int) *SequentialIDGenerator {
	if width < 1 {
		width = 1
	}
	g := &SequentialIDGenerator{prefix: prefix, width: width}
	g.counter.Store(0)
	return g
}

// Next returns the next sequential ID. Never returns an error.
func (g *SequentialIDGenerator) Next() (MessageID, error) {
	n := g.counter.Add(1)
	var raw string
	if g.prefix == "" {
		raw = fmt.Sprintf("%0*d", g.width, n)
	} else {
		raw = fmt.Sprintf("%s-%0*d", g.prefix, g.width, n)
	}
	return NewMessageID(raw)
}

// Reset sets the counter back to zero. Intended for test teardown only;
// do not call during concurrent use without external synchronisation.
func (g *SequentialIDGenerator) Reset() { g.counter.Store(0) }

// Counter returns the current counter value (the number of IDs issued so far).
func (g *SequentialIDGenerator) Counter() int64 { return g.counter.Load() }

// =============================================================================
// UUIDIDGenerator
// =============================================================================

// UUIDIDGenerator produces IDs backed by 16 bytes of cryptographic randomness
// encoded as 32 lowercase hex characters.
//
// While not strictly RFC 4122 UUID v4, the 128-bit random space gives a
// collision probability so low it is negligible in practice.
// No external packages are required; only stdlib crypto/rand is used.
//
// Example output: "a3f1c2e4b8d07f9e1a2c4b6d8f0e3a5b"
type UUIDIDGenerator struct{}

// NewUUIDIDGenerator returns a UUIDIDGenerator.
// No configuration is required; the generator is stateless.
func NewUUIDIDGenerator() *UUIDIDGenerator {
	return &UUIDIDGenerator{}
}

// Next generates a new random ID using crypto/rand.
// Returns an error only if the OS entropy source fails, which is exceedingly rare.
func (g *UUIDIDGenerator) Next() (MessageID, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return MessageID{}, newErr(ErrCodeInvalidMessage,
			"UUIDIDGenerator: failed to read random bytes", err)
	}
	return NewMessageID(hex.EncodeToString(buf[:]))
}

// =============================================================================
// PrefixedIDGenerator
// =============================================================================

// PrefixedIDGenerator wraps any IDGenerator and prepends a fixed string and
// separator to every generated ID.
//
// This is composable: combine with UUIDIDGenerator for "usr-<uuid>", or with
// SequentialIDGenerator for "sys-0001" in tests.
//
// Example:
//
//	base := NewSequentialIDGenerator("", 4)
//	g    := NewPrefixedIDGenerator("turn", "-", base)
//	g.Next() // → "turn-0001"
//	g.Next() // → "turn-0002"
type PrefixedIDGenerator struct {
	prefix    string
	separator string
	inner     IDGenerator
}

// NewPrefixedIDGenerator validates its arguments and wraps inner with a prefix.
//
//	prefix    – the string prepended to every ID
//	separator – inserted between prefix and inner ID (typically "-" or "_")
//	inner     – the underlying generator; must not be nil
func NewPrefixedIDGenerator(prefix, separator string, inner IDGenerator) (*PrefixedIDGenerator, error) {
	if inner == nil {
		return nil, newErr(ErrCodeInvalidConfig,
			"PrefixedIDGenerator: inner generator must not be nil", nil)
	}
	return &PrefixedIDGenerator{
		prefix:    prefix,
		separator: separator,
		inner:     inner,
	}, nil
}

// Next delegates to the inner generator and prepends the configured prefix.
func (g *PrefixedIDGenerator) Next() (MessageID, error) {
	base, err := g.inner.Next()
	if err != nil {
		return MessageID{}, err
	}
	return NewMessageID(g.prefix + g.separator + base.String())
}

// =============================================================================
// Package-level default generator
// =============================================================================

// DefaultIDGenerator returns a UUIDIDGenerator suitable for production use.
// Use NewSequentialIDGenerator for deterministic test IDs.
func DefaultIDGenerator() IDGenerator {
	return NewUUIDIDGenerator()
}
