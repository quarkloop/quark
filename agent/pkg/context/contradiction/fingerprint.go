// Package contradiction provides middleware for detecting contradictory messages
// in an AgentContext before they reach the LLM.
//
// # Concept
//
// Context accumulates contradictions silently.  The user says "I'm in Berlin"
// in turn 3.  In turn 15 they say "I'm in Munich."  Both MemoryMessages sit in
// context and the model gets confused.
//
// The ContradictionDetector runs as opt-in middleware on AppendMessage.  It
// computes a lightweight SemanticFingerprint for the incoming message and
// compares it against the fingerprints of existing messages.  When a potential
// contradiction is found it returns a ContradictionWarning — it never silently
// modifies or removes messages.
//
// # Detection strategy (heuristic)
//
// Full semantic understanding requires embeddings or an LLM call.  The
// heuristic tier covers a surprising fraction of real cases without any
// external calls:
//
//  1. Normalise the message text (lowercase, strip punctuation).
//  2. Extract a bag of significant tokens (skip stopwords).
//  3. Compute Jaccard similarity between token sets.
//  4. If similarity ≥ highSimilarityThreshold, the messages are about the
//     same topic — check for value contradiction.
//  5. "Value contradiction" is detected when:
//     - Two messages mention the same named entity (noun phrase)
//     - Paired with different descriptors (e.g. "Berlin" vs "Munich")
//
// # Pluggability
//
// EmbeddingFingerprintFunc is the hook for replacing the heuristic with a
// real embedding model.  Set it on the Detector and the similarity step
// uses cosine distance on the embedding vectors instead of Jaccard.
package contradiction

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// =============================================================================
// SemanticFingerprint
// =============================================================================

// SemanticFingerprint is a compact representation of a message's semantic
// content used for rapid similarity comparison.
//
// The heuristic fingerprint is a normalised set of content tokens.
// When an EmbeddingFingerprintFunc is configured, the Embedding field is
// populated and used instead of Tokens for similarity.
type SemanticFingerprint struct {
	// MessageID is the ID of the source message.
	MessageID string

	// Tokens is the normalised set of significant content tokens.
	// Used for Jaccard-similarity comparison.
	Tokens map[string]struct{}

	// Embedding is an optional dense vector representation.
	// When non-nil, cosine similarity is used instead of Jaccard.
	Embedding []float64
}

// JaccardSimilarity returns the Jaccard similarity between f and other.
// Result is in [0.0, 1.0]; 1.0 means identical token sets.
func (f SemanticFingerprint) JaccardSimilarity(other SemanticFingerprint) float64 {
	if len(f.Tokens) == 0 || len(other.Tokens) == 0 {
		return 0
	}
	intersection := 0
	for tok := range f.Tokens {
		if _, ok := other.Tokens[tok]; ok {
			intersection++
		}
	}
	union := len(f.Tokens) + len(other.Tokens) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// CosineSimilarity returns the cosine similarity between the embedding vectors.
// Returns 0 when either embedding is nil or empty.
func (f SemanticFingerprint) CosineSimilarity(other SemanticFingerprint) float64 {
	a, b := f.Embedding, other.Embedding
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// Similarity returns the best available similarity measure.
// Uses cosine similarity when both fingerprints have embeddings; Jaccard otherwise.
func (f SemanticFingerprint) Similarity(other SemanticFingerprint) float64 {
	if len(f.Embedding) > 0 && len(other.Embedding) > 0 {
		return f.CosineSimilarity(other)
	}
	return f.JaccardSimilarity(other)
}

// =============================================================================
// Fingerprint computation
// =============================================================================

// EmbeddingFingerprintFunc is an optional hook that computes a dense vector
// representation for text.  When non-nil, it is called during fingerprinting
// and the result is stored in SemanticFingerprint.Embedding.
//
// The function should return a unit-length vector for efficient cosine similarity.
type EmbeddingFingerprintFunc func(text string) ([]float64, error)

// ComputeFingerprint builds a SemanticFingerprint for text.
// embedFn may be nil; in that case only the heuristic token set is computed.
func ComputeFingerprint(id, text string, embedFn EmbeddingFingerprintFunc) (SemanticFingerprint, error) {
	tokens := tokenise(text)
	fp := SemanticFingerprint{
		MessageID: id,
		Tokens:    tokens,
	}
	if embedFn != nil {
		vec, err := embedFn(text)
		if err != nil {
			return SemanticFingerprint{}, err
		}
		fp.Embedding = vec
	}
	return fp, nil
}

// tokenise produces a normalised bag-of-words from text.
// Converts to lowercase, strips punctuation, and removes common English stopwords.
func tokenise(text string) map[string]struct{} {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make(map[string]struct{}, len(words))
	for _, w := range words {
		if !isStopword(w) && len(w) > 2 {
			tokens[w] = struct{}{}
		}
	}
	return tokens
}

// SortedTokens returns the token set as a sorted slice (deterministic output).
func (f SemanticFingerprint) SortedTokens() []string {
	out := make([]string, 0, len(f.Tokens))
	for t := range f.Tokens {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// English stopwords excluded from fingerprinting.
var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "but": {},
	"in": {}, "on": {}, "at": {}, "to": {}, "for": {}, "of": {},
	"with": {}, "by": {}, "from": {}, "is": {}, "are": {}, "was": {},
	"were": {}, "be": {}, "been": {}, "being": {}, "have": {}, "has": {},
	"had": {}, "do": {}, "does": {}, "did": {}, "will": {}, "would": {},
	"could": {}, "should": {}, "may": {}, "might": {}, "can": {}, "not": {},
	"no": {}, "nor": {}, "so": {}, "yet": {}, "both": {}, "either": {},
	"neither": {}, "each": {}, "this": {}, "that": {}, "these": {}, "those": {},
	"i": {}, "me": {}, "my": {}, "we": {}, "our": {}, "you": {}, "your": {},
	"he": {}, "she": {}, "it": {}, "they": {}, "them": {}, "their": {},
	"what": {}, "which": {}, "who": {}, "whom": {}, "when": {}, "where": {},
	"why": {}, "how": {}, "all": {}, "any": {}, "few": {}, "more": {},
	"most": {}, "other": {}, "some": {}, "such": {}, "into": {}, "through": {},
	"during": {}, "before": {}, "after": {}, "above": {}, "below": {},
	"up": {}, "down": {}, "out": {}, "off": {}, "over": {}, "under": {},
	"again": {}, "then": {}, "once": {}, "only": {}, "own": {}, "same": {},
	"than": {}, "too": {}, "very": {}, "just": {}, "about": {}, "as": {},
}

func isStopword(w string) bool {
	_, ok := stopwords[w]
	return ok
}
