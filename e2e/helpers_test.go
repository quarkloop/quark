//go:build e2e

package e2e

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func isRateLimit(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "rate-limited")
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	quarkRoot := filepath.Join(filepath.Dir(thisFile), "..")
	workspaceRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	loadDotEnv(filepath.Join(quarkRoot, ".env"))
	loadDotEnv(filepath.Join(workspaceRoot, ".env"))
}

type providerConfig struct {
	provider string
	model    string
	apiKey   string
}

func cfgForTest(t *testing.T, envKey string) (providerConfig, bool) {
	t.Helper()
	if key := os.Getenv(envKey); key != "" {
		m := firstEnv("OPENROUTER_E2E_MODEL", "OPENROUTER_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL")
		if m == "" {
			m = "qwen/qwen3.6-plus:free"
		}
		return providerConfig{"openrouter", m, key}, true
	}
	return providerConfig{}, false
}

func providerFromKey(envKey string) string {
	switch {
	case envKey == "ZHIPU_API_KEY":
		return "zhipu"
	default:
		return "openrouter"
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
