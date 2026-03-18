package llmctx

// ---------------------------------------------------------------------------
// ContextSnapshot
// ---------------------------------------------------------------------------

// ContextSnapshot is a serialisable point-in-time capture of an AgentContext.
type ContextSnapshot struct {
	SnapshotID MessageID     `json:"snapshot_id"`
	CapturedAt Timestamp     `json:"captured_at"`
	Window     ContextWindow `json:"window"`
	Messages   []*Message    `json:"messages"`
	Stats      ContextStats  `json:"stats"`
}
