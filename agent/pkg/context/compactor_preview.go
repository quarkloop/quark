package llmctx

import comp "github.com/quarkloop/agent/pkg/context/compactor"

// =============================================================================
// compactor_preview.go  —  Root-package preview bridge
//
// CompactionPreview, EvictedMessage, CompactorWithPreview, and
// PreviewCompaction are all aliases/wrappers over llmctx/compactor types.
// The canonical implementations live there.
// =============================================================================

// PreviewCompaction performs a dry-run using any Compactor.
// If c implements CompactorWithPreview, its native Preview is called.
// Otherwise Compact is called on a shallow clone and the result is diffed.
func PreviewCompaction(c Compactor, messages []*Message, window ContextWindow) CompactionPreview {
	compMsgs := toCompactorMessages(messages)
	// If root compactor embeds a CompactorWithPreview (our rootCompactorAdapter does),
	// call it directly. Otherwise use the sub-package universal fallback.
	if cwp, ok := c.(CompactorWithPreview); ok {
		return cwp.Preview(messages, window)
	}
	// Fallback: wrap the root compactor as a comp.Compactor and use sub-package preview.
	return comp.PreviewCompact(&rootToCompAdapter{c}, compMsgs, window)
}

// PreviewCompact on AgentContext performs a dry-run without modifying the context.
// Returns ErrCodeNoCompactor when no compactor has been set.
func (ac *AgentContext) PreviewCompact() (CompactionPreview, error) {
	if ac.compactor == nil {
		return CompactionPreview{}, newErr(ErrCodeNoCompactor,
			"no compactor configured; set one via AgentContextBuilder.WithCompactor", nil)
	}
	ac.mu.RLock()
	msgs := cloneMessages(ac.messages)
	window := ac.contextWindow
	ac.mu.RUnlock()
	return PreviewCompaction(ac.compactor, msgs, window), nil
}

// makeEvicted creates an EvictedMessage from a root *Message.
// Used by graph_compactor.go preview path.
func makeEvicted(m *Message, pos int) EvictedMessage {
	return EvictedMessage{
		ID:       m.ID().String(),
		Type:     m.Type(),
		Author:   string(m.Author()),
		Weight:   m.Weight().Value(),
		Tokens:   m.TokenCount(),
		Position: pos,
	}
}

// noopPreview returns a preview indicating no compaction would occur.
func noopPreview(msgCount int, tokens TokenCount) CompactionPreview {
	return CompactionPreview{
		WouldCompact:  false,
		RetainedCount: msgCount,
		TokensBefore:  tokens,
		TokensAfter:   tokens,
		WouldFit:      true,
	}
}

// buildPreviewFromDiff builds a CompactionPreview by comparing original and compacted slices.
func buildPreviewFromDiff(
	original, compacted []*Message,
	tokensBefore TokenCount,
	window ContextWindow,
) CompactionPreview {
	retained := make(map[string]bool, len(compacted))
	for _, m := range compacted {
		retained[m.ID().String()] = true
	}
	var evicted []EvictedMessage
	for i, m := range original {
		if !retained[m.ID().String()] {
			evicted = append(evicted, makeEvicted(m, i))
		}
	}
	tokensAfter := totalTokens(compacted)
	reclaimed := tokensBefore.Sub(tokensAfter)
	return CompactionPreview{
		WouldCompact:    len(evicted) > 0,
		Evicted:         evicted,
		RetainedCount:   len(compacted),
		TokensBefore:    tokensBefore,
		TokensAfter:     tokensAfter,
		TokensReclaimed: reclaimed,
		WouldFit:        !tokensAfter.ExceedsWindow(window),
	}
}
