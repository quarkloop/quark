package hooks

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/quarkloop/agent/pkg/eventbus"
)

// ToolPermissions controls which tools the agent can invoke.
type ToolPermissions struct {
	Allowed []string
	Denied  []string
}

// FilesystemPermissions controls which paths the agent can access.
type FilesystemPermissions struct {
	AllowedPaths []string
	ReadOnly     []string
}

// PermissionGate blocks tool calls that aren't in the allowed list or are in the denied list.
func PermissionGate(perms ToolPermissions) *Hook {
	return &Hook{
		Name:     "permission_gate",
		Type:     TypeGate,
		Point:    BeforeToolCall,
		Priority: 100,
		Fn: func(ctx context.Context, point HookPoint, payload interface{}) HookResult {
			tp, ok := payload.(*ToolCallPayload)
			if !ok {
				return HookResult{Decision: Pass}
			}

			// Check deny list first.
			for _, denied := range perms.Denied {
				if strings.EqualFold(tp.ToolName, denied) || matchesPattern(tp.ToolName, denied) {
					return HookResult{Decision: Block, Payload: fmt.Sprintf("tool %q is denied", tp.ToolName)}
				}
			}

			// If allow list is non-empty, tool must be in it.
			if len(perms.Allowed) > 0 {
				found := false
				for _, allowed := range perms.Allowed {
					if strings.EqualFold(tp.ToolName, allowed) || matchesPattern(tp.ToolName, allowed) {
						found = true
						break
					}
				}
				if !found {
					return HookResult{Decision: Block, Payload: fmt.Sprintf("tool %q is not allowed", tp.ToolName)}
				}
			}

			return HookResult{Decision: Pass}
		},
	}
}

// FilesystemGate checks file paths against allowed_paths and read_only constraints.
func FilesystemGate(perms FilesystemPermissions) *Hook {
	return &Hook{
		Name:     "filesystem_gate",
		Type:     TypeGate,
		Point:    BeforeToolCall,
		Priority: 200,
		Fn: func(ctx context.Context, point HookPoint, payload interface{}) HookResult {
			tp, ok := payload.(*ToolCallPayload)
			if !ok {
				return HookResult{Decision: Pass}
			}

			// Only check read/write/bash tools that reference file paths.
			if tp.ToolName != "read" && tp.ToolName != "write" && tp.ToolName != "bash" {
				return HookResult{Decision: Pass}
			}

			path := extractFilePath(tp.Arguments)
			if path == "" {
				return HookResult{Decision: Pass}
			}

			// Check allowed paths.
			if len(perms.AllowedPaths) > 0 {
				allowed := false
				for _, ap := range perms.AllowedPaths {
					if isPathUnder(path, ap) {
						allowed = true
						break
					}
				}
				if !allowed {
					return HookResult{Decision: Block, Payload: fmt.Sprintf("path %q is outside allowed paths", path)}
				}
			}

			// Check read-only paths.
			if tp.ToolName == "write" && len(perms.ReadOnly) > 0 {
				for _, rp := range perms.ReadOnly {
					if isPathUnder(path, rp) {
						return HookResult{Decision: Block, Payload: fmt.Sprintf("path %q is read-only", path)}
					}
				}
			}

			return HookResult{Decision: Pass}
		},
	}
}

// SecretRedactor scans tool output for API keys and tokens, redacting them.
func SecretRedactor() *Hook {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
		regexp.MustCompile(`xai-[A-Za-z0-9_-]{20,}`),
		regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`),
		regexp.MustCompile(`gho_[A-Za-z0-9]{36,}`),
		regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`),
	}

	return &Hook{
		Name:     "secret_redactor",
		Type:     TypeModifier,
		Point:    AfterToolCall,
		Priority: 500,
		Fn: func(ctx context.Context, point HookPoint, payload interface{}) HookResult {
			trp, ok := payload.(*ToolResultPayload)
			if !ok {
				return HookResult{Decision: Pass}
			}

			redacted := trp.Content
			for _, re := range patterns {
				redacted = re.ReplaceAllString(redacted, "[REDACTED]")
			}

			if redacted != trp.Content {
				trp.Content = redacted
				return HookResult{Decision: Shape, Payload: trp}
			}
			return HookResult{Decision: Pass}
		},
	}
}

// AuditObserver emits events to EventBus for every tool invocation.
func AuditObserver(bus *eventbus.Bus) *Hook {
	return &Hook{
		Name:     "audit_observer",
		Type:     TypeObserver,
		Point:    BeforeToolCall,
		Priority: 10,
		Fn: func(ctx context.Context, point HookPoint, payload interface{}) HookResult {
			tp, ok := payload.(*ToolCallPayload)
			if !ok {
				return HookResult{Decision: Pass}
			}
			bus.Emit(eventbus.Event{
				Kind:      eventbus.KindToolCalled,
				SessionID: tp.SessionID,
				Data: map[string]string{
					"tool":  tp.ToolName,
					"step":  tp.StepID,
					"agent": "worker",
				},
			})
			return HookResult{Decision: Pass}
		},
	}
}

// ToolResultObserver emits completion events to EventBus.
func ToolResultObserver(bus *eventbus.Bus) *Hook {
	return &Hook{
		Name:     "tool_result_observer",
		Type:     TypeObserver,
		Point:    AfterToolCall,
		Priority: 10,
		Fn: func(ctx context.Context, point HookPoint, payload interface{}) HookResult {
			trp, ok := payload.(*ToolResultPayload)
			if !ok {
				return HookResult{Decision: Pass}
			}
			bus.Emit(eventbus.Event{
				Kind:      eventbus.KindToolCompleted,
				SessionID: trp.StepID,
				Data: map[string]string{
					"tool":    trp.ToolName,
					"step":    trp.StepID,
					"isError": fmt.Sprintf("%v", trp.IsError),
				},
			})
			return HookResult{Decision: Pass}
		},
	}
}

// RegisterBuiltInHooks registers all default security and audit hooks.
func RegisterBuiltInHooks(reg *Registry, perms ToolPermissions, fsPerms FilesystemPermissions, bus *eventbus.Bus) {
	reg.Register(PermissionGate(perms))
	reg.Register(FilesystemGate(fsPerms))
	reg.Register(SecretRedactor())
	if bus != nil {
		reg.Register(AuditObserver(bus))
		reg.Register(ToolResultObserver(bus))
	}
}

// Helpers

func matchesPattern(name, pattern string) bool {
	matched, _ := filepath.Match(pattern, name)
	return matched
}

func isPathUnder(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func extractFilePath(args map[string]interface{}) string {
	for _, key := range []string{"path", "file", "filepath", "target"} {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	// Check bash command for file references.
	if cmd, ok := args["command"].(string); ok {
		parts := strings.Fields(cmd)
		for _, p := range parts {
			if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "./") {
				return strings.Trim(p, "\"'")
			}
		}
	}
	return ""
}
