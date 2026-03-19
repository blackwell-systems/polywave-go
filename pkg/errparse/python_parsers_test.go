package errparse

import (
	"testing"
)

// ──────────────────────────────────────────────
// PytestParser tests
// ──────────────────────────────────────────────

func TestPytestParser_FailedTest(t *testing.T) {
	stdout := `============================= test session starts ==============================
collected 3 items

tests/test_foo.py::TestMath::test_add PASSED
tests/test_foo.py::TestMath::test_subtract FAILED

=================================== FAILURES ===================================
_________________________ TestMath.test_subtract _______________________________

    def test_subtract():
        assert 1 - 1 == 1
      File "tests/test_foo.py", line 10, in test_subtract
        assert 1 - 1 == 1
AssertionError: assert 0 == 1

FAILED tests/test_foo.py::TestMath::test_subtract - AssertionError: assert 0 == 1
======================= 1 failed, 1 passed in 0.12s ============================
`
	p := &PytestParser{}
	result := p.Parse(stdout, "")

	if result.Tool != "pytest" {
		t.Errorf("expected tool=pytest, got %q", result.Tool)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error, got none")
	}

	// Find the FAILED error
	var found bool
	for _, e := range result.Errors {
		if e.Rule == "tests/test_foo.py::TestMath::test_subtract" {
			found = true
			if e.File != "tests/test_foo.py" {
				t.Errorf("expected file=tests/test_foo.py, got %q", e.File)
			}
			if e.Severity != "error" {
				t.Errorf("expected severity=error, got %q", e.Severity)
			}
			if e.Tool != "pytest" {
				t.Errorf("expected tool=pytest, got %q", e.Tool)
			}
			// The traceback should have populated the line
			if e.Line != 10 {
				t.Errorf("expected line=10 from traceback, got %d", e.Line)
			}
		}
	}
	if !found {
		t.Error("did not find the expected FAILED test error")
	}
}

func TestPytestParser_CollectionError(t *testing.T) {
	stderr := `ERROR collecting tests/test_broken.py
tests/test_broken.py:1: SyntaxError: invalid syntax`

	p := &PytestParser{}
	result := p.Parse("", stderr)

	if result.Tool != "pytest" {
		t.Errorf("expected tool=pytest, got %q", result.Tool)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error, got none")
	}

	e := result.Errors[0]
	if e.File != "tests/test_broken.py" {
		t.Errorf("expected file=tests/test_broken.py, got %q", e.File)
	}
	if e.Severity != "error" {
		t.Errorf("expected severity=error, got %q", e.Severity)
	}
	if e.Tool != "pytest" {
		t.Errorf("expected tool=pytest, got %q", e.Tool)
	}
}

func TestPytestParser_NoFailures(t *testing.T) {
	stdout := `============================= test session starts ==============================
collected 2 items

tests/test_foo.py::test_one PASSED
tests/test_foo.py::test_two PASSED

============================== 2 passed in 0.05s ==============================
`
	p := &PytestParser{}
	result := p.Parse(stdout, "")

	if result.Tool != "pytest" {
		t.Errorf("expected tool=pytest, got %q", result.Tool)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors on clean run, got %d", len(result.Errors))
	}
	if result.Raw == "" {
		t.Error("expected Raw to be populated")
	}
}

// ──────────────────────────────────────────────
// MypyParser tests
// ──────────────────────────────────────────────

func TestMypyParser_TypeError(t *testing.T) {
	stdout := `src/foo.py:42: error: Incompatible types in assignment (expression has type "str", variable has type "int") [assignment]
Found 1 error in 1 file (checked 3 source files)
`
	p := &MypyParser{}
	result := p.Parse(stdout, "")

	if result.Tool != "mypy" {
		t.Errorf("expected tool=mypy, got %q", result.Tool)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error, got none")
	}

	e := result.Errors[0]
	if e.File != "src/foo.py" {
		t.Errorf("expected file=src/foo.py, got %q", e.File)
	}
	if e.Line != 42 {
		t.Errorf("expected line=42, got %d", e.Line)
	}
	if e.Severity != "error" {
		t.Errorf("expected severity=error, got %q", e.Severity)
	}
	if e.Rule != "assignment" {
		t.Errorf("expected rule=assignment, got %q", e.Rule)
	}
	if e.Tool != "mypy" {
		t.Errorf("expected tool=mypy, got %q", e.Tool)
	}
}

func TestMypyParser_MultipleErrors(t *testing.T) {
	stdout := `src/foo.py:10: error: Name "bar" is not defined [name-defined]
src/foo.py:20: warning: Unused variable "x" [misc]
src/bar.py:5: error: Argument 1 to "func" has incompatible type "int"; expected "str" [arg-type]
Found 3 errors in 2 files (checked 5 source files)
`
	p := &MypyParser{}
	result := p.Parse(stdout, "")

	if result.Tool != "mypy" {
		t.Errorf("expected tool=mypy, got %q", result.Tool)
	}
	if len(result.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(result.Errors))
	}

	// First error
	if result.Errors[0].File != "src/foo.py" || result.Errors[0].Line != 10 {
		t.Errorf("first error: expected src/foo.py:10, got %s:%d", result.Errors[0].File, result.Errors[0].Line)
	}
	if result.Errors[0].Rule != "name-defined" {
		t.Errorf("first error: expected rule=name-defined, got %q", result.Errors[0].Rule)
	}

	// Second error (warning)
	if result.Errors[1].Severity != "warning" {
		t.Errorf("second error: expected severity=warning, got %q", result.Errors[1].Severity)
	}

	// Third error
	if result.Errors[2].File != "src/bar.py" || result.Errors[2].Line != 5 {
		t.Errorf("third error: expected src/bar.py:5, got %s:%d", result.Errors[2].File, result.Errors[2].Line)
	}
	if result.Errors[2].Rule != "arg-type" {
		t.Errorf("third error: expected rule=arg-type, got %q", result.Errors[2].Rule)
	}
}

// ──────────────────────────────────────────────
// RuffParser tests
// ──────────────────────────────────────────────

func TestRuffParser_LintError(t *testing.T) {
	stdout := `src/foo.py:10:1: E501 Line too long (120 > 88 characters)
src/foo.py:25:5: W291 Trailing whitespace
`
	p := &RuffParser{}
	result := p.Parse(stdout, "")

	if result.Tool != "ruff" {
		t.Errorf("expected tool=ruff, got %q", result.Tool)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(result.Errors))
	}

	e := result.Errors[0]
	if e.File != "src/foo.py" {
		t.Errorf("expected file=src/foo.py, got %q", e.File)
	}
	if e.Line != 10 {
		t.Errorf("expected line=10, got %d", e.Line)
	}
	if e.Column != 1 {
		t.Errorf("expected col=1, got %d", e.Column)
	}
	if e.Rule != "E501" {
		t.Errorf("expected rule=E501, got %q", e.Rule)
	}
	if e.Severity != "error" {
		t.Errorf("expected severity=error for E-code, got %q", e.Severity)
	}
	if e.Tool != "ruff" {
		t.Errorf("expected tool=ruff, got %q", e.Tool)
	}

	// W291 should be a warning
	w := result.Errors[1]
	if w.Rule != "W291" {
		t.Errorf("expected rule=W291, got %q", w.Rule)
	}
	if w.Severity != "warning" {
		t.Errorf("expected severity=warning for W-code, got %q", w.Severity)
	}
}

func TestRuffParser_Suggestions(t *testing.T) {
	stdout := `src/foo.py:3:1: F401 [*] "os" imported but unused
  = help: Remove unused import: "os"
src/foo.py:7:5: E711 Comparison to None (use "is" or "is not")
`
	p := &RuffParser{}
	result := p.Parse(stdout, "")

	if result.Tool != "ruff" {
		t.Errorf("expected tool=ruff, got %q", result.Tool)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(result.Errors))
	}

	// First error should have a suggestion attached
	e := result.Errors[0]
	if e.Rule != "F401" {
		t.Errorf("expected rule=F401, got %q", e.Rule)
	}
	if e.Suggestion == "" {
		t.Error("expected suggestion to be populated from '= help:' line")
	}
	if e.Suggestion != `Remove unused import: "os"` {
		t.Errorf("unexpected suggestion: %q", e.Suggestion)
	}

	// Second error should have no suggestion
	e2 := result.Errors[1]
	if e2.Suggestion != "" {
		t.Errorf("expected no suggestion for second error, got %q", e2.Suggestion)
	}
}
