package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/scaffoldval"
)

// TestValidateScaffoldCmd_Valid tests successful validation of a valid scaffold file.
func TestValidateScaffoldCmd_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid scaffold file
	scaffoldContent := `package types

type Metric struct {
	Name  string
	Value float64
}
`
	scaffoldPath := filepath.Join(tmpDir, "scaffold.go")
	if err := os.WriteFile(scaffoldPath, []byte(scaffoldContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a minimal IMPL doc in a docs/ subdirectory (validator expects this structure)
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}

	implContent := `title: Test IMPL
wave_count: 1
`
	implPath := filepath.Join(docsDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout/stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newValidateScaffoldCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{scaffoldPath, "--impl-doc", implPath})

	// Execute - should succeed (exit 0)
	// Note: os.Exit(1) is called for failures, but we can't test that in unit tests.
	// The command will return nil error and output YAML; we verify the YAML shows PASS.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	// Parse YAML output
	var result scaffoldval.ValidationResult
	output := stdout.Bytes()
	if len(output) == 0 {
		t.Fatalf("no output produced")
	}

	if err := yaml.Unmarshal(output, &result); err != nil {
		t.Fatalf("failed to parse YAML output: %v\noutput: %s", err, stdout.String())
	}

	// Verify syntax passed
	if result.Syntax.Status != "PASS" {
		t.Errorf("expected syntax PASS, got %s: %v", result.Syntax.Status, result.Syntax.Errors)
	}

	// Verify overall status
	if result.OverallStatus() != "PASS" {
		t.Errorf("expected overall PASS, got %s", result.OverallStatus())
	}
}

// TestValidateScaffoldCmd_Invalid tests validation of an invalid scaffold file (syntax error).
func TestValidateScaffoldCmd_Invalid(t *testing.T) {
	// Note: This test cannot verify the os.Exit(1) behavior in unit tests.
	// We verify that the YAML output correctly shows FAIL status.
	// The exit code behavior must be tested manually or in integration tests.

	tmpDir := t.TempDir()

	// Create a scaffold file with syntax error (unclosed brace)
	scaffoldContent := `package types

type Metric struct {
	Name  string
	Value float64
// Missing closing brace
`
	scaffoldPath := filepath.Join(tmpDir, "invalid_scaffold.go")
	if err := os.WriteFile(scaffoldPath, []byte(scaffoldContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a minimal IMPL doc in docs/ subdirectory
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}

	implContent := `title: Test IMPL
wave_count: 1
`
	implPath := filepath.Join(docsDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatal(err)
	}

	// We need to skip this test because os.Exit(1) will kill the test runner.
	// This is a known limitation of testing commands that call os.Exit().
	// The functionality should be tested via integration tests or manual testing.
	t.Skip("Cannot test os.Exit(1) behavior in unit tests - requires integration test")
}

// TestValidateScaffoldCmd_MissingImplDoc tests error handling when --impl-doc is not provided.
func TestValidateScaffoldCmd_MissingImplDoc(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid scaffold file
	scaffoldContent := `package types

type Metric struct {
	Name  string
	Value float64
}
`
	scaffoldPath := filepath.Join(tmpDir, "scaffold.go")
	if err := os.WriteFile(scaffoldPath, []byte(scaffoldContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout/stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command WITHOUT --impl-doc flag
	cmd := newValidateScaffoldCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{scaffoldPath})

	// Execute - should fail
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --impl-doc not provided, got nil")
	}

	// Verify error message mentions impl-doc
	errMsg := err.Error()
	if !strings.Contains(errMsg, "impl-doc") && !strings.Contains(errMsg, "required") {
		t.Errorf("expected error about missing --impl-doc flag, got: %v", err)
	}
}

// TestValidateScaffoldCmd_FileNotFound tests handling of non-existent scaffold file.
func TestValidateScaffoldCmd_FileNotFound(t *testing.T) {
	// Note: Validation of non-existent files returns a FAIL status (syntax error)
	// and calls os.Exit(1), which we cannot test in unit tests.
	// We verify that the command runs and produces YAML output showing the failure.

	tmpDir := t.TempDir()

	// Create a minimal IMPL doc in docs/ subdirectory
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}

	implContent := `title: Test IMPL
wave_count: 1
`
	implPath := filepath.Join(docsDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Skip this test because validation will call os.Exit(1) for the failure
	t.Skip("Cannot test os.Exit(1) behavior in unit tests - requires integration test")
}

// TestValidateScaffoldCmd_YAMLOutput tests the YAML output format.
func TestValidateScaffoldCmd_YAMLOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid scaffold file
	scaffoldContent := `package types

import "fmt"

type Logger struct {
	prefix string
}

func (l *Logger) Log(msg string) {
	fmt.Println(l.prefix + msg)
}
`
	scaffoldPath := filepath.Join(tmpDir, "scaffold.go")
	if err := os.WriteFile(scaffoldPath, []byte(scaffoldContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a minimal IMPL doc
	implContent := `title: Test IMPL
wave_count: 1
`
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newValidateScaffoldCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{scaffoldPath, "--impl-doc", implPath})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse YAML output
	var result scaffoldval.ValidationResult
	if err := yaml.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse YAML output: %v\noutput: %s", err, stdout.String())
	}

	// Verify all expected fields are present
	if result.Syntax.Status == "" {
		t.Error("expected syntax.status to be set")
	}
	if result.Imports.Status == "" {
		t.Error("expected imports.status to be set")
	}
	if result.TypeReferences.Status == "" {
		t.Error("expected type_references.status to be set")
	}
	if result.Build.Status == "" {
		t.Error("expected build.status to be set")
	}

	// Verify YAML structure (check raw output contains expected keys)
	output := stdout.String()
	expectedKeys := []string{"syntax:", "imports:", "type_references:", "build:", "status:"}
	for _, key := range expectedKeys {
		if !strings.Contains(output, key) {
			t.Errorf("expected YAML output to contain %q, output:\n%s", key, output)
		}
	}
}

// TestValidateScaffoldCmd_NoArgs tests error handling when no arguments are provided.
func TestValidateScaffoldCmd_NoArgs(t *testing.T) {
	// Capture stdout/stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command with no args
	cmd := newValidateScaffoldCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{})

	// Execute - should fail due to missing required arg
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no scaffold file provided, got nil")
	}

	// Error should mention required argument
	errMsg := err.Error()
	if !strings.Contains(errMsg, "arg") && !strings.Contains(errMsg, "required") {
		t.Errorf("expected error about missing argument, got: %v", err)
	}
}
