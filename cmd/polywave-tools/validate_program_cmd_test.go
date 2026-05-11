package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateProgramCmd_ValidManifest tests validation of a valid PROGRAM manifest.
func TestValidateProgramCmd_ValidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid PROGRAM manifest
	manifestContent := `title: Test Program
program_slug: test-program
state: PLANNING
impls:
  - slug: impl-a
    title: Implementation A
    tier: 1
    status: pending
  - slug: impl-b
    title: Implementation B
    tier: 2
    depends_on:
      - impl-a
    status: pending
tiers:
  - number: 1
    impls:
      - impl-a
  - number: 2
    impls:
      - impl-b
completion:
  tiers_complete: 0
  tiers_total: 2
  impls_complete: 0
  impls_total: 2
  total_agents: 0
  total_waves: 0
`
	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newValidateProgramCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})

	// Execute - should succeed (exit 0)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	// Parse JSON output
	var result validateProgramOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify valid: true
	if !result.Valid {
		t.Errorf("expected valid: true, got false. Errors: %+v", result.Errors)
	}

	// Verify empty errors array
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %+v", len(result.Errors), result.Errors)
	}
}

// TestValidateProgramCmd_InvalidManifest tests validation of an invalid PROGRAM manifest.
func TestValidateProgramCmd_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid PROGRAM manifest (missing required field: title)
	manifestContent := `program_slug: test-program
state: PLANNING
impls:
  - slug: impl-a
    title: Implementation A
    tier: 1
    status: pending
tiers:
  - number: 1
    impls:
      - impl-a
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 0
  total_waves: 0
`
	manifestPath := filepath.Join(tmpDir, "PROGRAM-invalid.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newValidateProgramCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid program manifest, got nil")
	}
}

// TestValidateProgramCmd_P1Violation tests detection of P1 violations (same-tier dependencies).
func TestValidateProgramCmd_P1Violation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PROGRAM manifest with P1 violation (same-tier dependency)
	manifestContent := `title: Test Program
program_slug: test-program
state: PLANNING
impls:
  - slug: impl-a
    title: Implementation A
    tier: 1
    depends_on:
      - impl-b
    status: pending
  - slug: impl-b
    title: Implementation B
    tier: 1
    status: pending
tiers:
  - number: 1
    impls:
      - impl-a
      - impl-b
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 2
  total_agents: 0
  total_waves: 0
`
	manifestPath := filepath.Join(tmpDir, "PROGRAM-p1.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newValidateProgramCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for P1 violation, got nil")
	}
}

// TestValidateProgramCmd_MissingFile tests handling of non-existent manifest file.
func TestValidateProgramCmd_MissingFile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newValidateProgramCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"/nonexistent/path/PROGRAM-missing.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent manifest file, got nil")
	}
}

// TestValidateProgramCmd_JSONOutput tests the JSON output format.
func TestValidateProgramCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid PROGRAM manifest
	manifestContent := `title: Test Program
program_slug: test-program
state: VALIDATING
impls:
  - slug: impl-core
    title: Core Implementation
    tier: 1
    status: pending
tiers:
  - number: 1
    impls:
      - impl-core
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 5
  total_waves: 3
`
	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newValidateProgramCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse JSON output
	var result validateProgramOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify expected fields are present
	if result.Errors == nil {
		t.Error("expected errors field to be non-nil array")
	}

	// Verify JSON structure (check raw output contains expected keys)
	output := stdout.String()
	expectedKeys := []string{"\"valid\":", "\"errors\":"}
	for _, key := range expectedKeys {
		if !strings.Contains(output, key) {
			t.Errorf("expected JSON output to contain %q, output:\n%s", key, output)
		}
	}
}

// TestValidateProgramCmd_NoArgs tests error handling when no arguments are provided.
func TestValidateProgramCmd_NoArgs(t *testing.T) {
	// Capture stdout/stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command with no args
	cmd := newValidateProgramCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{})

	// Execute - should fail due to missing required arg
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no manifest file provided, got nil")
	}

	// Error should mention required argument
	errMsg := err.Error()
	if !strings.Contains(errMsg, "arg") && !strings.Contains(errMsg, "required") {
		t.Errorf("expected error about missing argument, got: %v", err)
	}
}
