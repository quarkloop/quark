package model

import "time"

// Event is one row in GET /events.
type Event struct {
	ID           string                 `json:"id"`
	Kind         string                 `json:"kind"`
	NodeName string                 `json:"nodeName"`
	SystemName string                 `json:"systemName"`
	Namespace    string                 `json:"namespace"`
	Timestamp    time.Time              `json:"timestamp"`
	Payload      map[string]interface{} `json:"payload"`
}
