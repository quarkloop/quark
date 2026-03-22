package agentcore

import (
	"fmt"
	"strings"
)

// Mode defines how the agent processes a user request.
type Mode string

const (
	// ModeAsk is read-only: single LLM call, no plans, no tools.
	ModeAsk Mode = "ask"
	// ModePlan creates a single execution plan for the requested task.
	ModePlan Mode = "plan"
	// ModeMasterPlan creates a broader master plan with multiple phases,
	// each phase becoming its own execution plan.
	ModeMasterPlan Mode = "masterplan"
	// ModeAuto uses the LLM to classify the request and route to the
	// appropriate mode automatically.
	ModeAuto Mode = "auto"
)

// ValidMode returns true if m is a recognised working mode.
func ValidMode(m string) bool {
	switch Mode(strings.ToLower(strings.TrimSpace(m))) {
	case ModeAsk, ModePlan, ModeMasterPlan, ModeAuto:
		return true
	}
	return false
}

// ParseMode normalises and validates a mode string.
// Returns ModeAuto when s is empty.
func ParseMode(s string) (Mode, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ModeAuto, nil
	}
	m := Mode(s)
	switch m {
	case ModeAsk, ModePlan, ModeMasterPlan, ModeAuto:
		return m, nil
	}
	return "", fmt.Errorf("invalid mode %q: must be ask, plan, masterplan, or auto", s)
}
