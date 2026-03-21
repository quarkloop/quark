package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/quarkloop/agent/pkg/activity"
)

// chatAuto handles a chat request in auto mode. It uses a lightweight LLM
// call to classify the request as ask/plan/masterplan, then routes to the
// resolved mode's handler. The classification call is never streamed; the
// resolved handler inherits the streaming flag from the original request.
func (a *Agent) chatAuto(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	resolved, err := a.classifyMode(ctx, req.Message)
	if err != nil {
		log.Printf("agent: auto classification failed: %v — falling back to plan", err)
		resolved = ModePlan
	}

	log.Printf("agent: auto classified as %s", resolved)
	a.emit(activity.ModeClassified, map[string]string{"resolved": string(resolved)})
	a.SetMode(resolved)
	a.updateSystemPrompt(resolved)

	switch resolved {
	case ModeAsk:
		return a.chatAsk(ctx, req)
	case ModePlan:
		return a.chatPlan(ctx, req)
	case ModeMasterPlan:
		return a.chatMasterPlan(ctx, req)
	default:
		return a.chatPlan(ctx, req)
	}
}

// classifyMode performs a single LLM call to determine the appropriate
// working mode for the given message.
func (a *Agent) classifyMode(ctx context.Context, message string) (Mode, error) {
	prompt := a.classificationPrompt(message)
	resp, err := a.inferWithContext(ctx, a.ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("classify: %w", err)
	}
	return ParseMode(strings.TrimSpace(resp.Content))
}
