package llmctx

import (
	"time"

	ctxstats "github.com/quarkloop/agent/pkg/context/stats"
)

// =============================================================================
// stats.go  —  Root-package re-exports of llmctx/stats + private internals
//
// Public types live in llmctx/stats. These aliases let callers who import
// only "llmctx" use them without a separate import.
//
// buildStats, compactionTracker, throughputTracker stay here because they
// are private to AgentContext and need []*Message access.
// =============================================================================

// Public type aliases from llmctx/stats.
type (
	TypeStats          = ctxstats.TypeStats
	AuthorStats        = ctxstats.AuthorStats
	VisibilityStats    = ctxstats.VisibilityStats
	TokensByVisibility = ctxstats.TokensByVisibility
	ThroughputStats    = ctxstats.ThroughputStats
	WindowPressure     = ctxstats.WindowPressure
	CompactionEvent    = ctxstats.CompactionEvent
	ContextStats       = ctxstats.Snapshot // ContextStats is the canonical name in the root package
)

const (
	PressureNone     = ctxstats.PressureNone
	PressureLow      = ctxstats.PressureLow
	PressureMedium   = ctxstats.PressureMedium
	PressureHigh     = ctxstats.PressureHigh
	PressureCritical = ctxstats.PressureCritical
)

// windowPressureFor maps a usage percentage to a WindowPressure level.
func windowPressureFor(pct float64, overLimit bool) WindowPressure {
	return ctxstats.PressureFor(pct, overLimit)
}

// -----------------------------------------------------------------------------
// compactionTracker — private to AgentContext
// -----------------------------------------------------------------------------

type compactionTracker struct {
	count          int32
	lastEvent      *CompactionEvent
	totalReclaimed TokenCount
	ratioSum       float64
}

func (t *compactionTracker) record(before, after TokenCount, removed int32) {
	e := &CompactionEvent{
		OccurredAt:      time.Now().UTC(),
		TokensBefore:    before,
		TokensAfter:     after,
		MessagesRemoved: removed,
	}
	t.lastEvent = e
	t.count++
	t.totalReclaimed = t.totalReclaimed.Add(e.TokensReclaimed())
	t.ratioSum += e.CompressionRatio()
}

func (t *compactionTracker) avgCompressionRatio() float64 {
	if t.count == 0 {
		return 0
	}
	return t.ratioSum / float64(t.count)
}

// -----------------------------------------------------------------------------
// throughputTracker — private to AgentContext
// -----------------------------------------------------------------------------

type throughputTracker struct {
	totalAppended int64
	totalRemoved  int64
}

func newThroughputTracker() throughputTracker {
	return throughputTracker{}
}

func (t *throughputTracker) recordAppend() {
	t.totalAppended++
}

func (t *throughputTracker) recordRemove(n int) {
	t.totalRemoved += int64(n)
}

func (t *throughputTracker) snapshot() ThroughputStats {
	return ThroughputStats{
		TotalAppended: t.totalAppended,
		TotalRemoved:  t.totalRemoved,
	}
}

// -----------------------------------------------------------------------------
// buildStats — pure function, needs []*Message and AgentContext internals
// -----------------------------------------------------------------------------

func buildStats(
	messages []*Message,
	window ContextWindow,
	cachedTokens TokenCount,
	tracker compactionTracker,
	tput throughputTracker,
) ContextStats {
	byType := make(map[MessageType]TypeStats, 12)
	byAuthor := make(map[string]AuthorStats, 4)

	var vis VisibilityStats
	var tvByVis TokensByVisibility
	var oldest, newest time.Time

	for _, m := range messages {
		tc := m.TokenCount()
		mt := m.Type()
		ma := string(m.Author())

		ts := byType[mt]
		ts.Count++
		ts.TokenCount = ts.TokenCount.Add(tc)
		byType[mt] = ts

		as := byAuthor[ma]
		as.Count++
		as.TokenCount = as.TokenCount.Add(tc)
		byAuthor[ma] = as

		anyVisible := false
		if m.IsVisibleTo(VisibleToUser) {
			vis.UserVisible++
			tvByVis.UserTokens = tvByVis.UserTokens.Add(tc)
			anyVisible = true
		}
		if m.IsVisibleTo(VisibleToLLM) {
			vis.LLMVisible++
			tvByVis.LLMTokens = tvByVis.LLMTokens.Add(tc)
			anyVisible = true
		}
		if m.IsVisibleTo(VisibleToDeveloper) {
			vis.DeveloperVisible++
			tvByVis.DeveloperTokens = tvByVis.DeveloperTokens.Add(tc)
			anyVisible = true
		}
		if !anyVisible {
			vis.HiddenFromAll++
		}

		ca := m.CreatedAt().Time()
		if oldest.IsZero() || ca.Before(oldest) {
			oldest = ca
		}
		if newest.IsZero() || ca.After(newest) {
			newest = ca
		}
	}

	for k, ts := range byType {
		if ts.Count > 0 {
			ts.AvgTokens = float64(ts.TokenCount.Value()) / float64(ts.Count)
			byType[k] = ts
		}
	}
	for k, as := range byAuthor {
		if as.Count > 0 {
			as.AvgTokens = float64(as.TokenCount.Value()) / float64(as.Count)
			byAuthor[k] = as
		}
	}

	var ageSeconds, sinceLastMsg float64
	if !oldest.IsZero() && !newest.IsZero() {
		ageSeconds = newest.Sub(oldest).Seconds()
	}
	if !newest.IsZero() {
		sinceLastMsg = time.Since(newest).Seconds()
	}

	var remaining TokenCount
	if !window.IsUnbound() {
		windowTC, _ := NewTokenCount(window.Value())
		remaining = windowTC.Sub(cachedTokens)
	}

	usagePct := window.UsagePct(cachedTokens)
	overLimit := cachedTokens.ExceedsWindow(window)

	return ContextStats{
		TotalMessages:           int32(len(messages)),
		TotalTokens:             cachedTokens,
		Window:                  window,
		WindowUsagePct:          usagePct,
		TokensRemaining:         remaining,
		IsOverLimit:             overLimit,
		Pressure:                windowPressureFor(usagePct, overLimit),
		ByType:                  byType,
		ByAuthor:                byAuthor,
		Visibility:              vis,
		TokensByVisibility:      tvByVis,
		Throughput:              tput.snapshot(),
		CapturedAt:              time.Now().UTC(),
		OldestMessageAt:         oldest,
		NewestMessageAt:         newest,
		ContextAgeSeconds:       ageSeconds,
		SecondsSinceLastMessage: sinceLastMsg,
		CompactionCount:         tracker.count,
		LastCompaction:          tracker.lastEvent,
		TotalTokensReclaimed:    tracker.totalReclaimed,
		AverageCompressionRatio: tracker.avgCompressionRatio(),
	}
}
