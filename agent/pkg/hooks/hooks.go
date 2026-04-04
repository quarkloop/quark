// Package hooks provides an extensible interception system for agent actions.
//
// Three hook types fire at defined points in the agent loop:
//   - Observer: fire-and-forget, 500ms timeout, cannot modify
//   - Modifier: can mutate payloads, 5s timeout
//   - Gate: approve/deny actions, 60s timeout
//
// Hook decisions: pass, shape (modifier only), block, divert, halt, collapse.
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

// HookPoint identifies where in the agent loop a hook fires.
type HookPoint string

const (
	BeforeToolCall  HookPoint = "before_tool_call"
	AfterToolCall   HookPoint = "after_tool_call"
	BeforeInference HookPoint = "before_inference"
	AfterInference  HookPoint = "after_inference"
	BeforePlanExec  HookPoint = "before_plan_exec"
	AfterPlanExec   HookPoint = "after_plan_exec"
)

// HookType classifies the hook's behavior.
type HookType string

const (
	TypeObserver HookType = "observer"
	TypeModifier HookType = "modifier"
	TypeGate     HookType = "gate"
)

// Decision is the hook's verdict.
type Decision string

const (
	Pass     Decision = "pass"
	Shape    Decision = "shape"    // modifier only — payload was mutated
	Block    Decision = "block"    // deny this specific action
	Divert   Decision = "divert"   // redirect to different path
	Halt     Decision = "halt"     // stop the current turn
	Collapse Decision = "collapse" // hard abort, tear down everything
)

// HookResult is returned by a hook invocation.
type HookResult struct {
	Decision Decision
	Payload  any // mutated payload for Shape; reason string for Block/Halt
}

// HookFunc is the signature for in-process hooks.
type HookFunc func(ctx context.Context, point HookPoint, payload any) HookResult

// Hook is a single registered hook.
type Hook struct {
	Name     string
	Type     HookType
	Point    HookPoint
	Priority int // lower = runs first
	Timeout  time.Duration
	Fn       HookFunc // in-process hook
	Endpoint string   // out-of-process hook (localhost HTTP)
}

// Registry manages registered hooks and executes them at hook points.
type Registry struct {
	mu    sync.RWMutex
	hooks map[HookPoint][]*Hook
}

// New creates an empty hook registry.
func New() *Registry {
	return &Registry{
		hooks: make(map[HookPoint][]*Hook),
	}
}

// Register adds a hook to the registry. Hooks are sorted by priority on insert.
func (r *Registry) Register(h *Hook) {
	if h.Timeout == 0 {
		switch h.Type {
		case TypeObserver:
			h.Timeout = 500 * time.Millisecond
		case TypeModifier:
			h.Timeout = 5 * time.Second
		case TypeGate:
			h.Timeout = 60 * time.Second
		}
	}

	r.mu.Lock()
	r.hooks[h.Point] = append(r.hooks[h.Point], h)
	sort.Slice(r.hooks[h.Point], func(i, j int) bool {
		return r.hooks[h.Point][i].Priority < r.hooks[h.Point][j].Priority
	})
	r.mu.Unlock()
}

// Execute runs all hooks for a given point. Returns the (possibly mutated)
// payload, the most restrictive decision, and any error.
//
// Execution order: Observers (fire-and-forget) → Modifiers (sequential) → Gates (sequential).
// First Block/Halt/Collapse from any hook stops further execution.
func (r *Registry) Execute(ctx context.Context, point HookPoint, payload any) (any, Decision, error) {
	r.mu.RLock()
	hs := r.hooks[point]
	r.mu.RUnlock()

	if len(hs) == 0 {
		return payload, Pass, nil
	}

	currentPayload := payload

	// Phase 1: Observers — fire-and-forget
	for _, h := range hs {
		if h.Type != TypeObserver {
			continue
		}
		go func(hk *Hook) {
			tctx, cancel := context.WithTimeout(ctx, hk.Timeout)
			defer cancel()
			if hk.Fn != nil {
				hk.Fn(tctx, point, currentPayload)
			} else if hk.Endpoint != "" {
				callRemoteHook(tctx, hk.Endpoint, point, currentPayload)
			}
		}(h)
	}

	// Phase 2: Modifiers — sequential, can mutate payload
	for _, h := range hs {
		if h.Type != TypeModifier {
			continue
		}
		result, err := invokeHook(ctx, h, point, currentPayload)
		if err != nil {
			continue // skip failing hooks
		}
		switch result.Decision {
		case Shape:
			if result.Payload != nil {
				currentPayload = result.Payload
			}
		case Block, Halt, Collapse:
			return currentPayload, result.Decision, nil
		}
	}

	// Phase 3: Gates — sequential, approve/deny
	for _, h := range hs {
		if h.Type != TypeGate {
			continue
		}
		result, err := invokeHook(ctx, h, point, currentPayload)
		if err != nil {
			continue
		}
		switch result.Decision {
		case Block, Halt, Collapse:
			return currentPayload, result.Decision, nil
		}
	}

	return currentPayload, Pass, nil
}

func invokeHook(ctx context.Context, h *Hook, point HookPoint, payload any) (HookResult, error) {
	tctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()

	if h.Fn != nil {
		return h.Fn(tctx, point, payload), nil
	}
	if h.Endpoint != "" {
		return callRemoteHook(tctx, h.Endpoint, point, payload)
	}
	return HookResult{Decision: Pass}, nil
}

func callRemoteHook(ctx context.Context, endpoint string, point HookPoint, payload any) (HookResult, error) {
	body, err := json.Marshal(map[string]any{
		"point":   string(point),
		"payload": payload,
	})
	if err != nil {
		return HookResult{Decision: Pass}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return HookResult{Decision: Pass}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return HookResult{Decision: Pass}, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return HookResult{Decision: Pass}, err
	}

	var result HookResult
	if err := json.Unmarshal(data, &result); err != nil {
		return HookResult{Decision: Pass}, fmt.Errorf("decode hook result: %w", err)
	}
	return result, nil
}
