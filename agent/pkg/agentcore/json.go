package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractJSON pulls the first JSON object from a string (handles markdown fences).
func ExtractJSON(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		end := strings.LastIndex(s, "```")
		if end > 3 {
			s = strings.TrimSpace(s[3:end])
			if nl := strings.Index(s, "\n"); nl >= 0 {
				s = strings.TrimSpace(s[nl:])
			}
		}
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				raw := []byte(s[start : i+1])
				if json.Valid(raw) {
					return raw, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("malformed JSON in response")
}
