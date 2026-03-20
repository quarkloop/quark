package agentapi

import (
	"fmt"
	"strings"
)

const (
	PathHealth         = "/health"
	PathMode           = "/mode"
	PathStats          = "/stats"
	PathChat           = "/chat"
	PathStop           = "/stop"
	PathPlan           = "/plan"
	PathActivity       = "/activity"
	PathActivityStream = "/activity/stream"
)

func JoinPath(basePath, suffix string) string {
	basePath = strings.TrimRight(basePath, "/")
	if basePath == "" {
		return suffix
	}
	return basePath + suffix
}

func AgentProxyBasePath(agentID string) string {
	return fmt.Sprintf("/api/v1/agents/%s", agentID)
}

func AgentProxyBasePattern() string {
	return "/api/v1/agents/{id}"
}
