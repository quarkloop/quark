package llmctx

// =============================================================================
// compactor_preview.go  —  Preview helpers for dry-run compaction
// =============================================================================

// PreviewCompaction performs a dry-run using any Compactor.
// If c implements CompactorWithPreview, its native Preview is called.
// Otherwise Compact is called on a shallow clone and the result is diffed.
func PreviewCompaction(c Compactor, messages []*Message, window ContextWindow) CompactionPreview {
	if cwp, ok := c.(CompactorWithPreview); ok {
		return cwp.Preview(messages, window)
	}
	// Fallback: run Compact on a clone and diff the results.
	tokensBefore := totalTokens(messages)
	clone := cloneMessages(messages)
	compacted, err := c.Compact(clone, window)
	if err != nil {
		return noopPreview(len(messages), tokensBefore)
	}
	return buildPreviewFromDiff(messages, compacted, tokensBefore, window)
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

// makePreview builds a CompactionPreview from pre-computed eviction data.
func makePreview(evicted []EvictedMessage, msgCount int, tokensBefore, tokensAfter TokenCount, window ContextWindow) CompactionPreview {
	reclaimed := tokensBefore.Sub(tokensAfter)
	return CompactionPreview{
		WouldCompact:    len(evicted) > 0,
		Evicted:         evicted,
		RetainedCount:   msgCount - len(evicted),
		TokensBefore:    tokensBefore,
		TokensAfter:     tokensAfter,
		TokensReclaimed: reclaimed,
		WouldFit:        !tokensAfter.ExceedsWindow(window),
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
