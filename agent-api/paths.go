package agentapi

import (
	"fmt"
	"strings"
)

// API path suffixes appended to the base path. The direct agent serves
// these under DefaultBasePath ("/api/v1/agent"); the api-server proxy
// serves them under "/api/v1/agents/{id}".
const (
	// PathHealth is the liveness probe endpoint (GET). Returns agent ID
	// and a simple status string. Designed for fast port scanning.
	PathHealth = "/health"

	// PathInfo is the agent metadata endpoint (GET). Returns provider,
	// model, current mode, and registered tools. Use for UI display.
	PathInfo = "/info"

	// PathMode returns the agent's current working mode (GET).
	PathMode = "/mode"

	// PathStats returns agent statistics including context metrics (GET).
	PathStats = "/stats"

	// PathChat accepts a user message and returns the agent's reply (POST).
	// Supports JSON and multipart/form-data (for file attachments).
	PathChat = "/chat"

	// PathStop requests a graceful agent shutdown (POST).
	PathStop = "/stop"

	// PathPlan returns the agent's current execution plan (GET).
	PathPlan = "/plan"

	// PathActivity returns historical activity records (GET).
	// Accepts an optional ?limit= query parameter.
	PathActivity = "/activity"

	// PathActivityStream opens a Server-Sent Events stream of real-time
	// activity records (GET). Each SSE frame is a JSON ActivityRecord.
	PathActivityStream = "/activity/stream"
)

// JoinPath concatenates a base path and a suffix, stripping trailing slashes.
func JoinPath(basePath, suffix string) string {
	basePath = strings.TrimRight(basePath, "/")
	if basePath == "" {
		return suffix
	}
	return basePath + suffix
}

// AgentProxyBasePath returns the api-server URL prefix for a specific agent.
func AgentProxyBasePath(agentID string) string {
	return fmt.Sprintf("/api/v1/agents/%s", agentID)
}

// AgentProxyBasePattern returns the api-server URL pattern with a path
// variable placeholder for route registration.
func AgentProxyBasePattern() string {
	return "/api/v1/agents/{id}"
}
