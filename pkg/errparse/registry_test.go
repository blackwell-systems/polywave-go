package errparse

import (
	"testing"
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
		Errors: []StructuredError{{File: "main.go", Line: 1, Severity: "error", Message: "syntax error", Tool: "go-build"}},
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
