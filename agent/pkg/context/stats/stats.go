// Package stats provides immutable metrics snapshot types for AgentContext.
//
// These are pure value types with no dependencies on the runtime state of
// an AgentContext. They can be logged, serialised, transmitted over a
// network, or compared across snapshots without retaining a reference to
// the originating context.
//
// The snapshot is produced by AgentContext.Stats() and covers:
//
//   - Volume:      message count and aggregate token total
//   - Window:      budget usage, remaining capacity, pressure level
//   - Composition: per-type and per-author breakdowns
//   - Visibility:  message counts per surface (user / LLM / developer)
//   - Distribution: avg / median / p90 / std-dev of per-message token counts
//   - Throughput:  append and removal rates since context creation
//   - Timing:      oldest/newest message timestamps, context age
//   - Compaction:  history of Compact() invocations
package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/quarkloop/agent/pkg/context/message"
	"github.com/quarkloop/agent/pkg/context/tokenizer"
)

// =============================================================================
// TypeStats
// =============================================================================

// TypeStats aggregates message count and token count for a single MessageType.
type TypeStats struct {
	Count      int32                 `json:"count"`
	TokenCount tokenizer.TokenCount  `json:"token_count"`
	// AvgTokens is the mean token count per message of this type.
	AvgTokens float64 `json:"avg_tokens"`
}

// =============================================================================
// AuthorStats
// =============================================================================

// AuthorStats aggregates message count and token count for a single author role.
type AuthorStats struct {
	Count      int32                `json:"count"`
	TokenCount tokenizer.TokenCount `json:"token_count"`
	// AvgTokens is the mean token count per message from this author.
	AvgTokens float64 `json:"avg_tokens"`
}

// =============================================================================
// VisibilityStats
// =============================================================================

// VisibilityStats breaks down how many messages are surfaced to each target.
type VisibilityStats struct {
	// UserVisible is the number of messages shown in the end-user chat UI.
	UserVisible int32 `json:"user_visible"`
	// LLMVisible is the number of messages included in the LLM context.
	LLMVisible int32 `json:"llm_visible"`
	// DeveloperVisible is the number of messages visible in developer tooling.
	DeveloperVisible int32 `json:"developer_visible"`
	// HiddenFromAll is the number of messages hidden on every surface.
	HiddenFromAll int32 `json:"hidden_from_all"`
}

// TokensByVisibility holds aggregate token totals broken down by surface.
type TokensByVisibility struct {
	UserTokens      tokenizer.TokenCount `json:"user_tokens"`
	LLMTokens       tokenizer.TokenCount `json:"llm_tokens"`
	DeveloperTokens tokenizer.TokenCount `json:"developer_tokens"`
}

// =============================================================================
// DistributionStats
// =============================================================================

// DistributionStats captures the statistical distribution of token counts
// across all messages in the context window.
type DistributionStats struct {
	// AvgTokensPerMessage is the arithmetic mean of per-message token counts.
	AvgTokensPerMessage float64 `json:"avg_tokens_per_message"`
	// MedianTokens is the median per-message token count (p50).
	MedianTokens float64 `json:"median_tokens"`
	// P90Tokens is the 90th-percentile per-message token count.
	P90Tokens float64 `json:"p90_tokens"`
	// MaxTokensPerMessage is the highest single-message token count.
	MaxTokensPerMessage int32 `json:"max_tokens_per_message"`
	// MinTokensPerMessage is the lowest single-message token count.
	MinTokensPerMessage int32 `json:"min_tokens_per_message"`
	// StdDevTokens is the standard deviation of per-message token counts.
	StdDevTokens float64 `json:"std_dev_tokens"`
	// PeakMessageID is the ID of the message with the highest token count.
	PeakMessageID string `json:"peak_message_id,omitempty"`
	// PeakMessageType is the type of the peak-token message.
	PeakMessageType message.MessageType `json:"peak_message_type,omitempty"`
}

// =============================================================================
// ThroughputStats
// =============================================================================

// ThroughputStats measures the rate of message and token activity since the
// AgentContext was created.
type ThroughputStats struct {
	// TotalAppended is the cumulative number of AppendMessage calls.
	TotalAppended int64 `json:"total_appended"`
	// TotalRemoved is the cumulative number of messages removed or evicted.
	TotalRemoved int64 `json:"total_removed"`
	// MessagesPerSecond is the average append rate since context creation.
	MessagesPerSecond float64 `json:"messages_per_second"`
	// TokensPerSecond is the average token ingest rate since context creation.
	TokensPerSecond float64 `json:"tokens_per_second"`
	// TotalTokensIngested is the sum of all token counts ever appended.
	TotalTokensIngested int64 `json:"total_tokens_ingested"`
	// ContextUptimeSeconds is the elapsed time since the context was created.
	ContextUptimeSeconds float64 `json:"context_uptime_seconds"`
}

// =============================================================================
// WindowPressure
// =============================================================================

// WindowPressure categorises how close the context is to its token limit.
type WindowPressure string

const (
	// PressureNone: window is unbounded or usage < 50%.
	PressureNone WindowPressure = "none"
	// PressureLow: 50% ≤ usage < 70%.
	PressureLow WindowPressure = "low"
	// PressureMedium: 70% ≤ usage < 85%.
	PressureMedium WindowPressure = "medium"
	// PressureHigh: 85% ≤ usage < 95%.
	PressureHigh WindowPressure = "high"
	// PressureCritical: usage ≥ 95% or IsOverLimit.
	PressureCritical WindowPressure = "critical"
)

// PressureFor maps a usage percentage and over-limit flag to a WindowPressure level.
func PressureFor(pct float64, overLimit bool) WindowPressure {
	if overLimit || pct >= 95 {
		return PressureCritical
	}
	if pct >= 85 {
		return PressureHigh
	}
	if pct >= 70 {
		return PressureMedium
	}
	if pct >= 50 {
		return PressureLow
	}
	return PressureNone
}

// =============================================================================
// CompactionEvent
// =============================================================================

// CompactionEvent records a single invocation of AgentContext.Compact().
type CompactionEvent struct {
	// OccurredAt is the wall-clock time of the compaction.
	OccurredAt time.Time `json:"occurred_at"`
	// TokensBefore is the total token count immediately before compaction.
	TokensBefore tokenizer.TokenCount `json:"tokens_before"`
	// TokensAfter is the total token count immediately after compaction.
	TokensAfter tokenizer.TokenCount `json:"tokens_after"`
	// MessagesRemoved is the count of evicted messages.
	MessagesRemoved int32 `json:"messages_removed"`
}

// TokensReclaimed returns the tokens freed by this compaction.
func (e CompactionEvent) TokensReclaimed() tokenizer.TokenCount {
	return e.TokensBefore.Sub(e.TokensAfter)
}

// CompressionRatio returns TokensAfter / TokensBefore as a [0,1] ratio.
// Returns 1.0 when TokensBefore is zero.
func (e CompactionEvent) CompressionRatio() float64 {
	if e.TokensBefore.IsZero() {
		return 1.0
	}
	return float64(e.TokensAfter.Value()) / float64(e.TokensBefore.Value())
}

// =============================================================================
// Snapshot (ContextStats)
// =============================================================================

// Snapshot is the full, immutable metrics snapshot for an AgentContext.
// Returned by AgentContext.Stats(). All fields are safe to read from multiple
// goroutines once the snapshot has been returned.
//
// This type is aliased as ContextStats in the root llmctx package for
// backward compatibility.
type Snapshot struct {
	// --- Volume ---

	TotalMessages int32                `json:"total_messages"`
	TotalTokens   tokenizer.TokenCount `json:"total_tokens"`

	// --- Window ---

	Window          tokenizer.ContextWindow `json:"window"`
	WindowUsagePct  float64                 `json:"window_usage_pct"`
	TokensRemaining tokenizer.TokenCount    `json:"tokens_remaining"`
	IsOverLimit     bool                    `json:"is_over_limit"`
	Pressure        WindowPressure          `json:"pressure"`

	// --- Composition ---

	// ByType is keyed by MessageType (a string alias).
	ByType map[message.MessageType]TypeStats `json:"by_type"`
	// ByAuthor is keyed by the author role string.
	ByAuthor map[string]AuthorStats `json:"by_author"`

	// --- Visibility ---

	Visibility         VisibilityStats    `json:"visibility"`
	TokensByVisibility TokensByVisibility `json:"tokens_by_visibility"`

	// --- Distribution ---

	Distribution DistributionStats `json:"distribution"`

	// --- Throughput ---

	Throughput ThroughputStats `json:"throughput"`

	// --- Timing ---

	CapturedAt              time.Time `json:"captured_at"`
	OldestMessageAt         time.Time `json:"oldest_message_at,omitempty"`
	NewestMessageAt         time.Time `json:"newest_message_at,omitempty"`
	ContextAgeSeconds       float64   `json:"context_age_seconds"`
	SecondsSinceLastMessage float64   `json:"seconds_since_last_message"`

	// --- Compaction ---

	CompactionCount         int32            `json:"compaction_count"`
	LastCompaction          *CompactionEvent `json:"last_compaction,omitempty"`
	TotalTokensReclaimed    tokenizer.TokenCount `json:"total_tokens_reclaimed"`
	AverageCompressionRatio float64          `json:"average_compression_ratio"`
}

// String returns a concise human-readable summary for logging.
func (s Snapshot) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "messages=%d tokens=%d", s.TotalMessages, s.TotalTokens.Value())
	if !s.Window.IsUnbound() {
		fmt.Fprintf(&sb, "/%d (%.1f%% %s)", s.Window.Value(), s.WindowUsagePct, s.Pressure)
	}
	if s.IsOverLimit {
		sb.WriteString(" OVER_LIMIT")
	}
	fmt.Fprintf(&sb, " vis=user:%d,llm:%d,dev:%d",
		s.Visibility.UserVisible, s.Visibility.LLMVisible, s.Visibility.DeveloperVisible)
	if s.CompactionCount > 0 {
		fmt.Fprintf(&sb, " compactions=%d reclaimed=%d",
			s.CompactionCount, s.TotalTokensReclaimed.Value())
	}
	return sb.String()
}

// Summarise returns a multi-line human-readable report suitable for debug output.
func (s Snapshot) Summarise() string {
	var sb strings.Builder
	sb.WriteString("╔═══════════════════════════════════════════╗\n")
	sb.WriteString("║            Context Stats                  ║\n")
	sb.WriteString("╠═══════════════════════════════════════════╣\n")

	fmt.Fprintf(&sb, "║  Messages    : %-5d\n", s.TotalMessages)
	if s.Window.IsUnbound() {
		fmt.Fprintf(&sb, "║  Tokens      : %d  (unbounded window)\n", s.TotalTokens.Value())
	} else {
		fmt.Fprintf(&sb, "║  Tokens      : %d / %d  (%.1f%%  remaining=%d)\n",
			s.TotalTokens.Value(), s.Window.Value(),
			s.WindowUsagePct, s.TokensRemaining.Value())
		fmt.Fprintf(&sb, "║  Pressure    : %s", s.Pressure)
		if s.IsOverLimit {
			sb.WriteString("  ⚠ OVER LIMIT")
		}
		sb.WriteByte('\n')
	}

	fmt.Fprintf(&sb, "║  Visibility  : user=%-3d  llm=%-3d  dev=%-3d  hidden=%d\n",
		s.Visibility.UserVisible, s.Visibility.LLMVisible,
		s.Visibility.DeveloperVisible, s.Visibility.HiddenFromAll)
	fmt.Fprintf(&sb, "║  Tok/surface : user=%-5d  llm=%-5d  dev=%d\n",
		s.TokensByVisibility.UserTokens.Value(),
		s.TokensByVisibility.LLMTokens.Value(),
		s.TokensByVisibility.DeveloperTokens.Value())

	if s.TotalMessages > 0 {
		d := s.Distribution
		fmt.Fprintf(&sb, "║  Tok/msg     : avg=%.1f  median=%.1f  p90=%.1f  σ=%.1f\n",
			d.AvgTokensPerMessage, d.MedianTokens, d.P90Tokens, d.StdDevTokens)
		fmt.Fprintf(&sb, "║  Peak msg    : %d tokens  id=%s  type=%s\n",
			d.MaxTokensPerMessage, d.PeakMessageID, d.PeakMessageType)
	}

	if len(s.ByType) > 0 {
		sb.WriteString("║  By type:\n")
		type kv struct {
			k message.MessageType
			v TypeStats
		}
		pairs := make([]kv, 0, len(s.ByType))
		for k, v := range s.ByType {
			pairs = append(pairs, kv{k, v})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].v.Count > pairs[j].v.Count })
		for _, p := range pairs {
			fmt.Fprintf(&sb, "║    %-18s  count=%-4d  tokens=%-6d  avg=%.1f\n",
				p.k, p.v.Count, p.v.TokenCount.Value(), p.v.AvgTokens)
		}
	}

	if len(s.ByAuthor) > 0 {
		sb.WriteString("║  By author:\n")
		type kv struct {
			k string
			v AuthorStats
		}
		pairs := make([]kv, 0, len(s.ByAuthor))
		for k, v := range s.ByAuthor {
			pairs = append(pairs, kv{k, v})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].v.Count > pairs[j].v.Count })
		for _, p := range pairs {
			fmt.Fprintf(&sb, "║    %-12s  count=%-4d  tokens=%-6d  avg=%.1f\n",
				p.k, p.v.Count, p.v.TokenCount.Value(), p.v.AvgTokens)
		}
	}

	t := s.Throughput
	if t.TotalAppended > 0 {
		fmt.Fprintf(&sb, "║  Throughput  : %.2f msg/s  %.1f tok/s  uptime=%.1fs\n",
			t.MessagesPerSecond, t.TokensPerSecond, t.ContextUptimeSeconds)
		fmt.Fprintf(&sb, "║  Ingested    : %d msgs total  %d tokens total\n",
			t.TotalAppended, t.TotalTokensIngested)
	}

	fmt.Fprintf(&sb, "║  Age         : %.2fs  (last msg %.2fs ago)\n",
		s.ContextAgeSeconds, s.SecondsSinceLastMessage)

	if s.CompactionCount > 0 {
		fmt.Fprintf(&sb, "║  Compaction  : runs=%d  reclaimed=%d  avg_ratio=%.2f\n",
			s.CompactionCount, s.TotalTokensReclaimed.Value(), s.AverageCompressionRatio)
		if s.LastCompaction != nil {
			e := s.LastCompaction
			fmt.Fprintf(&sb, "║  Last run    : %d→%d tokens  removed=%d  ratio=%.2f\n",
				e.TokensBefore.Value(), e.TokensAfter.Value(),
				e.MessagesRemoved, e.CompressionRatio())
		}
	}

	sb.WriteString("╚═══════════════════════════════════════════╝")
	return sb.String()
}

// =============================================================================
// Percentile helper
// =============================================================================

// Percentile returns the p-th percentile of a pre-sorted float64 slice.
// p must be in [0, 100].
func Percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := p / 100.0 * float64(n-1)
	lo := int(rank)
	hi := lo + 1
	if hi >= n {
		return sorted[n-1]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
