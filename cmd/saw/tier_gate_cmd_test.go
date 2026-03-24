package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestTierGateCmd_Pass tests a tier gate that passes (all IMPLs complete, no gates defined).
func TestTierGateCmd_Pass(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid PROGRAM manifest with tier 1 complete
	manifestContent := `title: Test Program
program_slug: test-program
state: EXECUTING
impls:
  - slug: impl-a
    title: Implementation A
    tier: 1
    status: complete
  - slug: impl-b
    title: Implementation B
    tier: 2
    status: pending
tiers:
  - number: 1
    impls:
      - impl-a
  - number: 2
    impls:
      - impl-b
completion:
  tiers_complete: 1
  tiers_total: 2
  impls_complete: 1
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
	cmd := newTierGateCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath, "--tier", "1", "--repo-dir", tmpDir})

	// Execute - should succeed (exit 0)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	// Parse JSON output
	var result protocol.TierGateData
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify passed: true
	if !result.Passed {
		t.Errorf("expected passed: true, got false. Result: %+v", result)
	}

	// Verify tier number
	if result.TierNumber != 1 {
		t.Errorf("expected tier_number: 1, got %d", result.TierNumber)
	}

	// Verify all IMPLs done
	if !result.AllImplsDone {
		t.Errorf("expected all_impls_done: true, got false")
	}

	// Verify IMPL status
	if len(result.ImplStatuses) != 1 {
		t.Errorf("expected 1 IMPL status, got %d", len(result.ImplStatuses))
	} else {
		if result.ImplStatuses[0].Slug != "impl-a" {
			t.Errorf("expected IMPL slug 'impl-a', got '%s'", result.ImplStatuses[0].Slug)
		}
		if result.ImplStatuses[0].Status != "complete" {
			t.Errorf("expected IMPL status 'complete', got '%s'", result.ImplStatuses[0].Status)
		}
	}
}

// TestTierGateCmd_Fail tests a tier gate that fails (IMPLs not complete).
func TestTierGateCmd_Fail(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PROGRAM manifest with tier 1 incomplete
	manifestContent := `title: Test Program
program_slug: test-program
state: EXECUTING
impls:
  - slug: impl-a
    title: Implementation A
    tier: 1
    status: in-progress
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
	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Skip this test because os.Exit(1) will kill the test runner
	t.Skip("Cannot test os.Exit(1) behavior in unit tests - requires integration test")
}

// TestTierGateCmd_InvalidTier tests handling of tier number not found.
func TestTierGateCmd_InvalidTier(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid PROGRAM manifest
	manifestContent := `title: Test Program
program_slug: test-program
state: EXECUTING
impls:
  - slug: impl-a
    title: Implementation A
    tier: 1
    status: complete
tiers:
  - number: 1
    impls:
      - impl-a
completion:
  tiers_complete: 1
  tiers_total: 1
  impls_complete: 1
  impls_total: 1
  total_agents: 0
  total_waves: 0
`
	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Skip this test because os.Exit(2) will kill the test runner
	t.Skip("Cannot test os.Exit(2) behavior in unit tests - requires integration test")
}

// TestTierGateCmd_NoTierFlag tests error handling when --tier flag is missing.
func TestTierGateCmd_NoTierFlag(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")

	// Capture stdout/stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command without --tier flag
	cmd := newTierGateCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})

	// Execute - should fail due to missing required flag
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --tier flag is missing, got nil")
	}
}
