package agent

import (
	"fmt"
	"strings"

	"github.com/quarkloop/agent/pkg/model"
)

// systemPromptForMode returns the system prompt appropriate for the given mode.
func (a *Agent) systemPromptForMode(mode Mode) string {
	switch mode {
	case ModeAsk:
		return a.buildAskSystemPrompt()
	case ModePlan:
		return a.buildPlanModeSystemPrompt()
	case ModeMasterPlan:
		return a.buildMasterPlanModeSystemPrompt()
	default:
		return a.buildSupervisorSystemPrompt(nil)
	}
}

func (a *Agent) buildAskSystemPrompt() string {
	tools := a.dispatcher.List()
	if len(tools) == 0 {
		return `You are a helpful assistant. Answer the user's question directly and concisely.

Rules:
- Do NOT produce execution plans or JSON plan structures.
- Answer directly with the information available to you.`
	}

	hint := model.FormatHintForTools(a.gateway.Parser(), tools)
	return fmt.Sprintf(`You are a helpful assistant. Answer the user's question directly and concisely.

%s

Rules:
- Do NOT produce execution plans or JSON plan structures.
- Use tools when needed to answer the question, then give a direct answer.
- If you can answer without tools, just answer directly.`, hint)
}

func (a *Agent) buildPlanModeSystemPrompt() string {
	agents := make([]string, 0, len(a.subAgents))
	for name := range a.subAgents {
		agents = append(agents, name)
	}

	approvalRules := `- Set "status" to "draft" when creating or modifying a plan.
- Set "status" to "approved" ONLY when the user explicitly approves the plan (e.g. "approved", "looks good", "go ahead").
- If the user provides the current plan state, use it as context to update or approve.`
	if a.def.Config.ApprovalPolicy == ApprovalAuto {
		approvalRules = `- Always set "status" to "approved" — plans are auto-approved.`
	}

	return fmt.Sprintf(`You are a supervisor agent. Create a focused execution plan for the user's request.

Available worker agents: %s
Available tools: %s

Respond with a JSON object using this structure:
{
  "goal": "<restate the goal concisely>",
  "status": "draft",
  "steps": [
    {
      "id": "<short-slug>",
      "agent": "<agent-name from available agents, or 'supervisor'>",
      "description": "<specific task for this agent>",
      "depends_on": ["<step-id>", ...]
    }
  ]
}

Rules:
- Each step must have a unique "id" (e.g. "research-1", "write-draft").
- "agent" must be one of the available agents or "supervisor".
- "depends_on" lists step IDs that must complete before this step can start.
- Keep steps focused and atomic.
- Maximise parallelism: only add dependencies where truly required.
%s
- Respond with ONLY the JSON object, no explanation.`,
		strings.Join(agents, ", "),
		strings.Join(a.dispatcher.List(), ", "),
		approvalRules)
}

func (a *Agent) buildMasterPlanModeSystemPrompt() string {
	approvalRules := `- Set "status" to "draft" when creating or modifying a masterplan.
- Set "status" to "approved" ONLY when the user explicitly approves (e.g. "approved", "looks good", "go ahead").`
	if a.def.Config.ApprovalPolicy == ApprovalAuto {
		approvalRules = `- Always set "status" to "approved" — masterplans are auto-approved.`
	}

	return fmt.Sprintf(`You are a supervisor agent creating a master plan — a high-level overview for a large task.

Break the task into sequential phases. Each phase will become its own detailed execution plan with concrete steps.

Respond with a JSON object using this structure:
{
  "goal": "<restate the goal concisely>",
  "vision": "<high-level description of the approach>",
  "status": "draft",
  "phases": [
    {
      "id": "<short-slug>",
      "description": "<what this phase accomplishes>",
      "depends_on": ["<phase-id>", ...]
    }
  ]
}

Rules:
- Each phase must have a unique "id" (e.g. "phase-setup", "phase-core").
- "depends_on" lists phase IDs that must complete before this phase can start.
- Phases should be meaningful milestones, not trivially small.
- Order phases logically — later phases build on earlier ones.
%s
- Respond with ONLY the JSON object, no explanation.`, approvalRules)
}

func (a *Agent) classificationPrompt(message string) string {
	return fmt.Sprintf(`Classify the following user request into exactly one of these modes:

- "ask" — a simple question that only needs a direct answer, no execution or planning required.
- "plan" — a task that needs a single execution plan with concrete steps to accomplish.
- "masterplan" — a large, multi-phase project that requires multiple plans to complete.

User request:
%s

Respond with ONLY the mode name (ask, plan, or masterplan). Nothing else.`, message)
}
