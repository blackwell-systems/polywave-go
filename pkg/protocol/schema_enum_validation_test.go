package protocol

import (
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func TestValidateAllEnums_Valid(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "b.go", Agent: "B", Wave: 1, Action: "modify"},
			{File: "c.go", Agent: "C", Wave: 1, Action: "delete"},
		},
		QualityGates: &QualityGates{Level: "standard"},
		Scaffolds: []ScaffoldFile{
			{FilePath: "types.go", Status: "pending"},
			{FilePath: "errors.go", Status: "committed"},
		},
		PreMortem: &PreMortem{
			OverallRisk: "medium",
			Rows: []PreMortemRow{
				{Scenario: "test", Likelihood: "low", Impact: "high", Mitigation: "fix"},
				{Scenario: "test2", Likelihood: "medium", Impact: "medium", Mitigation: "fix2"},
			},
		},
	}

	errs := validateAllEnums(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateAllEnums_InvalidAction(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1, Action: "create"},
		},
	}

	errs := validateAllEnums(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeInvalidEnum {
		t.Errorf("expected code %s, got %s", result.CodeInvalidEnum, errs[0].Code)
	}
	if errs[0].Field != "file_ownership[0].action" {
		t.Errorf("expected field file_ownership[0].action, got %s", errs[0].Field)
	}
}

func TestValidateAllEnums_EmptyAction(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1, Action: ""},
		},
	}

	errs := validateAllEnums(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty action, got %d: %v", len(errs), errs)
	}
}

func TestValidateAllEnums_InvalidLevel(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: &QualityGates{Level: "extreme"},
	}

	errs := validateAllEnums(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeInvalidEnum {
		t.Errorf("expected code %s, got %s", result.CodeInvalidEnum, errs[0].Code)
	}
	if errs[0].Field != "quality_gates.level" {
		t.Errorf("expected field quality_gates.level, got %s", errs[0].Field)
	}
}

func TestValidateAllEnums_EmptyLevel(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: &QualityGates{Level: ""},
	}

	errs := validateAllEnums(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty level, got %d: %v", len(errs), errs)
	}
}

func TestValidateAllEnums_InvalidScaffoldStatus(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "types.go", Status: "done"},
		},
	}

	errs := validateAllEnums(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeInvalidEnum {
		t.Errorf("expected code %s, got %s", result.CodeInvalidEnum, errs[0].Code)
	}
	if errs[0].Field != "scaffolds[0].status" {
		t.Errorf("expected field scaffolds[0].status, got %s", errs[0].Field)
	}
}

func TestValidateAllEnums_ValidScaffoldCommitted(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "types.go", Status: "committed (abc123)"},
			{FilePath: "errors.go", Status: "committed"},
		},
	}

	errs := validateAllEnums(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for committed prefix, got %d: %v", len(errs), errs)
	}
}

func TestValidateAllEnums_InvalidLikelihood(t *testing.T) {
	m := &IMPLManifest{
		PreMortem: &PreMortem{
			Rows: []PreMortemRow{
				{Scenario: "test", Likelihood: "extreme", Impact: "low", Mitigation: "fix"},
			},
		},
	}

	errs := validateAllEnums(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeInvalidEnum {
		t.Errorf("expected code %s, got %s", result.CodeInvalidEnum, errs[0].Code)
	}
	if errs[0].Field != "pre_mortem.rows[0].likelihood" {
		t.Errorf("expected field pre_mortem.rows[0].likelihood, got %s", errs[0].Field)
	}
}

func TestValidateAllEnums_InvalidImpact(t *testing.T) {
	m := &IMPLManifest{
		PreMortem: &PreMortem{
			Rows: []PreMortemRow{
				{Scenario: "test", Likelihood: "low", Impact: "critical", Mitigation: "fix"},
			},
		},
	}

	errs := validateAllEnums(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeInvalidEnum {
		t.Errorf("expected code %s, got %s", result.CodeInvalidEnum, errs[0].Code)
	}
	if errs[0].Field != "pre_mortem.rows[0].impact" {
		t.Errorf("expected field pre_mortem.rows[0].impact, got %s", errs[0].Field)
	}
}

func TestValidateAllEnums_NilOptionals(t *testing.T) {
	m := &IMPLManifest{
		QualityGates: nil,
		PreMortem:    nil,
	}

	errs := validateAllEnums(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for nil optionals, got %d: %v", len(errs), errs)
	}
}

// --- InjectionMethod tests ---

func TestValidateInjectionMethod_valid(t *testing.T) {
	for _, v := range []InjectionMethod{InjectionMethodHook, InjectionMethodManualFallback, InjectionMethodUnknown} {
		m := &IMPLManifest{InjectionMethod: v, State: StateWaveExecuting}
		errs := validateInjectionMethod(m)
		if len(errs) != 0 {
			t.Errorf("expected no errors for %q, got %d: %v", v, len(errs), errs)
		}
	}
}

func TestValidateInjectionMethod_invalid(t *testing.T) {
	m := &IMPLManifest{InjectionMethod: "bad-value", State: StateWaveExecuting}
	errs := validateInjectionMethod(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Severity != "error" {
		t.Errorf("expected error severity, got %s", errs[0].Severity)
	}
	if errs[0].Code != result.CodeInvalidEnum {
		t.Errorf("expected code %s, got %s", result.CodeInvalidEnum, errs[0].Code)
	}
	if errs[0].Field != "injection_method" {
		t.Errorf("expected field injection_method, got %s", errs[0].Field)
	}
}

func TestValidateInjectionMethod_absent_active(t *testing.T) {
	// Absent injection_method is always skipped (backwards compatible; no absent-field warning yet).
	m := &IMPLManifest{InjectionMethod: "", State: StateWaveExecuting}
	errs := validateInjectionMethod(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for absent injection_method in active state, got %d: %v", len(errs), errs)
	}
}

func TestValidateInjectionMethod_absent_complete(t *testing.T) {
	m := &IMPLManifest{InjectionMethod: "", State: StateComplete}
	errs := validateInjectionMethod(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for COMPLETE state, got %d: %v", len(errs), errs)
	}
}

func TestValidateInjectionMethod_absent_empty_state(t *testing.T) {
	m := &IMPLManifest{InjectionMethod: "", State: ""}
	errs := validateInjectionMethod(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty state, got %d: %v", len(errs), errs)
	}
}

// --- ContextSource tests ---

func TestValidateAgentContextSource_valid(t *testing.T) {
	for _, v := range []ContextSource{ContextSourcePreparedBrief, ContextSourceFallbackFullContext, ContextSourceCrossRepoFull} {
		m := &IMPLManifest{
			State: StateWaveExecuting,
			Waves: []Wave{
				{Number: 1, Agents: []Agent{{ID: "A", Task: "t", Files: []string{"f.go"}, ContextSource: v}}},
			},
		}
		errs := validateAgentContextSource(m)
		if len(errs) != 0 {
			t.Errorf("expected no errors for %q, got %d: %v", v, len(errs), errs)
		}
	}
}

func TestValidateAgentContextSource_invalid(t *testing.T) {
	m := &IMPLManifest{
		State: StateWaveExecuting,
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Task: "t", Files: []string{"f.go"}, ContextSource: "bad-source"}}},
		},
	}
	errs := validateAgentContextSource(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Severity != "error" {
		t.Errorf("expected error severity, got %s", errs[0].Severity)
	}
	if errs[0].Code != result.CodeInvalidEnum {
		t.Errorf("expected code %s, got %s", result.CodeInvalidEnum, errs[0].Code)
	}
}

func TestValidateAgentContextSource_absent_active(t *testing.T) {
	// Absent context_source is always skipped (backwards compatible; no absent-field warning yet).
	m := &IMPLManifest{
		State: StateWaveExecuting,
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Task: "t", Files: []string{"f.go"}, ContextSource: ""}}},
		},
	}
	errs := validateAgentContextSource(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for absent context_source in active state, got %d: %v", len(errs), errs)
	}
}

func TestValidateAgentContextSource_absent_scout(t *testing.T) {
	m := &IMPLManifest{
		State: StateScoutPending,
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Task: "t", Files: []string{"f.go"}, ContextSource: ""}}},
		},
	}
	errs := validateAgentContextSource(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for SCOUT_PENDING state, got %d: %v", len(errs), errs)
	}
}
