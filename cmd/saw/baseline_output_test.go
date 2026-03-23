package main

import (
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/errparse"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func TestFormatBaselineOutput_AllPass(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: true,
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: true, Required: true},
			{Type: "lint", Passed: true, Required: true},
			{Type: "test", Passed: true, Required: true},
		},
	}

	out := FormatBaselineOutput(result)

	for _, gate := range []string{"build", "lint", "test"} {
		if !strings.Contains(out, gate+": PASS") {
			t.Errorf("expected %q: PASS in output, got:\n%s", gate, out)
		}
	}
	if !strings.Contains(out, "Baseline verification passed.") {
		t.Errorf("expected 'Baseline verification passed.' in output, got:\n%s", out)
	}
	if strings.Contains(out, "Error:") {
		t.Errorf("expected no error line when all pass, got:\n%s", out)
	}
}

func TestFormatBaselineOutput_OneFails(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: false,
		Reason: "baseline_verification_failed",
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: true, Required: true},
			{
				Type:     "lint",
				Passed:   false,
				Required: true,
				Stderr:   "pkg/foo/bar.go:10:5: undefined: SomeFunc\npkg/foo/bar.go:20:1: another error",
			},
		},
	}

	out := FormatBaselineOutput(result)

	if !strings.Contains(out, "build: PASS") {
		t.Errorf("expected 'build: PASS' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "lint: FAIL - pkg/foo/bar.go:10:5: undefined: SomeFunc") {
		t.Errorf("expected lint FAIL with stderr detail, got:\n%s", out)
	}
	if !strings.Contains(out, "Error: baseline verification failed.") {
		t.Errorf("expected failure error message in output, got:\n%s", out)
	}
}

func TestFormatBaselineOutput_WithSkipped(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: true,
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: true, Required: true},
			{
				Type:       "test",
				Skipped:    true,
				SkipReason: "lint failed",
				Required:   true,
			},
		},
	}

	out := FormatBaselineOutput(result)

	if !strings.Contains(out, "test: SKIP (lint failed)") {
		t.Errorf("expected 'test: SKIP (lint failed)' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Baseline verification passed.") {
		t.Errorf("expected 'Baseline verification passed.' in output, got:\n%s", out)
	}
}

func TestFormatBaselineOutput_CachedResult(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed:    true,
		FromCache: true,
		CommitSHA: "abc1234",
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: true, Required: true, FromCache: true},
		},
	}

	out := FormatBaselineOutput(result)

	if !strings.Contains(out, "(cached at abc1234)") {
		t.Errorf("expected '(cached at abc1234)' in header, got:\n%s", out)
	}
	if !strings.Contains(out, "Baseline verification passed.") {
		t.Errorf("expected 'Baseline verification passed.' in output, got:\n%s", out)
	}
}

func TestFormatBaselineOutput_EmptyGates(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed:      true,
		GateResults: []protocol.GateResult{},
	}

	out := FormatBaselineOutput(result)

	if !strings.Contains(out, "Baseline verification (E21A):") {
		t.Errorf("expected header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Baseline verification passed.") {
		t.Errorf("expected 'Baseline verification passed.' in output, got:\n%s", out)
	}
}

func TestFormatBaselineOutput_ParsedErrors(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: false,
		Reason: "baseline_verification_failed",
		GateResults: []protocol.GateResult{
			{
				Type:     "build",
				Passed:   false,
				Required: true,
				Stderr:   "some raw stderr output",
				ParsedErrors: []errparse.StructuredError{
					{Message: "pkg/foo/bar.go:5: cannot use x (type int) as type string"},
					{Message: "pkg/foo/bar.go:10: undefined: Baz"},
				},
			},
		},
	}

	out := FormatBaselineOutput(result)

	// ParsedErrors takes priority over Stderr
	if !strings.Contains(out, "build: FAIL - pkg/foo/bar.go:5: cannot use x (type int) as type string") {
		t.Errorf("expected first parsed error in FAIL line, got:\n%s", out)
	}
	// Should NOT show raw stderr when parsed errors exist
	if strings.Contains(out, "some raw stderr output") {
		t.Errorf("expected raw stderr suppressed when parsed errors present, got:\n%s", out)
	}
}

func TestDiagnoseMigrationFailure_GoUndefined(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: false,
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: false, Stderr: "pkg/foo/bar.go:10: undefined: FooType"},
		},
	}
	got := DiagnoseMigrationFailure(result)
	if got == "" {
		t.Fatal("expected suggestion for 'undefined: FooType', got empty string")
	}
	if !strings.Contains(got, "wave boundary") {
		t.Errorf("expected 'wave boundary' in suggestion, got: %s", got)
	}
}

func TestDiagnoseMigrationFailure_GoTypeMismatch(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: false,
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: false, Stderr: "cannot use X as Y in argument"},
		},
	}
	got := DiagnoseMigrationFailure(result)
	if got == "" {
		t.Fatal("expected suggestion for type mismatch, got empty string")
	}
}

func TestDiagnoseMigrationFailure_PythonModuleNotFound(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: false,
		GateResults: []protocol.GateResult{
			{Type: "test", Passed: false, Stderr: "ModuleNotFoundError: No module named foo"},
		},
	}
	got := DiagnoseMigrationFailure(result)
	if got == "" {
		t.Fatal("expected suggestion for Python module not found, got empty string")
	}
}

func TestDiagnoseMigrationFailure_Unrelated(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: false,
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: false, Stderr: "syntax error: unexpected token"},
		},
	}
	got := DiagnoseMigrationFailure(result)
	if got != "" {
		t.Errorf("expected empty string for unrelated error, got: %s", got)
	}
}

func TestDiagnoseMigrationFailure_AllPassing(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: true,
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: true},
			{Type: "test", Passed: true},
		},
	}
	got := DiagnoseMigrationFailure(result)
	if got != "" {
		t.Errorf("expected empty string when all gates pass, got: %s", got)
	}
}

func TestFormatBaselineOutput_WithMigrationHint(t *testing.T) {
	result := &protocol.BaselineResult{
		Passed: false,
		Reason: "baseline_verification_failed",
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: false, Required: true, Stderr: "pkg/foo/bar.go:5: undefined: OldFunc"},
		},
	}
	out := FormatBaselineOutput(result)
	if !strings.Contains(out, "Error: baseline verification failed.") {
		t.Errorf("expected error line, got:\n%s", out)
	}
	if !strings.Contains(out, "Hint: Baseline broken at wave boundary.") {
		t.Errorf("expected migration hint in output, got:\n%s", out)
	}
	if !strings.Contains(out, "re-export aliases") {
		t.Errorf("expected re-export suggestion in hint, got:\n%s", out)
	}
}
