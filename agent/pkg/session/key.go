package session

import (
	"fmt"
	"strings"
)

// Key construction functions produce hierarchical session keys.
// Format: "agent:<agentID>:<type>[:<id>]"

// MainKey returns the key for an agent's persistent main session.
func MainKey(agentID string) string {
	return fmt.Sprintf("agent:%s:main", agentID)
}

// ChatKey returns the key for a user chat session.
func ChatKey(agentID, sessionID string) string {
	return fmt.Sprintf("agent:%s:chat:%s", agentID, sessionID)
}

// SubAgentKey returns the key for a worker/sub-agent session.
func SubAgentKey(agentID, childID string) string {
	return fmt.Sprintf("agent:%s:subagent:%s", agentID, childID)
}

// CronKey returns the key for a scheduled task session run.
func CronKey(agentID, cronID string, run int) string {
	return fmt.Sprintf("agent:%s:cron:%s:run:%d", agentID, cronID, run)
}

// ParseKey extracts the agent ID, session type, and session-specific ID
// from a hierarchical session key.
func ParseKey(key string) (agentID string, t Type, sessionID string, err error) {
	parts := strings.Split(key, ":")
	if len(parts) < 3 || parts[0] != "agent" {
		return "", "", "", fmt.Errorf("invalid session key %q: expected agent:<id>:<type>[:<id>]", key)
	}
	agentID = parts[1]
	switch Type(parts[2]) {
	case TypeMain:
		return agentID, TypeMain, "", nil
	case TypeChat:
		if len(parts) < 4 {
			return "", "", "", fmt.Errorf("invalid chat session key %q: missing session ID", key)
		}
		return agentID, TypeChat, parts[3], nil
	case TypeSubAgent:
		if len(parts) < 4 {
			return "", "", "", fmt.Errorf("invalid subagent session key %q: missing child ID", key)
		}
		return agentID, TypeSubAgent, parts[3], nil
	case TypeCron:
		if len(parts) < 4 {
			return "", "", "", fmt.Errorf("invalid cron session key %q: missing cron ID", key)
		}
		// Return everything after "cron:" as the session ID (includes run number).
		return agentID, TypeCron, strings.Join(parts[3:], ":"), nil
	default:
		return "", "", "", fmt.Errorf("unknown session type %q in key %q", parts[2], key)
	}
}

// ValidateKey checks that a session key is well-formed.
func ValidateKey(key string) error {
	_, _, _, err := ParseKey(key)
	return err
}
