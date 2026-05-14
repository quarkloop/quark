package api

import "github.com/quarkloop/runtime/pkg/message"

type errorResponse struct {
	Error string `json:"error"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

type messageResponse struct {
	ID        string `json:"id,omitempty"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

func mapMessageResponses(messages []message.Message) []messageResponse {
	out := make([]messageResponse, 0, len(messages))
	for _, msg := range messages {
		out = append(out, messageResponse{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		})
	}
	return out
}

type agentInfoResponse struct {
	ID           string   `json:"id"`
	Sessions     int      `json:"sessions"`
	WorkStatus   string   `json:"work_status"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
	Channels     any      `json:"channels,omitempty"`
}

type channelsResponse struct {
	Active    any `json:"active"`
	Available any `json:"available"`
}
