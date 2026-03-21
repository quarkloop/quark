package quarkfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quarkloop/tools/space/pkg/quarkfile"
)

// base returns a minimal valid Quarkfile.
func base() *quarkfile.Quarkfile {
	return &quarkfile.Quarkfile{
		Quark: "1.0",
		Meta:  quarkfile.Meta{Name: "test-space"},
		Model: quarkfile.Model{Provider: "anthropic", Name: "claude-opus-4-6"},
		Supervisor: quarkfile.Supervisor{
			Agent: "quark/supervisor@latest",
		},
	}
}

func TestValidate_ValidMinimal(t *testing.T) {
	if err := quarkfile.Validate(t.TempDir(), base()); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidate_MissingQuarkVersion(t *testing.T) {
	qf := base()
	qf.Quark = ""
	assertInvalid(t, qf, "quark version")
}

func TestValidate_MissingMetaName(t *testing.T) {
	qf := base()
	qf.Meta.Name = ""
	assertInvalid(t, qf, "meta.name")
}

func TestValidate_MissingModelProvider(t *testing.T) {
	qf := base()
	qf.Model.Provider = ""
	assertInvalid(t, qf, "model.provider")
}

func TestValidate_MissingModelName(t *testing.T) {
	qf := base()
	qf.Model.Name = ""
	assertInvalid(t, qf, "model.name")
}

func TestValidate_InvalidModelProvider(t *testing.T) {
	qf := base()
	qf.Model.Provider = "made-up-provider"
	assertInvalid(t, qf, "provider")
}

func TestValidate_ValidProviders(t *testing.T) {
	for _, p := range []string{"anthropic", "openai", "zhipu", "noop"} {
		qf := base()
		qf.Model.Provider = p
		if err := quarkfile.Validate(t.TempDir(), qf); err != nil {
			t.Errorf("provider %q should be valid, got: %v", p, err)
		}
	}
}

func TestValidate_MissingSupervisorAgent(t *testing.T) {
	qf := base()
	qf.Supervisor.Agent = ""
	assertInvalid(t, qf, "supervisor.agent")
}

func TestValidate_SupervisorPromptMissing(t *testing.T) {
	qf := base()
	qf.Supervisor.Prompt = "./prompts/no-such-file.txt"
	assertInvalidDir(t, t.TempDir(), qf, "not found")
}

func TestValidate_SupervisorPromptExists(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "prompts"), 0755)
	os.WriteFile(filepath.Join(dir, "prompts", "supervisor.txt"), []byte("hello"), 0644)
	qf := base()
	qf.Supervisor.Prompt = "./prompts/supervisor.txt"
	if err := quarkfile.Validate(dir, qf); err != nil {
		t.Fatalf("expected valid with existing prompt, got: %v", err)
	}
}

func TestValidate_AgentMissingRef(t *testing.T) {
	qf := base()
	qf.Agents = []quarkfile.Agent{{Name: "researcher", Ref: ""}}
	assertInvalid(t, qf, "missing ref")
}

func TestValidate_AgentMissingName(t *testing.T) {
	qf := base()
	qf.Agents = []quarkfile.Agent{{Name: "", Ref: "quark/researcher@latest"}}
	assertInvalid(t, qf, "missing name")
}

func TestValidate_AgentPromptMissing(t *testing.T) {
	qf := base()
	qf.Agents = []quarkfile.Agent{
		{Name: "researcher", Ref: "quark/researcher@latest", Prompt: "./prompts/researcher.txt"},
	}
	assertInvalidDir(t, t.TempDir(), qf, "not found")
}

func TestValidate_ToolMissingRef(t *testing.T) {
	qf := base()
	qf.Tools = []quarkfile.Tool{{Name: "search", Ref: ""}}
	assertInvalid(t, qf, "missing ref")
}

func TestValidate_ToolMissingName(t *testing.T) {
	qf := base()
	qf.Tools = []quarkfile.Tool{{Name: "", Ref: "quark/web-search@latest"}}
	assertInvalid(t, qf, "missing name")
}

func TestValidate_InvalidRestartPolicy(t *testing.T) {
	qf := base()
	qf.Restart = "whenever-i-feel-like-it"
	assertInvalid(t, qf, "restart")
}

func TestValidate_ValidRestartPolicies(t *testing.T) {
	for _, policy := range []string{"on-failure", "always", "never", ""} {
		qf := base()
		qf.Restart = policy
		if err := quarkfile.Validate(t.TempDir(), qf); err != nil {
			t.Errorf("policy %q should be valid, got: %v", policy, err)
		}
	}
}

// helpers

func assertInvalid(t *testing.T, qf *quarkfile.Quarkfile, wantSubstring string) {
	t.Helper()
	assertInvalidDir(t, t.TempDir(), qf, wantSubstring)
}

func assertInvalidDir(t *testing.T, dir string, qf *quarkfile.Quarkfile, wantSubstring string) {
	t.Helper()
	err := quarkfile.Validate(dir, qf)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Errorf("expected error containing %q, got: %v", wantSubstring, err)
	}
}
