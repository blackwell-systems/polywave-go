package errparse

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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
	res := p.Parse(stdout, stderr)

	if res.Tool != "go-build" {
		t.Errorf("expected Tool 'go-build', got %q", res.Tool)
	}
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(res.Errors))
	}
	e := res.Errors[0]
	if e.File != "main.go" {
		t.Errorf("expected File 'main.go', got %q", e.File)
	}
	if e.Line != 10 {
		t.Errorf("expected Line 10, got %d", e.Line)
	}
	if e.Context["column"] != "5" {
		t.Errorf("expected Context[column] '5', got %q", e.Context["column"])
	}
	if e.Message != "undefined: foo" {
		t.Errorf("unexpected message %q", e.Message)
	}
	if e.Severity != "error" {
		t.Errorf("expected Severity 'error', got %q", e.Severity)
	}
	if e.Code != result.CodeToolError {
		t.Errorf("expected Code %q, got %q", result.CodeToolError, e.Code)
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
	res := p.Parse("", stderr)

	if len(res.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d: %+v", len(res.Errors), res.Errors)
	}

	// First error
	if res.Errors[0].File != "./pkg/foo/bar.go" {
		t.Errorf("e[0] file: got %q", res.Errors[0].File)
	}
	if res.Errors[0].Line != 3 {
		t.Errorf("e[0] line: got %d", res.Errors[0].Line)
	}
	if res.Errors[0].Context["column"] != "8" {
		t.Errorf("e[0] col: got %q", res.Errors[0].Context["column"])
	}

	// Second error (no column)
	if res.Errors[1].Line != 7 {
		t.Errorf("e[1] line: got %d", res.Errors[1].Line)
	}
	if res.Errors[1].Context != nil && res.Errors[1].Context["column"] != "" {
		t.Errorf("e[1] col should be empty, got %q", res.Errors[1].Context["column"])
	}

	// Third error
	if res.Errors[2].File != "./cmd/main.go" {
		t.Errorf("e[2] file: got %q", res.Errors[2].File)
	}
}

func TestGoBuildParser_NoErrors(t *testing.T) {
	p := &GoBuildParser{}
	res := p.Parse("", "")

	if res == nil {
		t.Fatal("Parse returned nil")
	}
	if len(res.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(res.Errors))
	}
	if res.Tool != "go-build" {
		t.Errorf("expected Tool 'go-build', got %q", res.Tool)
	}
}

func TestGoBuildParser_ANSIStripped(t *testing.T) {
	p := &GoBuildParser{}
	// ANSI-colored output (some terminals do this)
	stderr := "\x1b[31mmain.go:5:1: syntax error\x1b[0m\n"
	res := p.Parse("", stderr)

	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error after stripping ANSI, got %d", len(res.Errors))
	}
	if res.Errors[0].File != "main.go" {
		t.Errorf("file: got %q", res.Errors[0].File)
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
	res := p.Parse(stdout, "")

	if res.Tool != "go-test" {
		t.Errorf("expected Tool 'go-test', got %q", res.Tool)
	}

	// We expect FAIL entries + file-ref entries
	// FAIL: TestFoo + foo_test.go:42  + FAIL: TestBar + bar_test.go:17
	if len(res.Errors) < 4 {
		t.Fatalf("expected at least 4 error entries, got %d: %+v", len(res.Errors), res.Errors)
	}

	// First entry should be the FAIL: TestFoo marker
	if res.Errors[0].Message != "FAIL: TestFoo" {
		t.Errorf("e[0] message: got %q", res.Errors[0].Message)
	}
	if res.Errors[0].Severity != "error" {
		t.Errorf("e[0] severity: got %q", res.Errors[0].Severity)
	}

	// Second entry is the file reference for TestFoo
	if res.Errors[1].File != "foo_test.go" {
		t.Errorf("e[1] file: got %q", res.Errors[1].File)
	}
	if res.Errors[1].Line != 42 {
		t.Errorf("e[1] line: got %d", res.Errors[1].Line)
	}
	if res.Errors[1].Message != "expected 1, got 2" {
		t.Errorf("e[1] message: got %q", res.Errors[1].Message)
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
	res := p.Parse(stdout, "")

	if len(res.Errors) == 0 {
		t.Fatal("expected at least one error for panic")
	}

	// Find the panic error
	var panicErr *result.SAWError
	for i := range res.Errors {
		if len(res.Errors[i].Message) > 5 && res.Errors[i].Message[:6] == "panic:" {
			panicErr = &res.Errors[i]
			break
		}
	}
	if panicErr == nil {
		t.Fatalf("no panic error found in: %+v", res.Errors)
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
	res := p.Parse(stdout, "")

	if len(res.Errors) != 0 {
		t.Errorf("expected 0 errors for passing test output, got %d", len(res.Errors))
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
	res := p.Parse("", stderr)

	if res.Tool != "go-vet" {
		t.Errorf("expected Tool 'go-vet', got %q", res.Tool)
	}
	if len(res.Errors) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(res.Errors))
	}

	e := res.Errors[0]
	if e.File != "./pkg/mypackage/myfile.go" {
		t.Errorf("file: got %q", e.File)
	}
	if e.Line != 23 {
		t.Errorf("line: got %d", e.Line)
	}
	if e.Context["column"] != "2" {
		t.Errorf("col: got %q", e.Context["column"])
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
	res := p.Parse("", "")
	if len(res.Errors) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(res.Errors))
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
	res := p.Parse(stdout, "")

	if res.Tool != "golangci-lint" {
		t.Errorf("expected Tool 'golangci-lint', got %q", res.Tool)
	}
	if len(res.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d: %+v", len(res.Errors), res.Errors)
	}

	e0 := res.Errors[0]
	if e0.File != "pkg/foo/bar.go" {
		t.Errorf("e[0] file: got %q", e0.File)
	}
	if e0.Line != 12 {
		t.Errorf("e[0] line: got %d", e0.Line)
	}
	if e0.Context["column"] != "3" {
		t.Errorf("e[0] col: got %q", e0.Context["column"])
	}
	if e0.Context["rule"] != "revive" {
		t.Errorf("e[0] rule: got %q", e0.Context["rule"])
	}
	if e0.Message != "variable name 'x' is too short" {
		t.Errorf("e[0] message: got %q", e0.Message)
	}
	if e0.Severity != "warning" {
		t.Errorf("e[0] severity: got %q", e0.Severity)
	}

	e1 := res.Errors[1]
	if e1.Context["rule"] != "golint" {
		t.Errorf("e[1] rule: got %q", e1.Context["rule"])
	}

	e2 := res.Errors[2]
	if e2.Context["rule"] != "errcheck" {
		t.Errorf("e[2] rule: got %q", e2.Context["rule"])
	}
}

func TestGolangciLintParser_Suggestions(t *testing.T) {
	p := &GolangciLintParser{}

	stdout := `pkg/foo/bar.go:12:3: variable name 'x' is too short (revive)
Fix: rename to 'xValue'
pkg/baz/qux.go:5:10: some other issue (errcheck)
`
	res := p.Parse(stdout, "")

	if len(res.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d: %+v", len(res.Errors), res.Errors)
	}

	if res.Errors[0].Suggestion != "rename to 'xValue'" {
		t.Errorf("e[0] suggestion: got %q", res.Errors[0].Suggestion)
	}
	// Second error has no suggestion
	if res.Errors[1].Suggestion != "" {
		t.Errorf("e[1] suggestion should be empty, got %q", res.Errors[1].Suggestion)
	}
}

func TestGolangciLintParser_BracketRule(t *testing.T) {
	p := &GolangciLintParser{}
	// Some versions of golangci-lint use [rule] instead of (rule)
	stdout := "pkg/foo.go:10:2: some warning [staticcheck]\n"
	res := p.Parse(stdout, "")

	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(res.Errors))
	}
	if res.Errors[0].Context["rule"] != "staticcheck" {
		t.Errorf("rule: got %q", res.Errors[0].Context["rule"])
	}
}

func TestGolangciLintParser_NoErrors(t *testing.T) {
	p := &GolangciLintParser{}
	res := p.Parse("", "")
	if len(res.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(res.Errors))
	}
}

func TestGolangciLintParser_ANSIColors(t *testing.T) {
	p := &GolangciLintParser{}
	// golangci-lint sometimes colorizes output
	stdout := "\x1b[33mpkg/foo.go:7:1: unused variable (deadcode)\x1b[0m\n"
	res := p.Parse(stdout, "")

	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error after ANSI stripping, got %d", len(res.Errors))
	}
	if res.Errors[0].Context["rule"] != "deadcode" {
		t.Errorf("rule: got %q", res.Errors[0].Context["rule"])
	}
}
