package quarkfile_test

import (
	"strings"
	"testing"

	"github.com/quarkloop/cli/pkg/quarkfile"
)

// base returns a minimal valid Quarkfile.
func base() *quarkfile.Quarkfile {
	return &quarkfile.Quarkfile{
		Quark:   "1.0",
		Meta:    quarkfile.Meta{Name: "test-space"},
		Plugins: []quarkfile.PluginRef{{Ref: "quark/tool-bash"}},
	}
}

// baseWithModel returns a valid Quarkfile with model section.
func baseWithModel() *quarkfile.Quarkfile {
	qf := base()
	qf.Model = quarkfile.Model{Provider: "anthropic", Name: "claude-sonnet-4.6"}
	return qf
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

func TestValidate_MissingPlugins(t *testing.T) {
	qf := base()
	qf.Plugins = nil
	assertInvalid(t, qf, "plugins")
}

func TestValidate_EmptyPluginRef(t *testing.T) {
	qf := base()
	qf.Plugins = []quarkfile.PluginRef{{Ref: ""}}
	assertInvalid(t, qf, "missing ref")
}

func TestValidate_MissingModelProvider(t *testing.T) {
	qf := baseWithModel()
	qf.Model.Provider = ""
	assertInvalid(t, qf, "model")
}

func TestValidate_MissingModelName(t *testing.T) {
	qf := baseWithModel()
	qf.Model.Name = ""
	assertInvalid(t, qf, "model")
}

func TestValidate_InvalidModelProvider(t *testing.T) {
	qf := baseWithModel()
	qf.Model.Provider = "made-up-provider"
	assertInvalid(t, qf, "provider")
}

func TestValidate_ValidProviders(t *testing.T) {
	for _, p := range []string{"anthropic", "openai", "zhipu", "noop"} {
		qf := baseWithModel()
		qf.Model.Provider = p
		if err := quarkfile.Validate(t.TempDir(), qf); err != nil {
			t.Errorf("provider %q should be valid, got: %v", p, err)
		}
	}
}

func TestValidate_NoModelSection(t *testing.T) {
	// Model is optional — a Quarkfile without model should validate.
	qf := base()
	if err := quarkfile.Validate(t.TempDir(), qf); err != nil {
		t.Fatalf("expected valid without model section, got: %v", err)
	}
}

func TestValidate_GatewayBudgetNegative(t *testing.T) {
	qf := base()
	qf.Gateway.TokenBudgetPerHour = -100
	assertInvalid(t, qf, "token_budget_per_hour")
}

func TestValidate_InvalidRoutingRegex(t *testing.T) {
	qf := base()
	qf.Routing.Rules = []quarkfile.RoutingRuleEntry{
		{Match: "[invalid", Provider: "openai", Model: "gpt-5"},
	}
	assertInvalid(t, qf, "regex")
}

func TestValidate_RoutingRuleMissingFields(t *testing.T) {
	qf := base()
	qf.Routing.Rules = []quarkfile.RoutingRuleEntry{
		{Match: "code_.*"},
	}
	assertInvalid(t, qf, "provider or model")
}

func TestValidate_ValidRoutingRules(t *testing.T) {
	qf := base()
	qf.Routing.Rules = []quarkfile.RoutingRuleEntry{
		{Match: "code_.*", Provider: "anthropic", Model: "claude-sonnet-4.6"},
	}
	if err := quarkfile.Validate(t.TempDir(), qf); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidate_NegativeRetentionPolicy(t *testing.T) {
	qf := base()
	qf.Permissions.Audit.RetentionDays = -5
	assertInvalid(t, qf, "retention_days")
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
