package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/builddiag"
	"gopkg.in/yaml.v3"
)

func TestDiagnoseBuildFailureCmd_GoError(t *testing.T) {
	// Register test patterns
	builddiag.RegisterPatterns("go", []builddiag.ErrorPattern{
		{
			Name:        "missing_package",
			Regex:       `cannot find package ".*"`,
			Fix:         "go mod tidy",
			Rationale:   "Package not in go.mod dependencies",
			AutoFixable: true,
			Confidence:  0.95,
		},
	})

	// Create temp error log
	tmpDir := t.TempDir()
	errorLog := filepath.Join(tmpDir, "error.log")
	if err := os.WriteFile(errorLog, []byte(`cannot find package "foo"`), 0644); err != nil {
		t.Fatal(err)
	}

	// Run command
	cmd := newDiagnoseBuildFailureCmd()
	cmd.SetArgs([]string{errorLog, "--language", "go"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse YAML output
	var diag builddiag.Diagnosis
	if err := yaml.Unmarshal(out.Bytes(), &diag); err != nil {
		t.Fatalf("failed to parse YAML: %v\nOutput:\n%s", err, out.String())
	}

	// Verify diagnosis
	if diag.Pattern != "missing_package" {
		t.Errorf("expected pattern 'missing_package', got: '%s'", diag.Pattern)
	}
	if diag.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got: %f", diag.Confidence)
	}
	if diag.Fix != "go mod tidy" {
		t.Errorf("expected fix 'go mod tidy', got: '%s'", diag.Fix)
	}
	if !diag.AutoFixable {
		t.Error("expected auto_fixable to be true")
	}
}

func TestDiagnoseBuildFailureCmd_RustError(t *testing.T) {
	// Register test patterns
	builddiag.RegisterPatterns("rust", []builddiag.ErrorPattern{
		{
			Name:        "missing_dependency",
			Regex:       `error\[E0432\]: unresolved import`,
			Fix:         "cargo add <crate>",
			Rationale:   "Dependency not declared in Cargo.toml",
			AutoFixable: false,
			Confidence:  0.90,
		},
	})

	// Create temp error log
	tmpDir := t.TempDir()
	errorLog := filepath.Join(tmpDir, "error.log")
	if err := os.WriteFile(errorLog, []byte(`error[E0432]: unresolved import 'tokio'`), 0644); err != nil {
		t.Fatal(err)
	}

	// Run command
	cmd := newDiagnoseBuildFailureCmd()
	cmd.SetArgs([]string{errorLog, "--language", "rust"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse YAML output
	var diag builddiag.Diagnosis
	if err := yaml.Unmarshal(out.Bytes(), &diag); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Verify diagnosis
	if diag.Pattern != "missing_dependency" {
		t.Errorf("expected pattern 'missing_dependency', got: %s", diag.Pattern)
	}
	if diag.Confidence != 0.90 {
		t.Errorf("expected confidence 0.90, got: %f", diag.Confidence)
	}
	if diag.AutoFixable {
		t.Error("expected auto_fixable to be false")
	}
}

func TestDiagnoseBuildFailureCmd_InvalidLanguage(t *testing.T) {
	// Create temp error log
	tmpDir := t.TempDir()
	errorLog := filepath.Join(tmpDir, "error.log")
	if err := os.WriteFile(errorLog, []byte("some error"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run command with unsupported language
	cmd := newDiagnoseBuildFailureCmd()
	cmd.SetArgs([]string{errorLog, "--language", "cobol"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported language, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("expected 'unsupported language' error, got: %v", err)
	}
}

func TestDiagnoseBuildFailureCmd_FileNotFound(t *testing.T) {
	// Run command with non-existent file
	cmd := newDiagnoseBuildFailureCmd()
	cmd.SetArgs([]string{"/nonexistent/error.log", "--language", "go"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to read error log") {
		t.Errorf("expected 'failed to read error log' error, got: %v", err)
	}
}

func TestDiagnoseBuildFailureCmd_YAMLOutput(t *testing.T) {
	// Register test patterns
	builddiag.RegisterPatterns("go", []builddiag.ErrorPattern{
		{
			Name:        "test_pattern",
			Regex:       "test error",
			Fix:         "test fix",
			Rationale:   "test rationale",
			AutoFixable: true,
			Confidence:  0.85,
		},
	})

	// Create temp error log
	tmpDir := t.TempDir()
	errorLog := filepath.Join(tmpDir, "error.log")
	if err := os.WriteFile(errorLog, []byte("test error"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run command
	cmd := newDiagnoseBuildFailureCmd()
	cmd.SetArgs([]string{errorLog, "--language", "go"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify YAML structure
	output := out.String()
	requiredFields := []string{"diagnosis:", "confidence:", "fix:", "rationale:", "auto_fixable:"}
	for _, field := range requiredFields {
		if !strings.Contains(output, field) {
			t.Errorf("YAML output missing field: %s\nOutput:\n%s", field, output)
		}
	}

	// Verify it's valid YAML
	var diag builddiag.Diagnosis
	if err := yaml.Unmarshal(out.Bytes(), &diag); err != nil {
		t.Fatalf("output is not valid YAML: %v\nOutput:\n%s", err, output)
	}
}

func TestDiagnoseBuildFailureCmd_MissingLanguageFlag(t *testing.T) {
	// Create temp error log
	tmpDir := t.TempDir()
	errorLog := filepath.Join(tmpDir, "error.log")
	if err := os.WriteFile(errorLog, []byte("some error"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run command without --language flag
	cmd := newDiagnoseBuildFailureCmd()
	cmd.SetArgs([]string{errorLog})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --language flag, got nil")
	}

	output := out.String()
	if !strings.Contains(output, "required flag") && !strings.Contains(err.Error(), "required flag") {
		t.Errorf("expected 'required flag' error message, got: %v\nOutput: %s", err, output)
	}
}

func TestDiagnoseBuildFailureCmd_NoPatternMatch(t *testing.T) {
	// Register test patterns that won't match
	builddiag.RegisterPatterns("go", []builddiag.ErrorPattern{
		{
			Name:        "specific_pattern",
			Regex:       "very specific error that won't match",
			Fix:         "specific fix",
			Rationale:   "specific rationale",
			AutoFixable: true,
			Confidence:  0.95,
		},
	})

	// Create temp error log with unmatched error
	tmpDir := t.TempDir()
	errorLog := filepath.Join(tmpDir, "error.log")
	if err := os.WriteFile(errorLog, []byte("generic build error"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run command
	cmd := newDiagnoseBuildFailureCmd()
	cmd.SetArgs([]string{errorLog, "--language", "go"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Should succeed with "unknown" diagnosis
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for unmatched pattern, got: %v", err)
	}

	// Parse YAML output
	var diag builddiag.Diagnosis
	if err := yaml.Unmarshal(out.Bytes(), &diag); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Verify unknown diagnosis
	if diag.Pattern != "unknown" {
		t.Errorf("expected pattern 'unknown', got: %s", diag.Pattern)
	}
	if diag.Confidence != 0.0 {
		t.Errorf("expected confidence 0.0, got: %f", diag.Confidence)
	}
	if diag.AutoFixable {
		t.Error("expected auto_fixable to be false for unknown pattern")
	}
}
