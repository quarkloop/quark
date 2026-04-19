package quarkfile_test

import (
	"strings"
	"testing"

	"github.com/quarkloop/supervisor/pkg/quarkfile"
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

// Execution mode tests

func TestValidate_ValidExecutionModes(t *testing.T) {
	for _, mode := range []string{"", "autonomous", "assistive", "workflow"} {
		qf := base()
		qf.Execution.Mode = mode
		if err := quarkfile.Validate(t.TempDir(), qf); err != nil {
			t.Errorf("mode %q should be valid, got: %v", mode, err)
		}
	}
}

func TestValidate_InvalidExecutionMode(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "invalid-mode"
	assertInvalid(t, qf, "execution.mode")
}

func TestValidate_InvalidApprovalTimeout(t *testing.T) {
	qf := base()
	qf.Execution.ApprovalTimeout = "not-a-duration"
	assertInvalid(t, qf, "approval_timeout")
}

func TestValidate_ValidApprovalTimeout(t *testing.T) {
	qf := base()
	qf.Execution.ApprovalTimeout = "5m"
	if err := quarkfile.Validate(t.TempDir(), qf); err != nil {
		t.Fatalf("expected valid approval_timeout, got: %v", err)
	}
}

func TestValidate_DAGOnlyInWorkflowMode(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "autonomous"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "step-1", Name: "Step 1", Action: "do something"},
	}
	assertInvalid(t, qf, "only valid when mode is 'workflow'")
}

func TestValidate_ValidDAG(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "step-1", Name: "First", Action: "do first"},
		{ID: "step-2", Name: "Second", Action: "do second", DependsOn: []string{"step-1"}},
	}
	if err := quarkfile.Validate(t.TempDir(), qf); err != nil {
		t.Fatalf("expected valid DAG, got: %v", err)
	}
}

func TestValidate_DAGMissingID(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{Name: "Missing ID", Action: "do something"},
	}
	assertInvalid(t, qf, "missing required field 'id'")
}

func TestValidate_DAGDuplicateID(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "dup", Name: "First", Action: "do first"},
		{ID: "dup", Name: "Second", Action: "do second"},
	}
	assertInvalid(t, qf, "duplicate step id")
}

func TestValidate_DAGMissingName(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "step-1", Action: "do something"},
	}
	assertInvalid(t, qf, "missing required field 'name'")
}

func TestValidate_DAGMissingAction(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "step-1", Name: "Missing Action"},
	}
	assertInvalid(t, qf, "missing required field 'action'")
}

func TestValidate_DAGInvalidTimeout(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "step-1", Name: "Bad Timeout", Action: "do something", Timeout: "not-valid"},
	}
	assertInvalid(t, qf, "timeout is not a valid duration")
}

func TestValidate_DAGNegativeRetryCount(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "step-1", Name: "Negative Retry", Action: "do something", RetryCount: -1},
	}
	assertInvalid(t, qf, "retry_count must be >= 0")
}

func TestValidate_DAGUnknownDependency(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "step-1", Name: "Step", Action: "do something", DependsOn: []string{"nonexistent"}},
	}
	assertInvalid(t, qf, "depends_on references unknown step")
}

func TestValidate_DAGSelfDependency(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "step-1", Name: "Self Dep", Action: "do something", DependsOn: []string{"step-1"}},
	}
	assertInvalid(t, qf, "cannot depend on itself")
}

func TestValidate_DAGCircularDependency(t *testing.T) {
	qf := base()
	qf.Execution.Mode = "workflow"
	qf.Execution.DAG = []quarkfile.DAGStep{
		{ID: "a", Name: "A", Action: "do a", DependsOn: []string{"c"}},
		{ID: "b", Name: "B", Action: "do b", DependsOn: []string{"a"}},
		{ID: "c", Name: "C", Action: "do c", DependsOn: []string{"b"}},
	}
	assertInvalid(t, qf, "circular dependency")
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
