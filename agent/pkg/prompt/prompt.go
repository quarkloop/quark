// Package prompt provides the embedded system prompt for the agent.
package prompt

import _ "embed"

//go:embed systemprompt.md
var SystemPrompt string
