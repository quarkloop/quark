package llmctx

import (
	"context"
	"fmt"

	"github.com/quarkloop/agent/pkg/context/freshness"
	msg "github.com/quarkloop/agent/pkg/context/message"
)

// =============================================================================
// freshness_scan.go  —  Freshness / invalidation scanning on AgentContext
//
// Concept:
//   A message's content may be permanently correct (a contract date, a name)
//   but its contextual truth value can expire.  "The user is in Berlin" was
//   true at 10am; at 2pm they flew to Munich.  A system status message saying
//   "the database is healthy" is stale after 5 minutes.
//
//   FreshnessPolicy separates two concerns:
//     - IsStale(createdAt, ctx) → bool:  is this still valid?
//     - Refresh(ctx) → string:           what is the updated content?
//
// Usage:
//   1. Attach a policy when constructing or after appending a message:
//        msg = msg.WithFreshnessPolicy(freshness.NewTTLPolicy(5 * time.Minute))
//        ac.AppendMessage(msg)
//
//   2. Before each LLM request call ScanFreshness:
//        report, err := ac.ScanFreshness(vctx)
//        // inspect report.Stale, report.Refreshed, report.Removed
//
//   3. Or let the context self-heal with RefreshStale, which updates payloads
//      in-place for messages whose policies support Refresh:
//        report, err := ac.RefreshStale(vctx)
// =============================================================================

// StalenessAction describes what the scanner did (or recommends doing) with
// a stale message.
type StalenessAction string

const (
	// StalenessActionRefreshed means the message payload was updated in-place
	// with content returned by its policy's Refresh method.
	StalenessActionRefreshed StalenessAction = "refreshed"

	// StalenessActionFlagged means the message is stale but no Refresh
	// implementation was available.  The caller must decide what to do.
	StalenessActionFlagged StalenessAction = "flagged"

	// StalenessActionRemoved means the message was removed from the context
	// because it was stale and the caller requested removal of non-refreshable
	// stale messages.
	StalenessActionRemoved StalenessAction = "removed"
)

// StaleMessageRecord describes a single stale message found during a scan.
type StaleMessageRecord struct {
	// ID is the stale message's identifier.
	ID MessageID

	// Type is the payload kind.
	Type MessageType

	// PolicyDescription is the human-readable description of the policy that
	// flagged the message.
	PolicyDescription string

	// Action is what the scanner did with the message.
	Action StalenessAction

	// RefreshedContent is the new text when Action == StalenessActionRefreshed.
	// Empty for other actions.
	RefreshedContent string
}

// FreshnessScanReport is the immutable result of a ScanFreshness call.
type FreshnessScanReport struct {
	// Stale is every message the scanner flagged as stale, regardless of action.
	Stale []StaleMessageRecord

	// TotalScanned is the number of messages examined (those with a policy).
	TotalScanned int

	// RefreshedCount is the number of messages that were self-healed.
	RefreshedCount int

	// FlaggedCount is the number of messages flagged but not updated.
	FlaggedCount int

	// RemovedCount is the number of messages removed.
	RemovedCount int
}

// HasIssues reports whether any messages were stale.
func (r FreshnessScanReport) HasIssues() bool { return len(r.Stale) > 0 }

// =============================================================================
// ScanFreshness
// =============================================================================

// ScanFreshness examines every message that carries a FreshnessPolicy.
//
// For each stale message:
//   - If the policy's Refresh returns non-empty content, the message payload
//     is updated in-place using UpdateMessagePayload (with recomputed tokens).
//   - Otherwise the message is recorded as StalenessActionFlagged.
//
// ctx is forwarded into every Refresh call, allowing slow external refresh
// implementations (e.g. database lookups) to be cancelled by the caller.
//
// Pass freshness.ValidationContext with at minimum Now set to time.Now().UTC().
// Optional fields (Location, RequestCount, SessionID, Extra) are consulted by
// the relevant policy implementations.
//
// ScanFreshness is safe for concurrent use; it acquires the write lock only
// when updating a message payload.
func (ac *AgentContext) ScanFreshness(ctx context.Context, vctx freshness.ValidationContext) (FreshnessScanReport, error) {
	return ac.scanFreshness(ctx, vctx, false)
}

// RefreshStale is identical to ScanFreshness but also removes stale messages
// that have no Refresh implementation (StalenessActionRemoved instead of
// StalenessActionFlagged).
//
// Use this when you want the context to be fully self-healing: messages that
// can be refreshed are updated; those that cannot are dropped entirely.
func (ac *AgentContext) RefreshStale(ctx context.Context, vctx freshness.ValidationContext) (FreshnessScanReport, error) {
	return ac.scanFreshness(ctx, vctx, true)
}

func (ac *AgentContext) scanFreshness(
	ctx context.Context,
	vctx freshness.ValidationContext,
	removeUnrefreshable bool,
) (FreshnessScanReport, error) {
	if err := ctx.Err(); err != nil {
		return FreshnessScanReport{}, newErr(ErrCodeInvalidMessage, "ScanFreshness: context cancelled", err)
	}

	// Snapshot the message slice under a read lock to avoid holding the write
	// lock during potentially slow Refresh calls (e.g. external API calls).
	ac.mu.RLock()
	snapshot := make([]*Message, len(ac.messages))
	copy(snapshot, ac.messages)
	ac.mu.RUnlock()

	var report FreshnessScanReport
	var toRemove []MessageID

	for _, m := range snapshot {
		// Honour cancellation between messages.
		if err := ctx.Err(); err != nil {
			return report, newErr(ErrCodeInvalidMessage, "ScanFreshness: context cancelled during scan", err)
		}

		policy := m.FreshnessPolicy()
		if policy == nil {
			continue
		}
		report.TotalScanned++

		if !policy.IsStale(m.createdAt.Time(), vctx) {
			continue
		}

		// Attempt self-healing via Refresh.
		newContent, err := policy.Refresh(vctx)
		if err != nil {
			return report, newErr(ErrCodeInvalidMessage,
				fmt.Sprintf("freshness: Refresh failed for message %s", m.ID()), err)
		}

		if newContent != "" {
			// Build a new payload of the same type carrying the refreshed text.
			refreshedPayload, buildErr := buildRefreshedPayload(m, newContent)
			if buildErr != nil {
				return report, buildErr
			}
			if updateErr := ac.UpdateMessagePayload(ctx, m.ID(), refreshedPayload); updateErr != nil {
				return report, updateErr
			}
			report.Stale = append(report.Stale, StaleMessageRecord{
				ID:                m.ID(),
				Type:              m.Type(),
				PolicyDescription: policy.Description(),
				Action:            StalenessActionRefreshed,
				RefreshedContent:  newContent,
			})
			report.RefreshedCount++
			continue
		}

		// No Refresh content available.
		if removeUnrefreshable {
			toRemove = append(toRemove, m.ID())
			report.Stale = append(report.Stale, StaleMessageRecord{
				ID:                m.ID(),
				Type:              m.Type(),
				PolicyDescription: policy.Description(),
				Action:            StalenessActionRemoved,
			})
			report.RemovedCount++
		} else {
			report.Stale = append(report.Stale, StaleMessageRecord{
				ID:                m.ID(),
				Type:              m.Type(),
				PolicyDescription: policy.Description(),
				Action:            StalenessActionFlagged,
			})
			report.FlaggedCount++
		}
	}

	// Remove messages outside the per-message loop to avoid index shifts.
	for _, id := range toRemove {
		if err := ac.RemoveMessageByID(ctx, id); err != nil {
			// Message may have been removed concurrently — not an error.
			if ce, ok := err.(*ContextError); ok && ce.Code == ErrCodeMessageNotFound {
				continue
			}
			return report, err
		}
	}

	return report, nil
}

// =============================================================================
// buildRefreshedPayload
// =============================================================================

// buildRefreshedPayload creates a new Payload of the same kind as the
// existing message but with the text field replaced by refreshedText.
//
// This is limited to the text-bearing payload types.  Image, Audio, and PDF
// payloads cannot be refreshed this way (their Data field is opaque bytes);
// callers should use a full UpdateMessagePayload for those.
func buildRefreshedPayload(m *Message, refreshedText string) (msg.Payload, error) {
	switch m.Type() {
	case TextMessageType:
		p, _ := m.AsText()
		p.Text = refreshedText
		return p, nil
	case SystemPromptType:
		p, _ := m.AsSystemPrompt()
		p.Text = refreshedText
		return p, nil
	case MemoryMessageType:
		p, _ := m.AsMemory()
		p.Summary = refreshedText
		return p, nil
	case ReasoningMessageType:
		p, _ := m.AsReasoning()
		p.Reasoning = refreshedText
		return p, nil
	case ErrorMessageType:
		p, _ := m.AsError()
		p.Message = refreshedText
		return p, nil
	default:
		return nil, newErr(ErrCodeInvalidMessage,
			fmt.Sprintf("freshness: cannot auto-refresh payload kind %q (no text field)", m.Type()), nil)
	}
}
