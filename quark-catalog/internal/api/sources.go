package api

// --- Source request/response types ---
//
// "Source" is the original .quark.ts text. It is stored as a TEXT column
// on the systems table (no separate sources table) and surfaced via
// dedicated NATS subjects so callers don't have to know that detail.

// SaveSourceRequest updates the source column for an existing system.
type SaveSourceRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Source    string `json:"source"`
}

// GetSourceRequest fetches the source text for a system.
type GetSourceRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// SourceEntry is a (namespace, name) pair returned by ListSources.
type SourceEntry struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// SourceListResponse wraps a slice of SourceEntry.
type SourceListResponse struct {
	Sources []SourceEntry `json:"sources"`
}

// SourceResponse is the body returned by GetSource.
type SourceResponse struct {
	Source string `json:"source"`
}
