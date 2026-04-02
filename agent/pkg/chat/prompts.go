package chat

import (
	"fmt"
	"strings"

	"github.com/quarkloop/agent/pkg/agentcore"
	"github.com/quarkloop/agent/pkg/model"
)

// AskPrompt returns the system prompt for ask mode.
func AskPrompt(def *agentcore.Definition, parser model.ToolCallParser, tools []string) string {
	if len(tools) == 0 {
		return `You are a helpful assistant. Answer the user's question directly and concisely.

Rules:
- Do NOT produce execution plans or JSON plan structures.
- Answer directly with the information available to you.`
	}

	hint := model.FormatHintForTools(parser, tools)
	return fmt.Sprintf(`You are a helpful assistant. Answer the user's question directly and concisely in plain text.

%s

Rules:
- Do NOT produce execution plans or JSON plan structures.
- Do NOT wrap your response in JSON. Always respond in plain natural language.
- Use tools when needed to answer the question, then give a direct answer.
- If you can answer without tools, just answer directly in plain text.`, hint)
}

// PlanPrompt returns the system prompt for plan mode.
func PlanPrompt(def *agentcore.Definition, tools []string, agents []string, policy agentcore.ApprovalPolicy) string {
	approvalRules := `- Set "status" to "draft" when creating or modifying a plan.
- Set "status" to "approved" ONLY when the user explicitly approves the plan (e.g. "approved", "looks good", "go ahead").
- If the user provides the current plan state, use it as context to update or approve.`
	if policy == agentcore.ApprovalAuto {
		approvalRules = `- Always set "status" to "approved" — plans are auto-approved.`
	}

	return fmt.Sprintf(`You are a supervisor agent. Create a focused execution plan for the user's request.

Available subagent agents: %s
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
		strings.Join(tools, ", "),
		approvalRules)
}

// MasterPlanPrompt returns the system prompt for masterplan mode.
func MasterPlanPrompt(def *agentcore.Definition, agents []string, policy agentcore.ApprovalPolicy) string {
	approvalRules := `- Set "status" to "draft" when creating or modifying a masterplan.
- Set "status" to "approved" ONLY when the user explicitly approves (e.g. "approved", "looks good", "go ahead").`
	if policy == agentcore.ApprovalAuto {
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

// SupervisorPrompt returns the default system prompt for the supervisor cycle.
func SupervisorPrompt(def *agentcore.Definition, tools []string, agents []string) string {
	return fmt.Sprintf(`You are the supervisor agent orchestrating a multi-agent space.

Available subagent agents: %s
Available tools: %s

Your job is to:
1. Break the goal into concrete, parallelizable steps
2. Assign each step to the most appropriate agent
3. Track progress and adjust the plan as needed
4. Declare completion when the goal is fully achieved

Always respond with valid JSON as instructed.`,
		strings.Join(agents, ", "),
		strings.Join(tools, ", "))
}

// ClassificationPrompt returns the prompt for auto-mode classification.
func ClassificationPrompt(message string) string {
	return fmt.Sprintf(`Classify the following user request into exactly one of these modes:

- "ask" — a simple question that only needs a direct answer, no execution or planning required.
- "plan" — a task that needs a single execution plan with concrete steps to accomplish.
- "masterplan" — a large, multi-phase project that requires multiple plans to complete.

User request:
%s

Respond with ONLY the mode name (ask, plan, or masterplan). Nothing else.`, message)
}

// SystemPromptForMode returns the system prompt appropriate for the given mode.
func SystemPromptForMode(def *agentcore.Definition, res *agentcore.Resources, mode agentcore.Mode) string {
	tools := res.Dispatcher.List()
	agents := make([]string, 0)

	var base string
	switch mode {
	case agentcore.ModeAsk:
		base = AskPrompt(def, res.Gateway.Parser(), tools)
	case agentcore.ModePlan:
		base = PlanPrompt(def, tools, agents, def.Config.ApprovalPolicy)
	case agentcore.ModeMasterPlan:
		base = MasterPlanPrompt(def, agents, def.Config.ApprovalPolicy)
	default:
		base = AskPrompt(def, res.Gateway.Parser(), tools)
	}

	// Append active skills if resolver is available.
	if res.SkillResolver != nil {
		skills := res.SkillResolver.ResolveByTrigger(base)
		for _, s := range skills {
			base += fmt.Sprintf("\n\n## Skill: %s\n%s\n", s.Name, s.Content)
		}
	}

	return base
}
