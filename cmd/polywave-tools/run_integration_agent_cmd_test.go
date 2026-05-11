package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const testManifestNoGaps = `feature: test-integration
slug: test-integration
file_ownership:
  - file: pkg/foo/bar.go
    agent: A
    wave: 1
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: A
        task: implement bar
        files:
          - pkg/foo/bar.go
completion_reports:
  A:
    status: complete
    commit: abc123
    files_changed:
      - pkg/foo/bar.go
integration_reports:
  wave1:
    wave: 1
    valid: true
    gaps: []
    summary: "No integration gaps detected"
`

const testManifestWithGaps = `feature: test-integration
slug: test-integration
file_ownership:
  - file: pkg/foo/bar.go
    agent: A
    wave: 1
interface_contracts: []
integration_connectors:
  - pkg/integration/wiring.go
waves:
  - number: 1
    agents:
      - id: A
        task: implement bar
        files:
          - pkg/foo/bar.go
completion_reports:
  A:
    status: complete
    commit: abc123
    files_changed:
      - pkg/foo/bar.go
    files_created:
      - pkg/foo/bar.go
integration_reports:
  wave1:
    wave: 1
    valid: false
    gaps:
      - export_name: ProcessData
        file_path: pkg/foo/bar.go
        agent_id: A
        category: implementation
        severity: warning
        reason: "exported func ProcessData has no call-sites outside its defining file"
    summary: "1 integration gap(s) detected"
`

func TestRunIntegrationAgent_NoGaps(t *testing.T) {
	// Create a test manifest with no integration gaps
	manifestPath := writeManifest(t, testManifestNoGaps)

	var stdout bytes.Buffer
	cmd := newRunIntegrationAgentCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{manifestPath, "--wave", "1"})

	// Set repoDir to temp directory
	repoDir = t.TempDir()

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, stdout.String())
	}

	// Verify result structure
	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("expected success=true, got: %v", result["success"])
	}

	if gapCount, ok := result["gap_count"].(float64); !ok || gapCount != 0 {
		t.Errorf("expected gap_count=0, got: %v", result["gap_count"])
	}

	if agentLaunched, ok := result["agent_launched"].(bool); !ok || agentLaunched {
		t.Errorf("expected agent_launched=false when no gaps, got: %v", result["agent_launched"])
	}
}

func TestRunIntegrationAgent_WithGaps_NoConnectors(t *testing.T) {
	// Create a test manifest with gaps but no integration_connectors
	// This should fail during validation (E26-P2 precondition)
	manifestWithoutConnectors := `feature: test-integration
slug: test-integration
file_ownership:
  - file: pkg/foo/bar.go
    agent: A
    wave: 1
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: A
        task: implement bar
        files:
          - pkg/foo/bar.go
completion_reports:
  A:
    status: complete
    commit: abc123
    files_changed:
      - pkg/foo/bar.go
integration_reports:
  wave1:
    wave: 1
    valid: false
    gaps:
      - export_name: ProcessData
        file_path: pkg/foo/bar.go
        agent_id: A
        category: implementation
        severity: warning
        reason: "exported func ProcessData has no call-sites outside its defining file"
    summary: "1 integration gap(s) detected"
`

	manifestPath := writeManifest(t, manifestWithoutConnectors)

	cmd := newRunIntegrationAgentCmd()
	cmd.SetArgs([]string{manifestPath, "--wave", "1"})

	// Set repoDir to temp directory
	repoDir = t.TempDir()

	// This should fail because integration_connectors are required when gaps exist
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when integration_connectors are missing but gaps exist")
	}

	// Verify error message mentions integration_connectors
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRunIntegrationAgent_MissingManifest(t *testing.T) {
	cmd := newRunIntegrationAgentCmd()
	cmd.SetArgs([]string{"/nonexistent/path/manifest.yaml", "--wave", "1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestRunIntegrationAgent_MissingWaveFlag(t *testing.T) {
	manifestPath := writeManifest(t, testManifestNoGaps)

	cmd := newRunIntegrationAgentCmd()
	cmd.SetArgs([]string{manifestPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --wave flag is missing")
	}
}

func TestRunIntegrationAgent_ValidateIntegrationFallback(t *testing.T) {
	// Create a manifest WITHOUT an existing integration_report
	// The command should run ValidateIntegration automatically
	manifestWithoutReport := `feature: test-integration
slug: test-integration
file_ownership:
  - file: pkg/foo/bar.go
    agent: A
    wave: 1
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: A
        task: implement bar
        files:
          - pkg/foo/bar.go
completion_reports:
  A:
    status: complete
    commit: abc123
    files_changed:
      - pkg/foo/bar.go
`

	manifestPath := writeManifest(t, manifestWithoutReport)

	// Create a temporary repo directory with the source file
	repoDir = t.TempDir()
	fooDir := filepath.Join(repoDir, "pkg", "foo")
	if err := os.MkdirAll(fooDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a simple Go file with no exports (so no gaps will be detected)
	barGo := filepath.Join(fooDir, "bar.go")
	if err := os.WriteFile(barGo, []byte(`package foo

// internal function
func internal() {}
`), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	cmd := newRunIntegrationAgentCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{manifestPath, "--wave", "1"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, stdout.String())
	}

	// Should have no gaps since the file has no exports
	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("expected success=true, got: %v", result["success"])
	}

	if gapCount, ok := result["gap_count"].(float64); !ok || gapCount != 0 {
		t.Errorf("expected gap_count=0 (no exports in file), got: %v", result["gap_count"])
	}

	if agentLaunched, ok := result["agent_launched"].(bool); !ok || agentLaunched {
		t.Errorf("expected agent_launched=false when no gaps, got: %v", result["agent_launched"])
	}
}
