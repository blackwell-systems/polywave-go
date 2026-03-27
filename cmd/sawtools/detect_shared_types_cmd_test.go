package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDetectSharedTypesCmd_ValidInput(t *testing.T) {
	// Create a temporary IMPL doc with shared type references
	tmpDir := t.TempDir()
	implDocPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	implContent := `
feature: Test feature
repository: .
waves:
  - number: 1
    agents:
      - id: A
        files: [src/models.rs]
        task: |
          Implement PreviewData and SectionTiming types.
        dependencies: []
      - id: B
        files: [src/upgrade/splitter.rs]
        task: |
          import PreviewData from crate::models
          import SectionTiming from crate::models
        dependencies: [A]
      - id: C
        files: [src/upgrade/mod.rs]
        task: |
          use PreviewData from crate::models
        dependencies: [A]

file_ownership:
  - agent: A
    wave: 1
    repo: .
    file: src/models.rs
  - agent: B
    wave: 1
    repo: .
    file: src/upgrade/splitter.rs
  - agent: C
    wave: 1
    repo: .
    file: src/upgrade/mod.rs

interface_contracts: []
scaffolds: []
`

	if err := os.WriteFile(implDocPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Create a fake repo root marker
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create repo marker: %v", err)
	}

	// Run the command
	cmd := newDetectSharedTypesCmd()
	cmd.SetArgs([]string{implDocPath, "--format", "yaml"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Parse YAML output
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse YAML output: %v\nOutput: %s", err, output)
	}

	// Verify shared_types key exists
	sharedTypes, ok := result["shared_types"]
	if !ok {
		t.Fatalf("Output missing 'shared_types' key: %s", output)
	}

	// Verify we got some candidates (PreviewData should be detected)
	typesSlice, ok := sharedTypes.([]interface{})
	if !ok {
		t.Fatalf("shared_types is not an array: %T", sharedTypes)
	}

	if len(typesSlice) == 0 {
		t.Fatalf("Expected at least one shared type candidate, got 0")
	}

	// Verify structure contains expected fields
	firstType := typesSlice[0].(map[string]interface{})
	requiredFields := []string{"type_name", "defining_agent", "defining_file", "referencing_agents", "referencing_files", "reason"}
	for _, field := range requiredFields {
		if _, ok := firstType[field]; !ok {
			t.Errorf("Candidate missing required field: %s", field)
		}
	}
}

func TestDetectSharedTypesCmd_NoCandidates(t *testing.T) {
	// Create a temporary IMPL doc with no shared types
	tmpDir := t.TempDir()
	implDocPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	implContent := `
feature: Test feature
repository: .
waves:
  - wave: 1
    agents:
      - id: A
        files: [src/models.rs]
        task: "Implement types."
        dependencies: []

file_ownership:
  - agent: A
    wave: 1
    repo: .
    file: src/models.rs

interface_contracts: []
scaffolds: []
`

	if err := os.WriteFile(implDocPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Create a fake repo root marker
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create repo marker: %v", err)
	}

	// Run the command
	cmd := newDetectSharedTypesCmd()
	cmd.SetArgs([]string{implDocPath, "--format", "yaml"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Parse YAML output
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse YAML output: %v\nOutput: %s", err, output)
	}

	// Verify shared_types key exists and is empty
	sharedTypes, ok := result["shared_types"]
	if !ok {
		t.Fatalf("Output missing 'shared_types' key: %s", output)
	}

	typesSlice, ok := sharedTypes.([]interface{})
	if !ok {
		t.Fatalf("shared_types is not an array: %T", sharedTypes)
	}

	if len(typesSlice) != 0 {
		t.Errorf("Expected 0 candidates, got %d", len(typesSlice))
	}
}

func TestDetectSharedTypesCmd_JSONFormat(t *testing.T) {
	// Create a temporary IMPL doc
	tmpDir := t.TempDir()
	implDocPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	implContent := `
feature: Test feature
repository: .
waves:
  - wave: 1
    agents:
      - id: A
        files: [src/models.rs]
        task: "Implement types."
        dependencies: []

file_ownership:
  - agent: A
    wave: 1
    repo: .
    file: src/models.rs

interface_contracts: []
scaffolds: []
`

	if err := os.WriteFile(implDocPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Create a fake repo root marker
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create repo marker: %v", err)
	}

	// Run the command with --format json
	cmd := newDetectSharedTypesCmd()
	cmd.SetArgs([]string{implDocPath, "--format", "json"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output is valid JSON
	if !strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Fatalf("Output doesn't look like JSON: %s", output)
	}

	// Verify shared_types key exists in JSON
	if !strings.Contains(output, "shared_types") {
		t.Fatalf("JSON output missing 'shared_types' key: %s", output)
	}
}

func TestDetectSharedTypesCmd_MissingFile(t *testing.T) {
	// Run command with non-existent file
	cmd := newDetectSharedTypesCmd()
	cmd.SetArgs([]string{"/nonexistent/path/IMPL-test.yaml", "--format", "yaml"})

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := cmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	// Should fail with exit code 1
	if err == nil {
		t.Fatal("Expected command to fail with missing file, but it succeeded")
	}

	// Read captured error
	var buf bytes.Buffer
	io.Copy(&buf, r)
	errOutput := buf.String()

	// Verify error message mentions the missing file
	if !strings.Contains(err.Error(), "failed to parse IMPL doc") {
		t.Errorf("Expected error about parsing IMPL doc, got: %v\nStderr: %s", err, errOutput)
	}
}
