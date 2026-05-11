package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunIntegrationWaveCmd_InvalidIMPLPath(t *testing.T) {
	cmd := newRunIntegrationWaveCmd()
	cmd.SetArgs([]string{"/nonexistent/path/IMPL-test.yaml", "--wave", "1"})

	// Execute command (should error)
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid IMPL path, got nil")
	}
}

func TestRunIntegrationWaveCmd_MissingWaveFlag(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Create minimal IMPL doc
	implContent := `title: Test Implementation
feature_slug: test-feature
waves:
  - number: 1
    type: integration
    agents:
      - id: A
        task: "Test task"
        files: [pkg/test.go]
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	cmd := newRunIntegrationWaveCmd()
	cmd.SetArgs([]string{implPath})

	// Execute command (should error due to missing --wave flag)
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for missing --wave flag, got nil")
	}
}

func TestRunIntegrationWaveCmd_WaveNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Create IMPL doc with only wave 1
	implContent := `title: Test Implementation
feature_slug: test-feature
waves:
  - number: 1
    type: integration
    agents:
      - id: A
        task: "Test task"
        files: [pkg/test.go]
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	cmd := newRunIntegrationWaveCmd()
	cmd.SetArgs([]string{implPath, "--wave", "2"})

	// Execute command (should error - wave 2 doesn't exist)
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for non-existent wave, got nil")
	}
	if err != nil && err.Error() != "run-integration-wave: wave 2 not found in manifest" {
		t.Errorf("Expected 'wave 2 not found' error, got: %v", err)
	}
}

func TestRunIntegrationWaveCmd_NonIntegrationWave(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Create IMPL doc with standard wave (not integration)
	implContent := `title: Test Implementation
feature_slug: test-feature
waves:
  - number: 1
    type: standard
    agents:
      - id: A
        task: "Test task"
        files: [pkg/test.go]
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	cmd := newRunIntegrationWaveCmd()
	cmd.SetArgs([]string{implPath, "--wave", "1"})

	// Execute command (should error - wave is standard, not integration)
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for non-integration wave, got nil")
	}
	// Error message should mention wave type mismatch
	if err != nil && err.Error() != "run-integration-wave: wave 1 is not type: integration (found: standard)" {
		t.Errorf("Expected wave type error, got: %v", err)
	}
}

func TestRunIntegrationWaveCmd_DefaultTypeIsStandard(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Create IMPL doc with no explicit type (defaults to standard)
	implContent := `title: Test Implementation
feature_slug: test-feature
waves:
  - number: 1
    agents:
      - id: A
        task: "Test task"
        files: [pkg/test.go]
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	cmd := newRunIntegrationWaveCmd()
	cmd.SetArgs([]string{implPath, "--wave", "1"})

	// Execute command (should error - default type is standard, not integration)
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for wave without explicit type: integration, got nil")
	}
	// Error message should show "standard" as the default
	if err != nil && err.Error() != "run-integration-wave: wave 1 is not type: integration (found: standard)" {
		t.Errorf("Expected default type error, got: %v", err)
	}
}

func TestRunIntegrationWaveCmd_ValidIntegrationWave(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Create IMPL doc with valid integration wave
	implContent := `title: Test Implementation
feature_slug: test-feature
waves:
  - number: 1
    type: integration
    agents:
      - id: A
        task: "Wiring task for agent A"
        files:
          - pkg/main.go
          - pkg/wiring.go
      - id: B
        task: "Wiring task for agent B"
        files:
          - pkg/api/routes.go
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Override repoDir to tmpDir for this test
	oldRepoDir := repoDir
	repoDir = tmpDir
	defer func() { repoDir = oldRepoDir }()

	cmd := newRunIntegrationWaveCmd()
	cmd.SetArgs([]string{implPath, "--wave", "1"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute command (should succeed)
	err := cmd.Execute()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify result structure
	if !result["success"].(bool) {
		t.Error("Expected success: true")
	}
	if int(result["wave"].(float64)) != 1 {
		t.Errorf("Expected wave: 1, got: %v", result["wave"])
	}

	// Verify agents array
	agents, ok := result["agents"].([]interface{})
	if !ok {
		t.Fatal("Expected agents array in result")
	}
	if len(agents) != 2 {
		t.Errorf("Expected 2 agents, got: %d", len(agents))
	}

	// Verify .polywave-agent-brief.md was created
	briefPath := filepath.Join(tmpDir, ".polywave-agent-brief.md")
	if _, err := os.Stat(briefPath); os.IsNotExist(err) {
		t.Error("Expected .polywave-agent-brief.md to be created")
	}
}

func TestRunIntegrationWaveCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Create IMPL doc with integration wave
	implContent := `title: Test Implementation
feature_slug: test-feature
waves:
  - number: 2
    type: integration
    agents:
      - id: X
        task: "Integration task"
        files: [pkg/integration.go]
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL doc: %v", err)
	}

	// Override repoDir to tmpDir for this test
	oldRepoDir := repoDir
	repoDir = tmpDir
	defer func() { repoDir = oldRepoDir }()

	cmd := newRunIntegrationWaveCmd()
	cmd.SetArgs([]string{implPath, "--wave", "2"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute command
	err := cmd.Execute()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify JSON structure
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify required fields
	if _, ok := result["success"]; !ok {
		t.Error("JSON output missing 'success' field")
	}
	if _, ok := result["wave"]; !ok {
		t.Error("JSON output missing 'wave' field")
	}
	if _, ok := result["agents"]; !ok {
		t.Error("JSON output missing 'agents' field")
	}
	if _, ok := result["message"]; !ok {
		t.Error("JSON output missing 'message' field")
	}
}
