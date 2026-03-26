package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestProgramStatusCmd_Output tests the program-status command output.
func TestProgramStatusCmd_Output(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid PROGRAM manifest
	manifestContent := `title: Test Program
program_slug: test-program
state: TIER_EXECUTING
impls:
  - slug: impl-a
    title: Implementation A
    tier: 1
    status: complete
  - slug: impl-b
    title: Implementation B
    tier: 2
    status: in-progress
  - slug: impl-c
    title: Implementation C
    tier: 2
    status: pending
tiers:
  - number: 1
    description: Core functionality
    impls:
      - impl-a
  - number: 2
    description: Extended features
    impls:
      - impl-b
      - impl-c
program_contracts:
  - name: CoreContract
    location: pkg/core/interface.go
    freeze_at: impl-a
completion:
  tiers_complete: 1
  tiers_total: 2
  impls_complete: 1
  impls_total: 3
  total_agents: 10
  total_waves: 5
`
	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to tmpDir so GetProgramStatus can use it as repoPath
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newProgramStatusCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})

	// Execute - should always succeed (exit 0)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	// Parse JSON output
	var result protocol.ProgramStatusData
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify program slug
	if result.ProgramSlug != "test-program" {
		t.Errorf("expected program_slug 'test-program', got '%s'", result.ProgramSlug)
	}

	// Verify title
	if result.Title != "Test Program" {
		t.Errorf("expected title 'Test Program', got '%s'", result.Title)
	}

	// Verify state
	if result.State != protocol.ProgramStateTierExecuting {
		t.Errorf("expected state TIER_EXECUTING, got %v", result.State)
	}

	// Verify current tier (should be 2, since tier 1 is complete)
	if result.CurrentTier != 2 {
		t.Errorf("expected current_tier 2, got %d", result.CurrentTier)
	}

	// Verify tier statuses
	if len(result.TierStatuses) != 2 {
		t.Fatalf("expected 2 tier statuses, got %d", len(result.TierStatuses))
	}

	// Verify tier 1 is complete
	tier1 := result.TierStatuses[0]
	if tier1.Number != 1 {
		t.Errorf("expected tier 1, got %d", tier1.Number)
	}
	if !tier1.Complete {
		t.Errorf("expected tier 1 to be complete")
	}
	if len(tier1.ImplStatuses) != 1 {
		t.Errorf("expected 1 IMPL in tier 1, got %d", len(tier1.ImplStatuses))
	}

	// Verify tier 2 is not complete
	tier2 := result.TierStatuses[1]
	if tier2.Number != 2 {
		t.Errorf("expected tier 2, got %d", tier2.Number)
	}
	if tier2.Complete {
		t.Errorf("expected tier 2 to be incomplete")
	}
	if len(tier2.ImplStatuses) != 2 {
		t.Errorf("expected 2 IMPLs in tier 2, got %d", len(tier2.ImplStatuses))
	}

	// Verify contract statuses
	if len(result.ContractStatuses) != 1 {
		t.Fatalf("expected 1 contract status, got %d", len(result.ContractStatuses))
	}
	contract := result.ContractStatuses[0]
	if contract.Name != "CoreContract" {
		t.Errorf("expected contract name 'CoreContract', got '%s'", contract.Name)
	}

	// Verify completion tracking
	if result.Completion.TiersComplete != 1 {
		t.Errorf("expected tiers_complete 1, got %d", result.Completion.TiersComplete)
	}
	if result.Completion.TiersTotal != 2 {
		t.Errorf("expected tiers_total 2, got %d", result.Completion.TiersTotal)
	}
	if result.Completion.ImplsComplete != 1 {
		t.Errorf("expected impls_complete 1, got %d", result.Completion.ImplsComplete)
	}
}

// TestProgramStatusCmd_NoArgs tests error handling when no arguments are provided.
func TestProgramStatusCmd_NoArgs(t *testing.T) {
	// Capture stdout/stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command with no args
	cmd := newProgramStatusCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{})

	// Execute - should fail due to missing required arg
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no manifest file provided, got nil")
	}
}

// TestProgramStatusCmd_ParseError tests handling of parse errors.
func TestProgramStatusCmd_ParseError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid YAML file
	manifestPath := filepath.Join(tmpDir, "PROGRAM-invalid.yaml")
	if err := os.WriteFile(manifestPath, []byte("invalid: yaml: content:\n  - broken"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newProgramStatusCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}
}
