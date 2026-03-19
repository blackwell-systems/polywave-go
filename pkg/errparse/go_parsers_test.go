package errparse

import (
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// GoBuildParser tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGoBuildParser_SingleError(t *testing.T) {
	p := &GoBuildParser{}
	if p.Name() != "go-build" {
		t.Fatalf("expected name 'go-build', got %q", p.Name())
	}

	stdout := ""
	stderr := "main.go:10:5: undefined: foo\n"
	result := p.Parse(stdout, stderr)

	if result.Tool != "go-build" {
		t.Errorf("expected Tool 'go-build', got %q", result.Tool)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	e := result.Errors[0]
	if e.File != "main.go" {
		t.Errorf("expected File 'main.go', got %q", e.File)
	}
	if e.Line != 10 {
		t.Errorf("expected Line 10, got %d", e.Line)
	}
	if e.Column != 5 {
		t.Errorf("expected Column 5, got %d", e.Column)
	}
	if e.Message != "undefined: foo" {
		t.Errorf("unexpected message %q", e.Message)
	}
	if e.Severity != "error" {
		t.Errorf("expected Severity 'error', got %q", e.Severity)
	}
	if e.Tool != "go-build" {
		t.Errorf("expected Tool 'go-build', got %q", e.Tool)
	}
}

func TestGoBuildParser_MultipleErrors(t *testing.T) {
	p := &GoBuildParser{}

	stderr := `./pkg/foo/bar.go:3:8: could not import fmt (missing package)
./pkg/foo/bar.go:7: syntax error: unexpected }
./cmd/main.go:12:3: undefined: baz
`
	result := p.Parse("", stderr)

	if len(result.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d: %+v", len(result.Errors), result.Errors)
	}

	// First error
	if result.Errors[0].File != "./pkg/foo/bar.go" {
		t.Errorf("e[0] file: got %q", result.Errors[0].File)
	}
	if result.Errors[0].Line != 3 {
		t.Errorf("e[0] line: got %d", result.Errors[0].Line)
	}
	if result.Errors[0].Column != 8 {
		t.Errorf("e[0] col: got %d", result.Errors[0].Column)
	}

	// Second error (no column)
	if result.Errors[1].Line != 7 {
		t.Errorf("e[1] line: got %d", result.Errors[1].Line)
	}
	if result.Errors[1].Column != 0 {
		t.Errorf("e[1] col should be 0, got %d", result.Errors[1].Column)
	}

	// Third error
	if result.Errors[2].File != "./cmd/main.go" {
		t.Errorf("e[2] file: got %q", result.Errors[2].File)
	}
}

func TestGoBuildParser_NoErrors(t *testing.T) {
	p := &GoBuildParser{}
	result := p.Parse("", "")

	if result == nil {
		t.Fatal("Parse returned nil")
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
	if result.Tool != "go-build" {
		t.Errorf("expected Tool 'go-build', got %q", result.Tool)
	}
}

func TestGoBuildParser_ANSIStripped(t *testing.T) {
	p := &GoBuildParser{}
	// ANSI-colored output (some terminals do this)
	stderr := "\x1b[31mmain.go:5:1: syntax error\x1b[0m\n"
	result := p.Parse("", stderr)

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error after stripping ANSI, got %d", len(result.Errors))
	}
	if result.Errors[0].File != "main.go" {
		t.Errorf("file: got %q", result.Errors[0].File)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GoTestParser tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGoTestParser_FailingTests(t *testing.T) {
	p := &GoTestParser{}
	if p.Name() != "go-test" {
		t.Fatalf("expected name 'go-test', got %q", p.Name())
	}

	stdout := `--- FAIL: TestFoo (0.00s)
	foo_test.go:42: expected 1, got 2
--- FAIL: TestBar (0.01s)
	bar_test.go:17: assertion failed
FAIL
`
	result := p.Parse(stdout, "")

	if result.Tool != "go-test" {
		t.Errorf("expected Tool 'go-test', got %q", result.Tool)
	}

	// We expect FAIL entries + file-ref entries
	// FAIL: TestFoo + foo_test.go:42  + FAIL: TestBar + bar_test.go:17
	if len(result.Errors) < 4 {
		t.Fatalf("expected at least 4 error entries, got %d: %+v", len(result.Errors), result.Errors)
	}

	// First entry should be the FAIL: TestFoo marker
	if result.Errors[0].Message != "FAIL: TestFoo" {
		t.Errorf("e[0] message: got %q", result.Errors[0].Message)
	}
	if result.Errors[0].Severity != "error" {
		t.Errorf("e[0] severity: got %q", result.Errors[0].Severity)
	}

	// Second entry is the file reference for TestFoo
	if result.Errors[1].File != "foo_test.go" {
		t.Errorf("e[1] file: got %q", result.Errors[1].File)
	}
	if result.Errors[1].Line != 42 {
		t.Errorf("e[1] line: got %d", result.Errors[1].Line)
	}
	if result.Errors[1].Message != "expected 1, got 2" {
		t.Errorf("e[1] message: got %q", result.Errors[1].Message)
	}
}

func TestGoTestParser_Panic(t *testing.T) {
	p := &GoTestParser{}

	stdout := `panic: runtime error: index out of range [3] with length 2

goroutine 1 [running]:
main.doWork(...)
	/Users/user/project/main.go:25 +0x6c
main.main()
	/Users/user/project/main.go:10 +0x3b
exit status 2
`
	result := p.Parse(stdout, "")

	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error for panic")
	}

	// Find the panic error
	var panicErr *StructuredError
	for i := range result.Errors {
		if len(result.Errors[i].Message) > 5 && result.Errors[i].Message[:6] == "panic:" {
			panicErr = &result.Errors[i]
			break
		}
	}
	if panicErr == nil {
		t.Fatalf("no panic error found in: %+v", result.Errors)
	}
	if panicErr.Severity != "error" {
		t.Errorf("panic severity: got %q", panicErr.Severity)
	}
	// Should have captured a file from the stack
	if panicErr.File == "" {
		t.Errorf("expected panic error to have file reference, got empty")
	}
}

func TestGoTestParser_NoFailures(t *testing.T) {
	p := &GoTestParser{}
	stdout := `ok  	github.com/example/pkg	0.002s
`
	result := p.Parse(stdout, "")

	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors for passing test output, got %d", len(result.Errors))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GoVetParser tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGoVetParser_Warnings(t *testing.T) {
	p := &GoVetParser{}
	if p.Name() != "go-vet" {
		t.Fatalf("expected name 'go-vet', got %q", p.Name())
	}

	stderr := `./pkg/mypackage/myfile.go:23:2: Printf format %d has arg x of wrong type string
./pkg/mypackage/myfile.go:30:5: unreachable code
`
	result := p.Parse("", stderr)

	if result.Tool != "go-vet" {
		t.Errorf("expected Tool 'go-vet', got %q", result.Tool)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(result.Errors))
	}

	e := result.Errors[0]
	if e.File != "./pkg/mypackage/myfile.go" {
		t.Errorf("file: got %q", e.File)
	}
	if e.Line != 23 {
		t.Errorf("line: got %d", e.Line)
	}
	if e.Column != 2 {
		t.Errorf("col: got %d", e.Column)
	}
	if e.Severity != "warning" {
		t.Errorf("severity: got %q", e.Severity)
	}
	if e.Tool != "go-vet" {
		t.Errorf("tool: got %q", e.Tool)
	}
}

func TestGoVetParser_NoWarnings(t *testing.T) {
	p := &GoVetParser{}
	result := p.Parse("", "")
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(result.Errors))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GolangciLintParser tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGolangciLintParser_WithRules(t *testing.T) {
	p := &GolangciLintParser{}
	if p.Name() != "golangci-lint" {
		t.Fatalf("expected name 'golangci-lint', got %q", p.Name())
	}

	stdout := `pkg/foo/bar.go:12:3: variable name 'x' is too short (revive)
pkg/foo/bar.go:25:1: exported function Foo should have comment or be unexported (golint)
pkg/baz/qux.go:5:10: Error return value of 'os.Remove' is not checked (errcheck)
`
	result := p.Parse(stdout, "")

	if result.Tool != "golangci-lint" {
		t.Errorf("expected Tool 'golangci-lint', got %q", result.Tool)
	}
	if len(result.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d: %+v", len(result.Errors), result.Errors)
	}

	e0 := result.Errors[0]
	if e0.File != "pkg/foo/bar.go" {
		t.Errorf("e[0] file: got %q", e0.File)
	}
	if e0.Line != 12 {
		t.Errorf("e[0] line: got %d", e0.Line)
	}
	if e0.Column != 3 {
		t.Errorf("e[0] col: got %d", e0.Column)
	}
	if e0.Rule != "revive" {
		t.Errorf("e[0] rule: got %q", e0.Rule)
	}
	if e0.Message != "variable name 'x' is too short" {
		t.Errorf("e[0] message: got %q", e0.Message)
	}
	if e0.Severity != "warning" {
		t.Errorf("e[0] severity: got %q", e0.Severity)
	}

	e1 := result.Errors[1]
	if e1.Rule != "golint" {
		t.Errorf("e[1] rule: got %q", e1.Rule)
	}

	e2 := result.Errors[2]
	if e2.Rule != "errcheck" {
		t.Errorf("e[2] rule: got %q", e2.Rule)
	}
}

func TestGolangciLintParser_Suggestions(t *testing.T) {
	p := &GolangciLintParser{}

	stdout := `pkg/foo/bar.go:12:3: variable name 'x' is too short (revive)
Fix: rename to 'xValue'
pkg/baz/qux.go:5:10: some other issue (errcheck)
`
	result := p.Parse(stdout, "")

	if len(result.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d: %+v", len(result.Errors), result.Errors)
	}

	if result.Errors[0].Suggestion != "rename to 'xValue'" {
		t.Errorf("e[0] suggestion: got %q", result.Errors[0].Suggestion)
	}
	// Second error has no suggestion
	if result.Errors[1].Suggestion != "" {
		t.Errorf("e[1] suggestion should be empty, got %q", result.Errors[1].Suggestion)
	}
}

func TestGolangciLintParser_BracketRule(t *testing.T) {
	p := &GolangciLintParser{}
	// Some versions of golangci-lint use [rule] instead of (rule)
	stdout := "pkg/foo.go:10:2: some warning [staticcheck]\n"
	result := p.Parse(stdout, "")

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Rule != "staticcheck" {
		t.Errorf("rule: got %q", result.Errors[0].Rule)
	}
}

func TestGolangciLintParser_NoErrors(t *testing.T) {
	p := &GolangciLintParser{}
	result := p.Parse("", "")
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

func TestGolangciLintParser_ANSIColors(t *testing.T) {
	p := &GolangciLintParser{}
	// golangci-lint sometimes colorizes output
	stdout := "\x1b[33mpkg/foo.go:7:1: unused variable (deadcode)\x1b[0m\n"
	result := p.Parse(stdout, "")

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error after ANSI stripping, got %d", len(result.Errors))
	}
	if result.Errors[0].Rule != "deadcode" {
		t.Errorf("rule: got %q", result.Errors[0].Rule)
	}
}
