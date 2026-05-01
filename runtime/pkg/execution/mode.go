// Package execution provides execution mode configuration and runtime behavior.
// It integrates the loop, approval gates, and DAG execution based on mode.
package execution

import (
	"time"
)

// Mode represents the agent's execution mode.
type Mode string

const (
	// ModeAutonomous is the default mode where the agent operates without human approval.
	ModeAutonomous Mode = "autonomous"

	// ModeAssistive is HITL mode where tool calls require human approval.
	ModeAssistive Mode = "assistive"

	// ModeWorkflow is DAG mode where the agent follows a predefined workflow.
	ModeWorkflow Mode = "workflow"
)

// ParseMode converts a string to a Mode, defaulting to autonomous.
func ParseMode(s string) Mode {
	switch s {
	case "assistive":
		return ModeAssistive
	case "workflow":
		return ModeWorkflow
	default:
		return ModeAutonomous
	}
}

// String returns the string representation of the mode.
func (m Mode) String() string {
	return string(m)
}

// Config holds execution mode configuration.
type Config struct {
	// Mode is the execution mode.
	Mode Mode

	// ApprovalTimeout is the timeout for approval requests in assistive mode.
	ApprovalTimeout time.Duration

	// DAGConfig is the workflow configuration for workflow mode.
	DAGConfig *DAGConfig
}

// DAGConfig holds workflow DAG configuration.
type DAGConfig struct {
	// Steps is the list of workflow steps.
	Steps []DAGStepConfig

	// MaxParallel is the maximum number of parallel steps. 0 = unlimited.
	MaxParallel int

	// DefaultTimeout is the default step timeout.
	DefaultTimeout time.Duration
}

// DAGStepConfig is the configuration for a single DAG step.
type DAGStepConfig struct {
	ID         string
	Name       string
	Action     string
	DependsOn  []string
	Timeout    time.Duration
	RetryCount int
}

// DefaultConfig returns a default autonomous configuration.
func DefaultConfig() Config {
	return Config{
		Mode:            ModeAutonomous,
		ApprovalTimeout: 24 * time.Hour,
	}
}
