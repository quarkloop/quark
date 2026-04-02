package model

import (
	"context"
	"regexp"
	"strings"
)

// RoutingRule maps a regex pattern to a specific gateway.
type RoutingRule struct {
	Match   string
	re      *regexp.Regexp
	Gateway Gateway
}

// RoutingGateway selects a gateway per request based on routing rules.
type RoutingGateway struct {
	rules    []*RoutingRule
	default_ Gateway
}

// NewRoutingGateway creates a routing gateway.
func NewRoutingGateway(rules []RoutingRule, default_ Gateway) *RoutingGateway {
	compiled := make([]*RoutingRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Match)
		if err != nil {
			continue
		}
		compiled = append(compiled, &RoutingRule{
			Match:   r.Match,
			re:      re,
			Gateway: r.Gateway,
		})
	}
	return &RoutingGateway{
		rules:    compiled,
		default_: default_,
	}
}

func (r *RoutingGateway) InferRaw(ctx context.Context, payload []byte) (*RawResponse, error) {
	gw := r.selectGateway(payload)
	return gw.InferRaw(ctx, payload)
}

func (r *RoutingGateway) Provider() string {
	return r.default_.Provider()
}

func (r *RoutingGateway) ModelName() string {
	return r.default_.ModelName()
}

func (r *RoutingGateway) MaxTokens() int {
	return r.default_.MaxTokens()
}

func (r *RoutingGateway) Parser() ToolCallParser {
	return r.default_.Parser()
}

func (r *RoutingGateway) selectGateway(payload []byte) Gateway {
	msg := extractUserMessage(payload)
	if msg == "" {
		return r.default_
	}
	for _, rule := range r.rules {
		if rule.re.MatchString(msg) {
			return rule.Gateway
		}
	}
	return r.default_
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
