package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListProgramsCmd_Discovery tests discovery of PROGRAM manifests.
func TestListProgramsCmd_Discovery(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple PROGRAM manifest files
	manifest1 := `title: Program One
program_slug: program-one
state: PLANNING
impls: []
tiers: []
completion:
  tiers_complete: 0
  tiers_total: 0
  impls_complete: 0
  impls_total: 0
  total_agents: 0
  total_waves: 0
`
	manifest2 := `title: Program Two
program_slug: program-two
state: TIER_EXECUTING
impls: []
tiers: []
completion:
  tiers_complete: 1
  tiers_total: 3
  impls_complete: 2
  impls_total: 5
  total_agents: 10
  total_waves: 4
`

	path1 := filepath.Join(tmpDir, "PROGRAM-one.yaml")
	path2 := filepath.Join(tmpDir, "PROGRAM-two.yaml")

	if err := os.WriteFile(path1, []byte(manifest1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte(manifest2), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newListProgramsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--dir", tmpDir})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse JSON output
	var result []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify we found both programs
	if len(result) != 2 {
		t.Fatalf("expected 2 programs, got %d: %+v", len(result), result)
	}

	// Verify first program (sorted by filename)
	prog1 := result[0]
	if prog1["slug"] != "program-one" {
		t.Errorf("expected first program slug 'program-one', got %v", prog1["slug"])
	}
	if prog1["title"] != "Program One" {
		t.Errorf("expected first program title 'Program One', got %v", prog1["title"])
	}
	if prog1["state"] != "PLANNING" {
		t.Errorf("expected first program state 'PLANNING', got %v", prog1["state"])
	}

	// Verify second program
	prog2 := result[1]
	if prog2["slug"] != "program-two" {
		t.Errorf("expected second program slug 'program-two', got %v", prog2["slug"])
	}
	if prog2["title"] != "Program Two" {
		t.Errorf("expected second program title 'Program Two', got %v", prog2["title"])
	}
	if prog2["state"] != "TIER_EXECUTING" {
		t.Errorf("expected second program state 'TIER_EXECUTING', got %v", prog2["state"])
	}
}

// TestListProgramsCmd_EmptyDir tests handling of empty directory.
func TestListProgramsCmd_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't create any PROGRAM files

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newListProgramsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--dir", tmpDir})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse JSON output
	var result []interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify empty array
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items: %+v", len(result), result)
	}
}

// TestListProgramsCmd_InvalidManifests tests handling of invalid manifests (they should be skipped).
func TestListProgramsCmd_InvalidManifests(t *testing.T) {
	tmpDir := t.TempDir()

	// Create one valid and one invalid manifest
	validManifest := `title: Valid Program
program_slug: valid-program
state: PLANNING
impls: []
tiers: []
completion:
  tiers_complete: 0
  tiers_total: 0
  impls_complete: 0
  impls_total: 0
  total_agents: 0
  total_waves: 0
`
	invalidManifest := `this is not valid YAML: [unclosed bracket`

	validPath := filepath.Join(tmpDir, "PROGRAM-valid.yaml")
	invalidPath := filepath.Join(tmpDir, "PROGRAM-invalid.yaml")

	if err := os.WriteFile(validPath, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidPath, []byte(invalidManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newListProgramsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--dir", tmpDir})

	// Execute - should succeed and skip invalid file
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse JSON output
	var result []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify only valid program was found
	if len(result) != 1 {
		t.Fatalf("expected 1 program, got %d: %+v", len(result), result)
	}

	if result[0]["slug"] != "valid-program" {
		t.Errorf("expected program slug 'valid-program', got %v", result[0]["slug"])
	}
}

// TestListProgramsCmd_JSONOutput tests the JSON output format structure.
func TestListProgramsCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PROGRAM manifest
	manifestContent := `title: Test Program
program_slug: test-program
state: REVIEWED
impls: []
tiers: []
completion:
  tiers_complete: 0
  tiers_total: 0
  impls_complete: 0
  impls_total: 0
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
	cmd := newListProgramsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--dir", tmpDir})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse JSON output
	var result []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify we have one result
	if len(result) != 1 {
		t.Fatalf("expected 1 program, got %d", len(result))
	}

	// Verify expected fields are present
	prog := result[0]
	expectedFields := []string{"path", "slug", "state", "title"}
	for _, field := range expectedFields {
		if _, exists := prog[field]; !exists {
			t.Errorf("expected field %q in output, got: %+v", field, prog)
		}
	}

	// Verify JSON structure in raw output
	output := stdout.String()
	expectedKeys := []string{"\"path\":", "\"slug\":", "\"state\":", "\"title\":"}
	for _, key := range expectedKeys {
		if !strings.Contains(output, key) {
			t.Errorf("expected JSON output to contain %q, output:\n%s", key, output)
		}
	}
}

// TestListProgramsCmd_DefaultDir tests the default directory behavior.
func TestListProgramsCmd_DefaultDir(t *testing.T) {
	// Capture stdout/stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command without --dir flag (uses default "docs/")
	cmd := newListProgramsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{})

	// Execute - should succeed even if docs/ doesn't exist (returns empty array)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse JSON output
	var result []interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Should return a valid (possibly empty) array
	if result == nil {
		t.Error("expected non-nil array in output")
	}
}
