package message

import (
	"fmt"
	"strings"
	"time"
)

// PlanStepStatus describes the execution state of a single plan step.
type PlanStepStatus string

const (
	// PlanStepPending is the initial state — not yet started.
	PlanStepPending PlanStepStatus = "pending"
	// PlanStepInProgress means the step is currently executing.
	PlanStepInProgress PlanStepStatus = "in_progress"
	// PlanStepCompleted means the step finished successfully.
	PlanStepCompleted PlanStepStatus = "completed"
	// PlanStepFailed means the step encountered an unrecoverable error.
	PlanStepFailed PlanStepStatus = "failed"
	// PlanStepSkipped means the step was deliberately bypassed.
	PlanStepSkipped PlanStepStatus = "skipped"
)

// PlanStep is a single action in a structured multi-step execution plan.
type PlanStep struct {
	// Index is the zero-based position of this step.
	Index int `json:"index"`

	// Description is the human-readable step description.
	Description string `json:"description"`

	// ToolName names the tool this step will invoke (empty for reasoning steps).
	ToolName string `json:"tool_name,omitempty"`

	// Status tracks the lifecycle of this step.
	Status PlanStepStatus `json:"status"`

	// ErrorMessage records failure detail when Status is PlanStepFailed.
	ErrorMessage string `json:"error_message,omitempty"`

	// Notes records agent reasoning, observations, or free-form commentary
	// accumulated during step execution. Multiple notes are newline-separated.
	// R20: supports incremental annotation during multi-turn execution.
	Notes string `json:"notes,omitempty"`

	// DependsOn lists the zero-based indices of steps that must reach a
	// terminal state (Completed or Skipped) before this step may begin.
	// An empty slice means the step has no prerequisites.
	// R20: enables non-linear, DAG-shaped plans.
	DependsOn []int `json:"depends_on,omitempty"`

	// StartedAt is stamped when the step transitions to PlanStepInProgress.
	// Nil when the step has not yet started.
	// R20: enables latency tracking and SLA detection.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is stamped when the step reaches any terminal state
	// (Completed, Failed, or Skipped).
	// R20: combined with StartedAt gives exact step duration.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// IsCompleted reports whether the step finished successfully.
func (s PlanStep) IsCompleted() bool { return s.Status == PlanStepCompleted }

// IsDone reports whether the step reached a terminal state.
func (s PlanStep) IsDone() bool {
	return s.Status == PlanStepCompleted ||
		s.Status == PlanStepFailed ||
		s.Status == PlanStepSkipped
}

// Duration returns the wall-clock time between StartedAt and CompletedAt.
// Returns 0 when either timestamp is absent.
func (s PlanStep) Duration() time.Duration {
	if s.StartedAt == nil || s.CompletedAt == nil {
		return 0
	}
	return s.CompletedAt.Sub(*s.StartedAt)
}

// WithStatus returns a copy of s with Status set to status.
// Automatically stamps StartedAt when transitioning to InProgress (if unset)
// and CompletedAt when reaching any terminal state (if unset).
func (s PlanStep) WithStatus(status PlanStepStatus) PlanStep {
	c := s
	c.Status = status
	now := time.Now().UTC()
	if status == PlanStepInProgress && c.StartedAt == nil {
		c.StartedAt = &now
	}
	if (status == PlanStepCompleted || status == PlanStepFailed || status == PlanStepSkipped) &&
		c.CompletedAt == nil {
		t := now
		c.CompletedAt = &t
	}
	return c
}

// WithNote returns a copy of s with note appended to Notes.
// Multiple notes are separated by a newline.
func (s PlanStep) WithNote(note string) PlanStep {
	c := s
	if c.Notes == "" {
		c.Notes = note
	} else {
		c.Notes = c.Notes + "\n" + note
	}
	return c
}

// PlanPayload represents a structured agent execution plan.
// Used in orchestration patterns such as ReAct and plan-and-execute.
type PlanPayload struct {
	// Goal is the high-level objective the plan is designed to achieve.
	Goal string `json:"goal"`

	// Steps is the ordered list of actions.
	Steps []PlanStep `json:"steps"`

	// CreatedAt records when the plan was first created.
	// R20: enables elapsed-time calculations and audit trails.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is stamped whenever the plan is mutated via WithStepStatus.
	// Nil when the plan has not been modified since creation.
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

func init() { RegisterPayloadFactory(PlanType, func() Payload { return &PlanPayload{} }) }

func (p PlanPayload) Kind() MessageType { return PlanType }
func (p PlanPayload) sealPayload()      {}

// WithStepStatus returns a copy of the payload with the step at index
// updated to the given status and UpdatedAt stamped. Index is zero-based.
// No-ops (returns p unchanged) when index is out of range.
func (p PlanPayload) WithStepStatus(index int, status PlanStepStatus) PlanPayload {
	if index < 0 || index >= len(p.Steps) {
		return p
	}
	c := p
	steps := make([]PlanStep, len(p.Steps))
	copy(steps, p.Steps)
	steps[index] = steps[index].WithStatus(status)
	c.Steps = steps
	now := time.Now().UTC()
	c.UpdatedAt = &now
	return c
}

// ReadySteps returns the indices of steps that are Pending and whose every
// dependency (DependsOn) has reached a terminal state.
// Returns an empty slice when no steps are ready.
func (p PlanPayload) ReadySteps() []int {
	terminal := make(map[int]bool, len(p.Steps))
	for _, s := range p.Steps {
		if s.IsDone() {
			terminal[s.Index] = true
		}
	}
	var ready []int
	for _, s := range p.Steps {
		if s.Status != PlanStepPending {
			continue
		}
		allMet := true
		for _, dep := range s.DependsOn {
			if !terminal[dep] {
				allMet = false
				break
			}
		}
		if allMet {
			ready = append(ready, s.Index)
		}
	}
	return ready
}

func (p PlanPayload) TextRepresentation() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[plan goal=%q]\n", p.Goal)
	for _, s := range p.Steps {
		deps := ""
		if len(s.DependsOn) > 0 {
			parts := make([]string, len(s.DependsOn))
			for i, d := range s.DependsOn {
				parts[i] = fmt.Sprintf("%d", d+1)
			}
			deps = fmt.Sprintf(" (after: %s)", strings.Join(parts, ","))
		}
		dur := ""
		if d := s.Duration(); d > 0 {
			dur = fmt.Sprintf(" [%s]", d.Round(time.Millisecond))
		}
		fmt.Fprintf(&sb, "  %s %d. %s%s%s\n", stepIcon(s.Status), s.Index+1, s.Description, deps, dur)
		if s.Notes != "" {
			fmt.Fprintf(&sb, "       note: %s\n", s.Notes)
		}
		if s.ErrorMessage != "" {
			fmt.Fprintf(&sb, "       error: %s\n", s.ErrorMessage)
		}
	}
	return sb.String()
}

// LLMText renders the plan for the model context.
func (p PlanPayload) LLMText() string { return p.TextRepresentation() }

// UserText renders a friendly plan summary.
func (p PlanPayload) UserText() string {
	done := 0
	for _, s := range p.Steps {
		if s.IsCompleted() {
			done++
		}
	}
	return fmt.Sprintf("📋 %s (%d/%d steps done)", p.Goal, done, len(p.Steps))
}

// DevText returns the full plan with status, timing, dependencies, and notes.
func (p PlanPayload) DevText() string { return p.TextRepresentation() }

func stepIcon(s PlanStepStatus) string {
	switch s {
	case PlanStepCompleted:
		return "[✓]"
	case PlanStepInProgress:
		return "[→]"
	case PlanStepFailed:
		return "[✗]"
	case PlanStepSkipped:
		return "[-]"
	default:
		return "[ ]"
	}
}
