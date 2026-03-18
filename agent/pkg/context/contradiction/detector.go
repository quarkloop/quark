package contradiction

import (
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// ContradictionWarning
// =============================================================================

// ContradictionSeverity indicates how confident the detector is.
type ContradictionSeverity string

const (
	// SeverityPossible means the messages are topically similar (high Jaccard)
	// but a definitive contradiction could not be confirmed heuristically.
	SeverityPossible ContradictionSeverity = "possible"

	// SeverityLikely means the messages mention the same entity with different
	// descriptors — a strong heuristic signal of contradiction.
	SeverityLikely ContradictionSeverity = "likely"
)

// ContradictionWarning describes a potential conflict between two messages.
// The detector never removes or modifies messages — it only warns.
type ContradictionWarning struct {
	// IncomingID is the ID of the message being appended.
	IncomingID string
	// ConflictingID is the ID of the existing message that may conflict.
	ConflictingID string
	// Similarity is the computed similarity score in [0, 1].
	Similarity float64
	// Severity is the confidence level of the warning.
	Severity ContradictionSeverity
	// Reason is a human-readable explanation.
	Reason string
	// DetectedAt is the wall-clock time of detection.
	DetectedAt time.Time
}

// String returns a compact log-friendly representation.
func (w ContradictionWarning) String() string {
	return fmt.Sprintf("[contradiction %s] incoming=%s conflicts=%s sim=%.2f: %s",
		w.Severity, w.IncomingID, w.ConflictingID, w.Similarity, w.Reason)
}

// =============================================================================
// ContradictionDetector interface
// =============================================================================

// ContradictionDetector checks an incoming message against the indexed history.
//
// Callers attach a Detector to the AgentContext as AppendMessage middleware.
// When a warning is produced the caller decides what to do: log it, surface
// it to the user, or remove the conflicting message.
type ContradictionDetector interface {
	// Check examines incomingText against all indexed messages.
	// Returns a (possibly empty) list of warnings sorted by descending similarity.
	Check(incomingID, incomingText string) ([]ContradictionWarning, error)

	// Index adds a message to the detector's internal index so future Check
	// calls can compare against it.  Must be called after Check to maintain
	// a consistent state.
	Index(id, text string) error

	// Remove removes a message from the index.  Called when a message is
	// evicted from the AgentContext so the index stays in sync.
	Remove(id string)

	// Clear resets the index.
	Clear()
}

// =============================================================================
// HeuristicDetector
// =============================================================================

// HeuristicDetector is the built-in ContradictionDetector.
// It uses SemanticFingerprint (Jaccard similarity by default; cosine when an
// EmbeddingFingerprintFunc is configured) to find topically similar messages,
// then applies entity-value heuristics to upgrade possible → likely.
//
// The detector is safe for concurrent use.
type HeuristicDetector struct {
	mu sync.RWMutex

	// index maps message ID → SemanticFingerprint
	index map[string]SemanticFingerprint

	// similarityThreshold: Jaccard/cosine score above which messages are
	// considered "about the same topic" and checked for contradiction.
	// Default: 0.3
	similarityThreshold float64

	// embedFn is an optional embedding function.
	embedFn EmbeddingFingerprintFunc
}

// NewHeuristicDetector returns a HeuristicDetector with default settings.
// Use With* methods to customise.
func NewHeuristicDetector() *HeuristicDetector {
	return &HeuristicDetector{
		index:               make(map[string]SemanticFingerprint),
		similarityThreshold: 0.3,
	}
}

// WithSimilarityThreshold sets the minimum similarity score required to flag
// two messages as topically similar.  Must be in (0, 1].
func (d *HeuristicDetector) WithSimilarityThreshold(t float64) *HeuristicDetector {
	if t <= 0 || t > 1 {
		panic(fmt.Sprintf("contradiction: similarity threshold must be in (0,1], got %.2f", t))
	}
	d.similarityThreshold = t
	return d
}

// WithEmbeddingFunc configures a dense-vector embedding function.
// When set, cosine similarity replaces Jaccard for the topic-similarity step.
func (d *HeuristicDetector) WithEmbeddingFunc(fn EmbeddingFingerprintFunc) *HeuristicDetector {
	d.embedFn = fn
	return d
}

// Check implements ContradictionDetector.
func (d *HeuristicDetector) Check(incomingID, incomingText string) ([]ContradictionWarning, error) {
	incoming, err := ComputeFingerprint(incomingID, incomingText, d.embedFn)
	if err != nil {
		return nil, fmt.Errorf("contradiction: fingerprint failed: %w", err)
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var warnings []ContradictionWarning
	for existingID, existing := range d.index {
		sim := incoming.Similarity(existing)
		if sim < d.similarityThreshold {
			continue
		}
		sev, reason := classifyConflict(incoming, existing)
		if sev == "" {
			// Topics are similar but no value conflict detected.
			sev = SeverityPossible
			reason = fmt.Sprintf("high topic similarity (%.2f)", sim)
		}
		warnings = append(warnings, ContradictionWarning{
			IncomingID:    incomingID,
			ConflictingID: existingID,
			Similarity:    sim,
			Severity:      sev,
			Reason:        reason,
			DetectedAt:    time.Now().UTC(),
		})
	}

	// Sort: highest similarity first.
	sortWarningsDesc(warnings)
	return warnings, nil
}

// Index implements ContradictionDetector.
func (d *HeuristicDetector) Index(id, text string) error {
	fp, err := ComputeFingerprint(id, text, d.embedFn)
	if err != nil {
		return fmt.Errorf("contradiction: index fingerprint failed: %w", err)
	}
	d.mu.Lock()
	d.index[id] = fp
	d.mu.Unlock()
	return nil
}

// Remove implements ContradictionDetector.
func (d *HeuristicDetector) Remove(id string) {
	d.mu.Lock()
	delete(d.index, id)
	d.mu.Unlock()
}

// Clear implements ContradictionDetector.
func (d *HeuristicDetector) Clear() {
	d.mu.Lock()
	d.index = make(map[string]SemanticFingerprint)
	d.mu.Unlock()
}

// IndexSize returns the number of messages currently indexed.
func (d *HeuristicDetector) IndexSize() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.index)
}

// =============================================================================
// classifyConflict — entity-value heuristic
// =============================================================================

// classifyConflict tries to determine whether two topically-similar messages
// contain a genuine value contradiction.
//
// Strategy: look for tokens in the incoming message that appear in the
// existing message's context but with a different "neighbour" — a rough
// proxy for "same entity, different value".
//
// This is intentionally conservative: false negatives are acceptable;
// false positives are noisy and erode trust in the detector.
func classifyConflict(incoming, existing SemanticFingerprint) (ContradictionSeverity, string) {
	// Tokens exclusive to each fingerprint — these are the "different values".
	onlyIncoming := exclusiveTokens(incoming.Tokens, existing.Tokens)
	onlyExisting := exclusiveTokens(existing.Tokens, incoming.Tokens)

	// Shared tokens — these are the "same entity".
	shared := sharedTokenCount(incoming.Tokens, existing.Tokens)

	// Heuristic: shared context + diverging descriptors → likely contradiction.
	if shared >= 2 && len(onlyIncoming) >= 1 && len(onlyExisting) >= 1 {
		return SeverityLikely, fmt.Sprintf(
			"shared context (%d tokens) with diverging values: %v vs %v",
			shared, onlyIncoming, onlyExisting,
		)
	}
	return "", ""
}

func exclusiveTokens(a, b map[string]struct{}) []string {
	var out []string
	for tok := range a {
		if _, inB := b[tok]; !inB {
			out = append(out, tok)
		}
	}
	return out
}

func sharedTokenCount(a, b map[string]struct{}) int {
	count := 0
	for tok := range a {
		if _, ok := b[tok]; ok {
			count++
		}
	}
	return count
}

func sortWarningsDesc(ws []ContradictionWarning) {
	for i := 1; i < len(ws); i++ {
		for j := i; j > 0 && ws[j].Similarity > ws[j-1].Similarity; j-- {
			ws[j], ws[j-1] = ws[j-1], ws[j]
		}
	}
}
