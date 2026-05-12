package indexer

import (
	"strings"
)

func EntityIDFromName(name string) string {
	id := strings.ToLower(strings.TrimSpace(name))
	id = strings.ReplaceAll(id, " ", "-")
	if id == "" {
		return ""
	}
	return id
}
