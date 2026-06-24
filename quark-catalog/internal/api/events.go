package api

// --- Event request/response types ---

// AppendEventRequest appends a single event row (insert-or-replace by ID).
type AppendEventRequest struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	NodeName   string         `json:"nodeName"`
	SystemName string         `json:"systemName"`
	Namespace  string         `json:"namespace"`
	Timestamp  string         `json:"timestamp"`
	Payload    map[string]any `json:"payload,omitempty"`
}

// AppendEventsRequest batches an AppendEvent operation across many events.
type AppendEventsRequest struct {
	Events []AppendEventRequest `json:"events"`
}

// QueryEventsRequest selects events by optional filters. Limit defaults
// to 100 and is clamped to 10000 by the store.
type QueryEventsRequest struct {
	Namespace  string   `json:"namespace"`
	SystemName string   `json:"systemName"`
	NodeName   string   `json:"nodeName"`
	Kinds      []string `json:"kinds"`
	Limit      int      `json:"limit"`
}

// CountEventsRequest counts events by optional filters.
type CountEventsRequest struct {
	Namespace  string   `json:"namespace"`
	SystemName string   `json:"systemName"`
	NodeName   string   `json:"nodeName"`
	Kinds      []string `json:"kinds"`
}

// EventResponse is the JSON shape returned for a single event row.
type EventResponse struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	NodeName   string         `json:"nodeName"`
	SystemName string         `json:"systemName"`
	Namespace  string         `json:"namespace"`
	Timestamp  string         `json:"timestamp"`
	Payload    map[string]any `json:"payload,omitempty"`
}

// EventListResponse wraps a slice of EventResponse.
type EventListResponse struct {
	Events []EventResponse `json:"events"`
}

// CountResponse is returned by CountEvents.
type CountResponse struct {
	Count int64 `json:"count"`
}
