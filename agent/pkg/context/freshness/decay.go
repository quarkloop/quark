package freshness

import (
	"fmt"
	"math"
	"time"
)

// DecayFunction maps (age, position, totalMessages) to a multiplier in [0,1].
//
// The result is applied to a message's base weight during compaction:
//
//	effectiveWeight = baseWeight * decayFn(age, position, total)
//
// 1.0 means full importance (no decay); 0.0 means fully decayed (evict first).
//
// The function accepts age as a Duration (not a wall-clock time) so that tests
// can control time deterministically without mocking.
type DecayFunction func(age time.Duration, position int, totalMessages int) float64

// NoDecay returns 1.0 for all inputs — the message never loses relevance.
func NoDecay() DecayFunction {
	return func(_ time.Duration, _ int, _ int) float64 { return 1.0 }
}

// LinearDecay returns a function that decays from 1.0 to 0.0 over halfLife.
// After halfLife the score is 0.5; after 2×halfLife it reaches 0.
//
// The score is floored at 0.
func LinearDecay(halfLife time.Duration) DecayFunction {
	if halfLife <= 0 {
		panic(fmt.Sprintf("freshness: LinearDecay halfLife must be > 0, got %v", halfLife))
	}
	return func(age time.Duration, _ int, _ int) float64 {
		score := 1.0 - (float64(age) / float64(2*halfLife))
		if score < 0 {
			return 0
		}
		return score
	}
}

// ExponentialDecay returns a function that halves the score every halfLife.
//
// The formula is: score = e^(-λ·age) where λ = ln(2)/halfLife.
// Score approaches 0 asymptotically and is always > 0.
func ExponentialDecay(halfLife time.Duration) DecayFunction {
	if halfLife <= 0 {
		panic(fmt.Sprintf("freshness: ExponentialDecay halfLife must be > 0, got %v", halfLife))
	}
	lambda := math.Log(2) / float64(halfLife)
	return func(age time.Duration, _ int, _ int) float64 {
		return math.Exp(-lambda * float64(age))
	}
}

// StepDecay returns a function that keeps full relevance until threshold, then
// drops to floorScore instantly.
//
// Example — full importance for the first 10 minutes, then 0.1:
//
//	StepDecay(10 * time.Minute, 0.1)
func StepDecay(threshold time.Duration, floorScore float64) DecayFunction {
	if threshold <= 0 {
		panic(fmt.Sprintf("freshness: StepDecay threshold must be > 0, got %v", threshold))
	}
	if floorScore < 0 || floorScore > 1 {
		panic(fmt.Sprintf("freshness: StepDecay floorScore must be in [0,1], got %v", floorScore))
	}
	return func(age time.Duration, _ int, _ int) float64 {
		if age < threshold {
			return 1.0
		}
		return floorScore
	}
}

// PositionDecay returns a function that weights messages by their recency of
// position in the context window.
//
// The most-recent message (highest index) scores 1.0.
// The oldest message (index 0) scores minScore.
//
// This is useful as a tiebreaker when ages are similar.
func PositionDecay(minScore float64) DecayFunction {
	if minScore < 0 || minScore > 1 {
		panic(fmt.Sprintf("freshness: PositionDecay minScore must be in [0,1], got %v", minScore))
	}
	return func(_ time.Duration, position int, total int) float64 {
		if total <= 1 {
			return 1.0
		}
		normalised := float64(position) / float64(total-1)
		return minScore + normalised*(1.0-minScore)
	}
}

// CombinedDecay blends multiple DecayFunctions using equal weights.
//
// The result is the arithmetic mean of all component scores.
func CombinedDecay(fns ...DecayFunction) DecayFunction {
	if len(fns) == 0 {
		return NoDecay()
	}
	return func(age time.Duration, position int, total int) float64 {
		sum := 0.0
		for _, fn := range fns {
			sum += fn(age, position, total)
		}
		return sum / float64(len(fns))
	}
}

// =============================================================================
// DecayScorer — bridges DecayFunction into the compactor's ScorerFunc API
// =============================================================================

// DecayResult is the output of evaluating a DecayFunction for a specific message.
type DecayResult struct {
	// Score is the computed importance multiplier in [0,1].
	Score float64
	// Age is the duration since the message was created.
	Age time.Duration
}

// EvaluateDecay applies fn to a message's age and position metadata.
//
//	createdAt   — message creation timestamp
//	position    — index in the current message list
//	total       — total number of messages in the list
//	now         — current wall-clock time
func EvaluateDecay(fn DecayFunction, createdAt time.Time, position int, total int, now time.Time) DecayResult {
	age := now.Sub(createdAt)
	if age < 0 {
		age = 0
	}
	return DecayResult{
		Score: fn(age, position, total),
		Age:   age,
	}
}
