package errparse

import (
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// mockParser is a simple Parser implementation for testing.
type mockParser struct {
	name   string
	result *ParseResult
}

func (m *mockParser) Name() string { return m.name }
func (m *mockParser) Parse(stdout, stderr string) *ParseResult {
	if m.result != nil {
		return m.result
	}
	return &ParseResult{Tool: m.name, Raw: stdout + stderr}
}

// TestDetectTool_GoBuild verifies "go build ./..." -> "go-build"
func TestDetectTool_GoBuild(t *testing.T) {
	got := DetectTool("", "go build ./...")
	if got != "go-build" {
		t.Errorf("expected go-build, got %q", got)
	}
}

// TestDetectTool_GoTest verifies "go test ./..." -> "go-test"
func TestDetectTool_GoTest(t *testing.T) {
	got := DetectTool("", "go test ./...")
	if got != "go-test" {
		t.Errorf("expected go-test, got %q", got)
	}
}

// TestDetectTool_Tsc verifies "tsc --noEmit" -> "tsc"
func TestDetectTool_Tsc(t *testing.T) {
	got := DetectTool("", "tsc --noEmit")
	if got != "tsc" {
		t.Errorf("expected tsc, got %q", got)
	}
}

// TestDetectTool_Eslint verifies "eslint ." -> "eslint"
func TestDetectTool_Eslint(t *testing.T) {
	got := DetectTool("", "eslint .")
	if got != "eslint" {
		t.Errorf("expected eslint, got %q", got)
	}
}

// TestDetectTool_Pytest verifies "pytest" -> "pytest"
func TestDetectTool_Pytest(t *testing.T) {
	got := DetectTool("", "pytest")
	if got != "pytest" {
		t.Errorf("expected pytest, got %q", got)
	}
}

// TestDetectTool_Mypy verifies "mypy ." -> "mypy"
func TestDetectTool_Mypy(t *testing.T) {
	got := DetectTool("", "mypy .")
	if got != "mypy" {
		t.Errorf("expected mypy, got %q", got)
	}
}

// TestDetectTool_Ruff verifies "ruff check ." -> "ruff"
func TestDetectTool_Ruff(t *testing.T) {
	got := DetectTool("", "ruff check .")
	if got != "ruff" {
		t.Errorf("expected ruff, got %q", got)
	}
}

// TestDetectTool_GolangciLint verifies "golangci-lint run" -> "golangci-lint"
func TestDetectTool_GolangciLint(t *testing.T) {
	got := DetectTool("", "golangci-lint run")
	if got != "golangci-lint" {
		t.Errorf("expected golangci-lint, got %q", got)
	}
}

// TestDetectTool_Unknown verifies "unknown-tool" -> ""
func TestDetectTool_Unknown(t *testing.T) {
	got := DetectTool("", "unknown-tool")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// TestParseOutput_Dispatches verifies dispatch to the correct parser.
func TestParseOutput_Dispatches(t *testing.T) {
	// Save registry state and restore after test.
	origRegistry := registry
	registry = map[string]Parser{}
	defer func() { registry = origRegistry }()

	expected := &ParseResult{
		Tool:   "go-build",
		Errors: []result.PolywaveError{{Code: result.CodeToolError, File: "main.go", Line: 1, Severity: "error", Message: "syntax error", Tool: "go-build"}},
		Raw:    "main.go:1: syntax error",
	}

	Register(&mockParser{name: "go-build", result: expected})

	got := ParseOutput("", "go build ./...", "main.go:1: syntax error", "")
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.Tool != "go-build" {
		t.Errorf("expected tool go-build, got %q", got.Tool)
	}
	if len(got.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(got.Errors))
	}
}

// TestParseOutput_UnknownTool verifies nil return for unknown tools.
func TestParseOutput_UnknownTool(t *testing.T) {
	got := ParseOutput("", "unknown-tool arg1", "some output", "")
	if got != nil {
		t.Errorf("expected nil for unknown tool, got %+v", got)
	}
}

// TestAllParsers_Registered verifies ALL 14 parsers are registered via their init() functions.
func TestAllParsers_Registered(t *testing.T) {
	tools := []string{
		"gofmt", "prettier-format", "ruff-format", "cargo-fmt",
		"go-build", "go-test", "go-vet", "golangci-lint",
		"tsc", "eslint", "npm-test",
		"pytest", "mypy", "ruff",
	}
	for _, tool := range tools {
		p := GetParser(tool)
		if p == nil {
			t.Errorf("parser %q not registered — missing init() call?", tool)
			continue
		}
		if p.Name() != tool {
			t.Errorf("parser %q has Name() = %q", tool, p.Name())
		}
	}
}

// TestParseOutput_Integration uses table-driven tests that call ParseOutput with
// realistic sample output for each tool.
func TestParseOutput_Integration(t *testing.T) {
	tests := []struct {
		name         string
		gateType     string
		command      string
		stdout       string
		stderr       string
		expectedTool string
	}{
		{
			name:         "go-build",
			gateType:     "build",
			command:      "go build ./...",
			stdout:       "",
			stderr:       "main.go:10:5: undefined: foo",
			expectedTool: "go-build",
		},
		{
			name:         "go-test",
			gateType:     "test",
			command:      "go test ./...",
			stdout:       "--- FAIL: TestFoo (0.00s)\n\tfoo_test.go:10: expected 1",
			stderr:       "",
			expectedTool: "go-test",
		},
		{
			name:         "go-vet",
			gateType:     "lint",
			command:      "go vet ./...",
			stdout:       "",
			stderr:       "main.go:5:2: unreachable code",
			expectedTool: "go-vet",
		},
		{
			name:         "golangci-lint",
			gateType:     "lint",
			command:      "golangci-lint run",
			stdout:       "",
			stderr:       "main.go:5:2: unused variable (deadcode)",
			expectedTool: "golangci-lint",
		},
		{
			name:         "tsc",
			gateType:     "typecheck",
			command:      "tsc --noEmit",
			stdout:       "src/file.ts(10,5): error TS2322: bad type",
			stderr:       "",
			expectedTool: "tsc",
		},
		{
			name:         "eslint",
			gateType:     "lint",
			command:      "eslint .",
			stdout:       "",
			stderr:       "src/file.ts:5:1: 'x' unused [no-unused-vars]",
			expectedTool: "eslint",
		},
		{
			name:         "npm-test",
			gateType:     "test",
			command:      "npm test",
			stdout:       "FAIL src/file.test.ts\n  \u25cf Suite \u203a test name\n    at Object.<anonymous> (src/file.test.ts:10:5)",
			stderr:       "",
			expectedTool: "npm-test",
		},
		{
			name:         "pytest",
			gateType:     "test",
			command:      "pytest",
			stdout:       "FAILED tests/test_foo.py::test_add - AssertionError",
			stderr:       "",
			expectedTool: "pytest",
		},
		{
			name:         "mypy",
			gateType:     "typecheck",
			command:      "mypy .",
			stdout:       "src/foo.py:10: error: Incompatible types [assignment]",
			stderr:       "",
			expectedTool: "mypy",
		},
		{
			name:         "ruff",
			gateType:     "lint",
			command:      "ruff check .",
			stdout:       "src/foo.py:10:1: E501 Line too long",
			stderr:       "",
			expectedTool: "ruff",
		},
		{
			name:         "gofmt",
			gateType:     "format",
			command:      "gofmt -l .",
			stdout:       "main.go",
			stderr:       "",
			expectedTool: "gofmt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseOutput(tc.gateType, tc.command, tc.stdout, tc.stderr)
			if got == nil {
				t.Fatalf("expected non-nil result for %s", tc.expectedTool)
			}
			if got.Tool != tc.expectedTool {
				t.Errorf("expected tool %q, got %q", tc.expectedTool, got.Tool)
			}
			if len(got.Errors) < 1 {
				t.Errorf("expected at least 1 error, got %d", len(got.Errors))
			}
		})
	}
}

// TestRuffParser_NonConsecutiveHelp verifies that ruff help/suggestion lines are
// correctly associated only with the immediately preceding error.
func TestRuffParser_NonConsecutiveHelp(t *testing.T) {
	parser := &RuffParser{}

	// Test case 1: help line separated by non-matching line
	t.Run("help_after_non_matching_line", func(t *testing.T) {
		input := "src/a.py:1:1: E501 Line too long\nFound 1 error.\n  = help: Shorten the line\nsrc/b.py:2:1: W291 Trailing whitespace"
		result := parser.Parse(input, "")
		if len(result.Errors) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(result.Errors))
		}
		if result.Errors[0].Suggestion != "Shorten the line" {
			t.Errorf("a.py: expected Suggestion=%q, got %q", "Shorten the line", result.Errors[0].Suggestion)
		}
		if result.Errors[1].Suggestion != "" {
			t.Errorf("b.py: expected empty Suggestion, got %q", result.Errors[1].Suggestion)
		}
	})

	// Test case 2: help line after intervening error
	t.Run("help_after_intervening_error", func(t *testing.T) {
		input := "src/a.py:1:1: E501 Line too long\nsrc/b.py:2:1: W291 Trailing whitespace\n  = help: Remove trailing whitespace"
		result := parser.Parse(input, "")
		if len(result.Errors) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(result.Errors))
		}
		if result.Errors[0].Suggestion != "" {
			t.Errorf("a.py: expected empty Suggestion, got %q", result.Errors[0].Suggestion)
		}
		if result.Errors[1].Suggestion != "Remove trailing whitespace" {
			t.Errorf("b.py: expected Suggestion=%q, got %q", "Remove trailing whitespace", result.Errors[1].Suggestion)
		}
	})
}

// TestRegister_CustomParser verifies registering and retrieving a custom parser.
func TestRegister_CustomParser(t *testing.T) {
	// Save registry state and restore after test.
	origRegistry := registry
	registry = map[string]Parser{}
	defer func() { registry = origRegistry }()

	custom := &mockParser{name: "my-custom-tool"}
	Register(custom)

	got := GetParser("my-custom-tool")
	if got == nil {
		t.Fatal("expected parser, got nil")
	}
	if got.Name() != "my-custom-tool" {
		t.Errorf("expected my-custom-tool, got %q", got.Name())
	}

	// Verify unknown parser returns nil
	nilParser := GetParser("nonexistent-tool")
	if nilParser != nil {
		t.Errorf("expected nil for unregistered tool, got %+v", nilParser)
	}
}
