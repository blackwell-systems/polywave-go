package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestFreezeContractsCmd_Success tests successful contract freezing.
func TestFreezeContractsCmd_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a contract source file
	contractFile := filepath.Join(tmpDir, "pkg", "types", "contract.go")
	if err := os.MkdirAll(filepath.Dir(contractFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractFile, []byte("package types\n\ntype Contract struct{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize git repo and commit the file
	initGitRepo(t, tmpDir)
	commitFile(t, tmpDir, "pkg/types/contract.go", "Add contract")

	// Create a PROGRAM manifest with a contract that should freeze at tier 1
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
program_contracts:
  - name: ContractInterface
    location: pkg/types/contract.go
    freeze_at: impl-a
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

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newFreezeContractsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath, "--tier", "1", "--repo-dir", tmpDir})

	// Execute - should succeed (exit 0)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	// Parse JSON output
	var result protocol.FreezeContractsData
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify success: true
	if !result.Success {
		t.Errorf("expected success: true, got false. Errors: %+v", result.Errors)
	}

	// Verify contracts frozen
	if len(result.ContractsFrozen) != 1 {
		t.Fatalf("expected 1 frozen contract, got %d", len(result.ContractsFrozen))
	}

	contract := result.ContractsFrozen[0]
	if contract.Name != "ContractInterface" {
		t.Errorf("expected contract name 'ContractInterface', got '%s'", contract.Name)
	}
	if !contract.FileExists {
		t.Errorf("expected file_exists: true, got false")
	}
	if !contract.Committed {
		t.Errorf("expected committed: true, got false")
	}
}

// TestFreezeContractsCmd_MissingFile tests handling of contract with missing file.
func TestFreezeContractsCmd_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo (but don't create the contract file)
	initGitRepo(t, tmpDir)

	// Create a PROGRAM manifest with a contract that doesn't exist
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
program_contracts:
  - name: MissingContract
    location: pkg/types/missing.go
    freeze_at: impl-a
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newFreezeContractsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath, "--tier", "1", "--repo-dir", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing contract file, got nil")
	}
}

// TestFreezeContractsCmd_UncommittedFile tests handling of uncommitted contract.
func TestFreezeContractsCmd_UncommittedFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a contract source file but don't commit it
	contractFile := filepath.Join(tmpDir, "pkg", "types", "contract.go")
	if err := os.MkdirAll(filepath.Dir(contractFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractFile, []byte("package types\n\ntype Contract struct{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize git repo but don't commit the contract file
	initGitRepo(t, tmpDir)

	// Create a PROGRAM manifest
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
program_contracts:
  - name: UncommittedContract
    location: pkg/types/contract.go
    freeze_at: impl-a
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newFreezeContractsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath, "--tier", "1", "--repo-dir", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for uncommitted contract file, got nil")
	}
}

// Helper: initialize a git repository
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runCommand(t, dir, "git", "init")
	runCommand(t, dir, "git", "config", "user.email", "test@example.com")
	runCommand(t, dir, "git", "config", "user.name", "Test User")
}

// Helper: commit a file to git
func commitFile(t *testing.T, dir, filePath, message string) {
	t.Helper()
	runCommand(t, dir, "git", "add", filePath)
	runCommand(t, dir, "git", "commit", "-m", message)
}

// Helper: run a command in a directory
func runCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("command %s %v failed: %v\noutput: %s", name, args, err, string(output))
	}
}
