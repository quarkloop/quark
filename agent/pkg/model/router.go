package model

import (
	"regexp"
	"strings"
)

// RoutingRule maps a regex pattern to a specific gateway.
type RoutingRule struct {
	Match   string
	re      *regexp.Regexp
	Gateway Gateway
}

// extractUserMessage pulls the last user message content from a JSON payload.
func extractUserMessage(payload []byte) string {
	// Simple extraction — look for the last "role":"user" message content.
	// This avoids full JSON parsing for performance.
	str := string(payload)
	lastUser := strings.LastIndex(str, `"role":"user"`)
	if lastUser < 0 {
		lastUser = strings.LastIndex(str, `"role": "user"`)
	}
	if lastUser < 0 {
		return ""
	}

	// Find the content field after this role.
	contentIdx := strings.Index(str[lastUser:], `"content"`)
	if contentIdx < 0 {
		return ""
	}
	contentIdx += lastUser

	// Find the string value after "content":
	valStart := strings.Index(str[contentIdx:], `": "`)
	if valStart < 0 {
		valStart = strings.Index(str[contentIdx:], `":"`)
	}
	if valStart < 0 {
		return ""
	}
	valStart += contentIdx + 3

	valEnd := strings.Index(str[valStart:], `"`)
	if valEnd < 0 {
		return ""
	}

	msg := str[valStart : valStart+valEnd]
	// Unescape JSON string.
	msg = strings.ReplaceAll(msg, `\"`, `"`)
	msg = strings.ReplaceAll(msg, `\n`, "\n")
	return msg
}
