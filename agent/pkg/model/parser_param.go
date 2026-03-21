package model

// ParamAliases maps alternative parameter names to canonical names per tool.
// Different models use different names for the same parameter (e.g. "command"
// vs "cmd" for bash). This table normalises them before dispatch.
var ParamAliases = map[string]map[string]string{
	"bash": {
		"command": "cmd",
		"shell":   "cmd",
		"script":  "cmd",
	},
	"read": {
		"file_path": "path",
		"filename":  "path",
		"file":      "path",
	},
	"write": {
		"file_path": "path",
		"filename":  "path",
		"file":      "path",
	},
}

// NormalizeArgs rewrites known parameter aliases to their canonical names
// for the given tool. Modifies and returns the map.
func NormalizeArgs(toolName string, args map[string]any) map[string]any {
	aliases, ok := ParamAliases[toolName]
	if !ok {
		return args
	}
	for alias, canonical := range aliases {
		if val, exists := args[alias]; exists {
			if _, hasCanonical := args[canonical]; !hasCanonical {
				args[canonical] = val
				delete(args, alias)
			}
		}
	}
	return args
}
