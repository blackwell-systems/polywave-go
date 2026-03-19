package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const programStateTestManifestYAML = `title: Test Program
program_slug: test-program
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
    total_agents: 2
    total_waves: 1
`

// TestUpdateProgramState_Success tests that update-program-state correctly updates
// the state field and outputs the expected JSON with "updated":true.
func TestUpdateProgramState_Success(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	if err := os.WriteFile(manifestPath, []byte(programStateTestManifestYAML), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newUpdateProgramStateCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath, "--state", "REVIEWED"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	var result UpdateProgramStateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	if !result.Updated {
		t.Errorf("expected updated: true, got false")
	}
	if result.ManifestPath != manifestPath {
		t.Errorf("expected manifest_path: %s, got %s", manifestPath, result.ManifestPath)
	}
	if result.PreviousState != "PLANNING" {
		t.Errorf("expected previous_state: PLANNING, got %s", result.PreviousState)
	}
	if result.NewState != "REVIEWED" {
		t.Errorf("expected new_state: REVIEWED, got %s", result.NewState)
	}
}

// TestUpdateProgramState_ManifestIsUpdatedOnDisk verifies that the YAML file
// is actually rewritten with the new state value after the command executes.
func TestUpdateProgramState_ManifestIsUpdatedOnDisk(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "PROGRAM-state.yaml")
	if err := os.WriteFile(manifestPath, []byte(programStateTestManifestYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newUpdateProgramStateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{manifestPath, "--state", "TIER_EXECUTING"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Re-read the manifest from disk to verify the update was persisted
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "TIER_EXECUTING") {
		t.Errorf("expected TIER_EXECUTING in updated manifest, got:\n%s", content)
	}
}
