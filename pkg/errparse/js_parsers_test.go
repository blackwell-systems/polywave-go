package errparse

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TscParser tests
// ---------------------------------------------------------------------------

func TestTscParser_TypeError(t *testing.T) {
	stdout := "src/file.ts(10,5): error TS2322: Type 'string' is not assignable to type 'number'\n"
	p := &TscParser{}
	result := p.Parse(stdout, "")

	if result.Tool != "tsc" {
		t.Errorf("expected tool 'tsc', got %q", result.Tool)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	e := result.Errors[0]
	if e.File != "src/file.ts" {
		t.Errorf("File = %q, want %q", e.File, "src/file.ts")
	}
	if e.Line != 10 {
		t.Errorf("Line = %d, want 10", e.Line)
	}
	if e.Column != 5 {
		t.Errorf("Column = %d, want 5", e.Column)
	}
	if e.Severity != "error" {
		t.Errorf("Severity = %q, want 'error'", e.Severity)
	}
	if !strings.Contains(e.Message, "Type 'string' is not assignable") {
		t.Errorf("unexpected message: %q", e.Message)
	}
	if e.Tool != "tsc" {
		t.Errorf("Tool = %q, want 'tsc'", e.Tool)
	}
}

func TestTscParser_MultipleErrors(t *testing.T) {
	stdout := `src/a.ts(1,1): error TS2304: Cannot find name 'foo'
src/b.ts(22,3): warning TS2345: Argument of type 'number' is not assignable to parameter of type 'string'
src/c.ts(5,10): error TS2339: Property 'bar' does not exist on type 'never'
`
	p := &TscParser{}
	result := p.Parse(stdout, "")

	if len(result.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(result.Errors))
	}

	cases := []struct {
		file     string
		line     int
		col      int
		severity string
	}{
		{"src/a.ts", 1, 1, "error"},
		{"src/b.ts", 22, 3, "warning"},
		{"src/c.ts", 5, 10, "error"},
	}
	for i, c := range cases {
		e := result.Errors[i]
		if e.File != c.file {
			t.Errorf("[%d] File = %q, want %q", i, e.File, c.file)
		}
		if e.Line != c.line {
			t.Errorf("[%d] Line = %d, want %d", i, e.Line, c.line)
		}
		if e.Column != c.col {
			t.Errorf("[%d] Column = %d, want %d", i, e.Column, c.col)
		}
		if e.Severity != c.severity {
			t.Errorf("[%d] Severity = %q, want %q", i, e.Severity, c.severity)
		}
		if e.Tool != "tsc" {
			t.Errorf("[%d] Tool = %q, want 'tsc'", i, e.Tool)
		}
	}
}

func TestTscParser_NoErrors(t *testing.T) {
	p := &TscParser{}
	result := p.Parse("", "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

func TestTscParser_StripANSI(t *testing.T) {
	// tsc doesn't normally emit ANSI codes but we still support it.
	stdout := "\x1b[31msrc/file.ts(3,7): error TS2304: Cannot find name 'x'\x1b[0m\n"
	p := &TscParser{}
	result := p.Parse(stdout, "")
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].File != "src/file.ts" {
		t.Errorf("File = %q after ANSI strip", result.Errors[0].File)
	}
}

// ---------------------------------------------------------------------------
// EslintParser tests
// ---------------------------------------------------------------------------

func TestEslintParser_WithRules(t *testing.T) {
	stdout := "src/file.ts:10:5: 'x' is not defined [no-undef]\n"
	p := &EslintParser{}
	result := p.Parse(stdout, "")

	if result.Tool != "eslint" {
		t.Errorf("Tool = %q, want 'eslint'", result.Tool)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	e := result.Errors[0]
	if e.File != "src/file.ts" {
		t.Errorf("File = %q", e.File)
	}
	if e.Line != 10 {
		t.Errorf("Line = %d", e.Line)
	}
	if e.Column != 5 {
		t.Errorf("Column = %d", e.Column)
	}
	if e.Rule != "no-undef" {
		t.Errorf("Rule = %q, want 'no-undef'", e.Rule)
	}
	if e.Tool != "eslint" {
		t.Errorf("Tool = %q", e.Tool)
	}
}

func TestEslintParser_Suggestions(t *testing.T) {
	// Rule that typically has a fix → expect suggestion in text mode.
	stdout := "src/app.ts:5:1: Unexpected var, use let or const instead [no-var]\n"
	p := &EslintParser{}
	result := p.Parse(stdout, "")

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	e := result.Errors[0]
	if e.Rule != "no-var" {
		t.Errorf("Rule = %q", e.Rule)
	}
	if e.Suggestion == "" {
		t.Error("expected non-empty Suggestion")
	}
	if !strings.Contains(e.Suggestion, "no-var") {
		t.Errorf("Suggestion should reference rule name, got: %q", e.Suggestion)
	}
}

func TestEslintParser_JSONFormat(t *testing.T) {
	stdout := `[{"filePath":"/project/src/index.ts","messages":[{"ruleId":"no-console","severity":1,"message":"Unexpected console statement.","line":3,"column":1}]}]`
	p := &EslintParser{}
	result := p.Parse(stdout, "")

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	e := result.Errors[0]
	if e.File != "/project/src/index.ts" {
		t.Errorf("File = %q", e.File)
	}
	if e.Rule != "no-console" {
		t.Errorf("Rule = %q", e.Rule)
	}
	if e.Severity != "warning" {
		t.Errorf("Severity = %q (severity 1 should be warning)", e.Severity)
	}
}

func TestEslintParser_JSONFormat_WithFix(t *testing.T) {
	stdout := `[{"filePath":"/project/src/util.ts","messages":[{"ruleId":"semi","severity":2,"message":"Missing semicolon.","line":7,"column":10,"fix":{"text":";"}}]}]`
	p := &EslintParser{}
	result := p.Parse(stdout, "")

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	e := result.Errors[0]
	if e.Suggestion == "" {
		t.Error("expected non-empty Suggestion for fix-able rule")
	}
	if !strings.Contains(e.Suggestion, "--fix") {
		t.Errorf("Suggestion should mention --fix, got: %q", e.Suggestion)
	}
}

// ---------------------------------------------------------------------------
// NpmTestParser tests
// ---------------------------------------------------------------------------

func TestNpmTestParser_JestFail(t *testing.T) {
	stdout := `
FAIL src/math.test.ts
  ● Math > add returns correct sum

    expect(received).toBe(expected)

    Expected: 5
    Received: 4

      at Object.<anonymous> (src/math.test.ts:8:20)
`
	p := &NpmTestParser{}
	result := p.Parse(stdout, "")

	if result.Tool != "npm-test" {
		t.Errorf("Tool = %q", result.Tool)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least 1 error")
	}
	e := result.Errors[0]
	if !strings.Contains(e.Message, "Math > add returns correct sum") {
		t.Errorf("Message = %q", e.Message)
	}
	if e.Tool != "npm-test" {
		t.Errorf("Tool = %q", e.Tool)
	}
	// Should resolve file reference from stack trace
	if e.File == "" || e.File == "unknown" {
		t.Errorf("expected file reference from stack trace, got %q", e.File)
	}
	if e.Line != 8 {
		t.Errorf("Line = %d, want 8", e.Line)
	}
}

func TestNpmTestParser_VitestFail(t *testing.T) {
	stdout := `
FAIL src/counter.test.ts

  × counter increments correctly
    AssertionError: expected 0 to equal 1

      at src/counter.test.ts:15:12
`
	p := &NpmTestParser{}
	result := p.Parse(stdout, "")

	if len(result.Errors) == 0 {
		t.Fatal("expected at least 1 error")
	}
	e := result.Errors[0]
	if !strings.Contains(e.Message, "counter increments correctly") {
		t.Errorf("Message = %q", e.Message)
	}
	if e.Tool != "npm-test" {
		t.Errorf("Tool = %q", e.Tool)
	}
}

func TestNpmTestParser_MultipleFailures(t *testing.T) {
	stdout := `
FAIL src/foo.test.ts
  ● Foo > does something

      at Object.<anonymous> (src/foo.test.ts:5:3)

  ● Foo > does another thing

      at Object.<anonymous> (src/foo.test.ts:12:3)

FAIL src/bar.test.ts
  ● Bar > works

      at Object.<anonymous> (src/bar.test.ts:3:1)
`
	p := &NpmTestParser{}
	result := p.Parse(stdout, "")

	if len(result.Errors) < 3 {
		t.Errorf("expected at least 3 errors, got %d", len(result.Errors))
	}
}
