// Package semantic provides opt-in middleware for semantic compression of
// messages on write, before compaction is needed.
//
// # Concept
//
// Traditional compaction is reactive: when the context overflows, messages are
// evicted.  Semantic compression is proactive: when a new message arrives that
// is semantically similar to an existing one, the two are merged immediately,
// keeping the context dense with meaning rather than dense with tokens.
//
// # Detection
//
// The SemanticIndex maintains a SemanticFingerprint for every indexed message.
// On each AppendMessage call, the middleware:
//
//  1. Computes a fingerprint for the incoming message.
//  2. Queries the index for messages above a similarity threshold.
//  3. If a match is found, emits a MergeCandidate.
//  4. The caller (or auto-merge logic) decides whether to merge.
//
// # Auto-merge
//
// When AutoMergeThreshold > 0, matches above that threshold are merged
// automatically.  The merged message replaces the older one; the incoming
// message is not appended.
//
// This is a strong transformation and should be used with a high threshold
// (≥ 0.85) and only for idempotent payload types (TextPayload, MemoryPayload).
//
// # Pluggability
//
// Supply an EmbeddingFunc to upgrade from Jaccard similarity to cosine
// similarity on real embedding vectors.
package semantic

import (
	"fmt"
	"sync"
	"time"

	"github.com/quarkloop/agent/pkg/context/contradiction" // shares the fingerprint infrastructure
)

// =============================================================================
// MergeCandidate
// =============================================================================

// MergeCandidate describes a pair of messages the compressor considers
// equivalent enough to merge.
type MergeCandidate struct {
	// IncomingID is the ID of the message being appended.
	IncomingID string
	// IncomingText is the content of the incoming message.
	IncomingText string
	// ExistingID is the ID of the matching message already in the context.
	ExistingID string
	// ExistingText is the content of the existing message.
	ExistingText string
	// Similarity is the computed score in [0, 1].
	Similarity float64
	// DetectedAt is the wall-clock time of the check.
	DetectedAt time.Time
}

// String returns a compact log-friendly representation.
func (mc MergeCandidate) String() string {
	return fmt.Sprintf("[merge-candidate] incoming=%s existing=%s sim=%.3f",
		mc.IncomingID, mc.ExistingID, mc.Similarity)
}

// =============================================================================
// MergeFunc
// =============================================================================

// MergeFunc is called when two messages are candidates for merging.
// It receives the texts of both messages and should return a merged text.
//
// The simplest implementation appends the new text to the old:
//
//	func(existing, incoming string) string { return existing + "\n" + incoming }
//
// A more powerful implementation calls an LLM to summarise both.
type MergeFunc func(existingText, incomingText string) (mergedText string, err error)

// =============================================================================
// SemanticIndex
// =============================================================================

// SemanticIndex maintains a fingerprint map for rapid similarity queries.
// It is the backing store for the SemanticCompressor.
type SemanticIndex struct {
	mu          sync.RWMutex
	fingerprints map[string]indexedEntry
	embedFn     contradiction.EmbeddingFingerprintFunc
}

type indexedEntry struct {
	id          string
	text        string
	fingerprint contradiction.SemanticFingerprint
}

// NewSemanticIndex returns an empty SemanticIndex.
// embedFn may be nil; Jaccard similarity is used as fallback.
func NewSemanticIndex(embedFn contradiction.EmbeddingFingerprintFunc) *SemanticIndex {
	return &SemanticIndex{
		fingerprints: make(map[string]indexedEntry),
		embedFn:     embedFn,
	}
}

// Add indexes a message.  Replaces any existing entry for the same ID.
func (idx *SemanticIndex) Add(id, text string) error {
	fp, err := contradiction.ComputeFingerprint(id, text, idx.embedFn)
	if err != nil {
		return fmt.Errorf("semantic: fingerprint failed for %q: %w", id, err)
	}
	idx.mu.Lock()
	idx.fingerprints[id] = indexedEntry{id: id, text: text, fingerprint: fp}
	idx.mu.Unlock()
	return nil
}

// Remove removes a message from the index.
func (idx *SemanticIndex) Remove(id string) {
	idx.mu.Lock()
	delete(idx.fingerprints, id)
	idx.mu.Unlock()
}

// Clear empties the index.
func (idx *SemanticIndex) Clear() {
	idx.mu.Lock()
	idx.fingerprints = make(map[string]indexedEntry)
	idx.mu.Unlock()
}

// Size returns the number of indexed messages.
func (idx *SemanticIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.fingerprints)
}

// QuerySimilar returns all indexed messages with similarity ≥ threshold,
// sorted by descending similarity.  The candidate's own ID is excluded.
func (idx *SemanticIndex) QuerySimilar(id, text string, threshold float64) ([]SimilarMatch, error) {
	incoming, err := contradiction.ComputeFingerprint(id, text, idx.embedFn)
	if err != nil {
		return nil, fmt.Errorf("semantic: query fingerprint failed: %w", err)
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var matches []SimilarMatch
	for existingID, entry := range idx.fingerprints {
		if existingID == id {
			continue
		}
		sim := incoming.Similarity(entry.fingerprint)
		if sim >= threshold {
			matches = append(matches, SimilarMatch{
				ID:         existingID,
				Text:       entry.text,
				Similarity: sim,
			})
		}
	}
	sortMatchesDesc(matches)
	return matches, nil
}

// SimilarMatch is one result from a similarity query.
type SimilarMatch struct {
	ID         string
	Text       string
	Similarity float64
}

func sortMatchesDesc(ms []SimilarMatch) {
	for i := 1; i < len(ms); i++ {
		for j := i; j > 0 && ms[j].Similarity > ms[j-1].Similarity; j-- {
			ms[j], ms[j-1] = ms[j-1], ms[j]
		}
	}
}

// =============================================================================
// SemanticCompressor
// =============================================================================

// SemanticCompressor is write-time middleware that detects and optionally
// merges semantically similar messages as they are appended.
//
// It wraps a SemanticIndex and exposes the Check / AutoMerge pipeline.
type SemanticCompressor struct {
	index *SemanticIndex

	// candidateThreshold: minimum similarity to emit a MergeCandidate.
	candidateThreshold float64

	// autoMergeThreshold: when > 0 and ≥ candidateThreshold, messages above
	// this score are merged automatically using mergeFn.
	autoMergeThreshold float64

	// mergeFn is called when two messages are auto-merged.  Required when
	// autoMergeThreshold > 0.
	mergeFn MergeFunc
}

// NewSemanticCompressor returns a SemanticCompressor.
//
//	index               — the SemanticIndex to query (required)
//	candidateThreshold  — similarity threshold for emitting MergeCandidates (0 < t ≤ 1)
func NewSemanticCompressor(index *SemanticIndex, candidateThreshold float64) (*SemanticCompressor, error) {
	if index == nil {
		return nil, fmt.Errorf("semantic: index must not be nil")
	}
	if candidateThreshold <= 0 || candidateThreshold > 1 {
		return nil, fmt.Errorf("semantic: candidateThreshold must be in (0,1], got %.2f", candidateThreshold)
	}
	return &SemanticCompressor{
		index:              index,
		candidateThreshold: candidateThreshold,
	}, nil
}

// WithAutoMerge enables automatic merging above autoMergeThreshold using fn.
// autoMergeThreshold must be ≥ candidateThreshold.
func (sc *SemanticCompressor) WithAutoMerge(autoMergeThreshold float64, fn MergeFunc) *SemanticCompressor {
	if fn == nil {
		panic("semantic: MergeFunc must not be nil")
	}
	sc.autoMergeThreshold = autoMergeThreshold
	sc.mergeFn = fn
	return sc
}

// CheckResult is the output of a single Check call.
type CheckResult struct {
	// Candidates lists messages that are similar enough to merge.
	// Empty when no matches exceed candidateThreshold.
	Candidates []MergeCandidate

	// AutoMerged is set when autoMergeThreshold is configured and a match
	// exceeded it.  Contains the merged text.
	AutoMerged *AutoMergeResult
}

// AutoMergeResult is returned when auto-merge fires.
type AutoMergeResult struct {
	// ReplacedID is the ID of the existing message that was replaced.
	ReplacedID string
	// MergedText is the content produced by MergeFunc.
	MergedText string
}

// Check examines an incoming message for similarity to indexed messages.
//
// Returns a CheckResult.  When AutoMerged is non-nil the caller should:
//  1. NOT append the incoming message.
//  2. Update the payload of AutoMerged.ReplacedID to AutoMerged.MergedText.
//
// When Candidates is non-empty but AutoMerged is nil, the caller may inspect
// the candidates and decide whether to merge, log, or ignore.
func (sc *SemanticCompressor) Check(id, text string) (CheckResult, error) {
	matches, err := sc.index.QuerySimilar(id, text, sc.candidateThreshold)
	if err != nil {
		return CheckResult{}, err
	}
	if len(matches) == 0 {
		return CheckResult{}, nil
	}

	candidates := make([]MergeCandidate, 0, len(matches))
	for _, m := range matches {
		candidates = append(candidates, MergeCandidate{
			IncomingID:   id,
			IncomingText: text,
			ExistingID:   m.ID,
			ExistingText: m.Text,
			Similarity:   m.Similarity,
			DetectedAt:   time.Now().UTC(),
		})
	}

	// Auto-merge: use the highest-similarity match.
	if sc.autoMergeThreshold > 0 && matches[0].Similarity >= sc.autoMergeThreshold {
		best := matches[0]
		merged, mergeErr := sc.mergeFn(best.Text, text)
		if mergeErr != nil {
			return CheckResult{Candidates: candidates},
				fmt.Errorf("semantic: merge failed: %w", mergeErr)
		}
		return CheckResult{
			Candidates: candidates,
			AutoMerged: &AutoMergeResult{
				ReplacedID: best.ID,
				MergedText: merged,
			},
		}, nil
	}

	return CheckResult{Candidates: candidates}, nil
}

// Index adds a message to the underlying SemanticIndex after it has been
// appended to the context.  Must be called after Check.
func (sc *SemanticCompressor) Index(id, text string) error {
	return sc.index.Add(id, text)
}

// Remove removes a message from the index.  Call when a message is evicted.
func (sc *SemanticCompressor) Remove(id string) {
	sc.index.Remove(id)
}

// IndexSize returns the number of indexed messages.
func (sc *SemanticCompressor) IndexSize() int { return sc.index.Size() }
