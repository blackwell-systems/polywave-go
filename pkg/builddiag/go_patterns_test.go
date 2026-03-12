package builddiag

import (
	"testing"
)

func TestGoPatterns_MissingPackage(t *testing.T) {
	errorLog := `main.go:5:2: cannot find package "github.com/foo/bar" in any of:
	/usr/local/go/src/github.com/foo/bar (from $GOROOT)
	/home/user/go/src/github.com/foo/bar (from $GOPATH)`

	diag, err := DiagnoseError(errorLog, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diag.Pattern != "missing_package" {
		t.Errorf("expected pattern 'missing_package', got %q", diag.Pattern)
	}

	if diag.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", diag.Confidence)
	}

	if !diag.AutoFixable {
		t.Errorf("expected AutoFixable=true for missing_package")
	}

	expectedFix := "go mod tidy && go build ./..."
	if diag.Fix != expectedFix {
		t.Errorf("expected fix %q, got %q", expectedFix, diag.Fix)
	}
}

func TestGoPatterns_UndefinedIdentifier(t *testing.T) {
	errorLog := `main.go:10:5: undefined: SomeFunction`

	diag, err := DiagnoseError(errorLog, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diag.Pattern != "undefined_identifier" {
		t.Errorf("expected pattern 'undefined_identifier', got %q", diag.Pattern)
	}

	if diag.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", diag.Confidence)
	}

	if diag.AutoFixable {
		t.Errorf("expected AutoFixable=false for undefined_identifier")
	}
}

func TestGoPatterns_TypeMismatch(t *testing.T) {
	errorLog := `main.go:15:10: cannot use myFunc (type func(int) string) as type Handler in argument to RegisterHandler`

	diag, err := DiagnoseError(errorLog, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diag.Pattern != "type_mismatch" {
		t.Errorf("expected pattern 'type_mismatch', got %q", diag.Pattern)
	}

	if diag.Confidence != 0.90 {
		t.Errorf("expected confidence 0.90, got %f", diag.Confidence)
	}

	if diag.AutoFixable {
		t.Errorf("expected AutoFixable=false for type_mismatch")
	}
}

func TestGoPatterns_ImportCycle(t *testing.T) {
	errorLog := `package github.com/user/project/pkg/foo
	imports github.com/user/project/pkg/bar
	imports github.com/user/project/pkg/foo: import cycle not allowed`

	diag, err := DiagnoseError(errorLog, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diag.Pattern != "import_cycle" {
		t.Errorf("expected pattern 'import_cycle', got %q", diag.Pattern)
	}

	if diag.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", diag.Confidence)
	}

	if diag.AutoFixable {
		t.Errorf("expected AutoFixable=false for import_cycle")
	}
}

func TestGoPatterns_SyntaxError(t *testing.T) {
	errorLog := `main.go:20:15: syntax error: unexpected comma, expecting )`

	diag, err := DiagnoseError(errorLog, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diag.Pattern != "syntax_error" {
		t.Errorf("expected pattern 'syntax_error', got %q", diag.Pattern)
	}

	if diag.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", diag.Confidence)
	}

	if diag.AutoFixable {
		t.Errorf("expected AutoFixable=false for syntax_error")
	}
}

func TestGoPatterns_MissingGoSumEntry(t *testing.T) {
	errorLog := `verifying github.com/foo/bar@v1.2.3: missing go.sum entry; to add it:
	go mod download github.com/foo/bar`

	diag, err := DiagnoseError(errorLog, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diag.Pattern != "missing_go_sum_entry" {
		t.Errorf("expected pattern 'missing_go_sum_entry', got %q", diag.Pattern)
	}

	if diag.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", diag.Confidence)
	}

	if !diag.AutoFixable {
		t.Errorf("expected AutoFixable=true for missing_go_sum_entry")
	}

	expectedFix := "go mod tidy"
	if diag.Fix != expectedFix {
		t.Errorf("expected fix %q, got %q", expectedFix, diag.Fix)
	}
}

func TestGoPatterns_ConfidenceLevels(t *testing.T) {
	// Verify all patterns are ordered by confidence (highest first)
	patterns := catalogs["go"]
	if len(patterns) == 0 {
		t.Fatal("no Go patterns registered")
	}

	prevConfidence := 1.0
	for i, pattern := range patterns {
		if pattern.Confidence > prevConfidence {
			t.Errorf("pattern %d (%s) has higher confidence (%.2f) than previous (%.2f) - patterns should be ordered by confidence descending",
				i, pattern.Name, pattern.Confidence, prevConfidence)
		}

		if pattern.Confidence < 0.0 || pattern.Confidence > 1.0 {
			t.Errorf("pattern %s has invalid confidence %.2f (must be 0.0-1.0)",
				pattern.Name, pattern.Confidence)
		}

		prevConfidence = pattern.Confidence
	}

	// Verify at least one high-confidence pattern exists
	hasHighConfidence := false
	for _, pattern := range patterns {
		if pattern.Confidence >= 0.90 {
			hasHighConfidence = true
			break
		}
	}
	if !hasHighConfidence {
		t.Error("expected at least one pattern with confidence >= 0.90")
	}
}

func TestGoPatterns_PackageNotInGOROOT(t *testing.T) {
	// Test alternative wording for missing package
	errorLog := `main.go:3:8: package github.com/unknown/pkg is not in GOROOT (/usr/local/go/src/github.com/unknown/pkg)`

	diag, err := DiagnoseError(errorLog, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diag.Pattern != "missing_package" {
		t.Errorf("expected pattern 'missing_package', got %q", diag.Pattern)
	}
}
