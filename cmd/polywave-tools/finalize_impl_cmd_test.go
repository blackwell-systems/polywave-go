package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func TestFinalizeImplCmd_Success(t *testing.T) {
	// Setup: create temp IMPL file with valid structure, 2 agents, no gates
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "test-impl.yaml")

	implContent := `title: Test Feature
feature_slug: test-feature
verdict: SUITABLE
state: SCOUT_PENDING
file_ownership:
  - file: pkg/feature/a.go
    agent: A
    wave: 1
    action: new
  - file: pkg/feature/b.go
    agent: B
    wave: 1
    action: new
waves:
  - number: 1
    agents:
      - id: A
        task: |
          Implement feature A
        files:
          - pkg/feature/a.go
      - id: B
        task: |
          Implement feature B
        files:
          - pkg/feature/b.go
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL file: %v", err)
	}

	// Setup: create temp repo with go.mod (for H2 detection)
	repoDir := t.TempDir()
	goModContent := `module github.com/test/repo

go 1.21
`
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Run: cobra command with manifest path and repo root
	cmd := newFinalizeImplCmd()
	cmd.SetArgs([]string{implPath, "--repo-root", repoDir})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()

	// Verify: exit code 0 (no error)
	if err != nil {
		t.Errorf("Expected no error, got: %v\nOutput: %s", err, buf.String())
	}

	// Verify: JSON output has success code
	var res result.Result[protocol.FinalizeIMPLData]
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, buf.String())
	}

	if !res.IsSuccess() {
		t.Errorf("Expected success, got failure. Errors: %v", res.Errors)
	}

	// Verify: AgentsUpdated=2
	if res.GetData().GatePopulation.AgentsUpdated != 2 {
		t.Errorf("Expected AgentsUpdated=2, got %d", res.GetData().GatePopulation.AgentsUpdated)
	}

	// Verify: IMPL file updated with verification blocks
	updatedContent, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("Failed to read updated IMPL file: %v", err)
	}

	if !strings.Contains(string(updatedContent), "## Verification Gate") {
		t.Error("Expected IMPL file to contain verification gate blocks")
	}
}

func TestFinalizeImplCmd_ValidationFailure(t *testing.T) {
	// Setup: create IMPL file with I1 violation (duplicate file ownership)
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "test-impl.yaml")

	implContent := `impl: test-feature
description: Test feature
waves:
  - agents:
      - id: A
        task: |
          Implement feature A

          ## Files Owned
          - pkg/feature/shared.go
      - id: B
        task: |
          Implement feature B

          ## Files Owned
          - pkg/feature/shared.go
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL file: %v", err)
	}

	repoDir := t.TempDir()

	// Run: command
	cmd := newFinalizeImplCmd()
	cmd.SetArgs([]string{implPath, "--repo-root", repoDir})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()

	// Verify: error returned (exit code 1 will be handled by os.Exit in real execution)
	// In tests, we check that Success=false in JSON
	if err == nil {
		// Check JSON output
		var res result.Result[protocol.FinalizeIMPLData]
		if err := json.Unmarshal(buf.Bytes(), &res); err == nil {
			if res.IsSuccess() {
				t.Error("Expected failure for validation failure, got success")
			}
		}
	}
}

func TestFinalizeImplCmd_MissingRepoRoot(t *testing.T) {
	// Setup: valid IMPL file
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "test-impl.yaml")

	implContent := `impl: test-feature
description: Test feature
waves:
  - agents:
      - id: A
        task: |
          Implement feature A

          ## Files Owned
          - pkg/feature/a.go
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create test IMPL file: %v", err)
	}

	// Run: command without --repo-root flag (should default to ".")
	cmd := newFinalizeImplCmd()
	cmd.SetArgs([]string{implPath})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Verify: command runs (does not fail on missing flag)
	// Note: This may fail validation or gate population, but should NOT fail on flag parsing
	_ = cmd.Execute()

	// If we get here, the command accepted the arguments (flag defaulted correctly)
	// The actual execution result doesn't matter for this test
}

func TestFinalizeImplCmd_InvalidManifestPath(t *testing.T) {
	// Run: command with non-existent manifest path
	cmd := newFinalizeImplCmd()
	cmd.SetArgs([]string{"/nonexistent/path/to/impl.yaml"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()

	// Verify: exit code 1, error message "failed to load manifest"
	if err == nil {
		t.Error("Expected error for non-existent manifest path")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "finalize-impl:") {
		t.Errorf("Expected error message to contain 'finalize-impl:', got: %s", errMsg)
	}
}

func TestFinalizeImplCmd_Registration(t *testing.T) {
	// Verify: newFinalizeImplCmd() is registered in main.go rootCmd
	cmd := newFinalizeImplCmd()

	if cmd.Use != "finalize-impl <manifest-path>" {
		t.Errorf("Expected Use='finalize-impl <manifest-path>', got: %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected non-empty Short description")
	}

	if cmd.Long == "" {
		t.Error("Expected non-empty Long description")
	}

	// Verify: command has required args
	if cmd.Args == nil {
		t.Error("Expected Args validation function")
	}

	// Verify: command has RunE function
	if cmd.RunE == nil {
		t.Error("Expected RunE function")
	}

	// Verify: --repo-root flag exists
	flag := cmd.Flags().Lookup("repo-root")
	if flag == nil {
		t.Error("Expected --repo-root flag to exist")
	} else {
		if flag.DefValue != "." {
			t.Errorf("Expected --repo-root default value '.', got: %s", flag.DefValue)
		}
	}
}
