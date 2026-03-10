package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyBuild_BothPass(t *testing.T) {
	// Create temporary manifest with passing commands
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL.yaml")

	manifestContent := `
title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
test_command: "true"
lint_command: "true"
file_ownership: []
waves: []
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	// Run VerifyBuild
	result, err := VerifyBuild(manifestPath, tmpDir)
	if err != nil {
		t.Fatalf("VerifyBuild failed: %v", err)
	}

	// Verify both commands passed
	if !result.TestPassed {
		t.Errorf("expected TestPassed=true, got false. Output: %s", result.TestOutput)
	}
	if !result.LintPassed {
		t.Errorf("expected LintPassed=true, got false. Output: %s", result.LintOutput)
	}

	// Verify commands are captured
	if result.TestCommand != "true" {
		t.Errorf("expected TestCommand='true', got %q", result.TestCommand)
	}
	if result.LintCommand != "true" {
		t.Errorf("expected LintCommand='true', got %q", result.LintCommand)
	}
}

func TestVerifyBuild_TestFails(t *testing.T) {
	// Create temporary manifest with failing test command
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL.yaml")

	manifestContent := `
title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
test_command: "false"
lint_command: "true"
file_ownership: []
waves: []
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	// Run VerifyBuild
	result, err := VerifyBuild(manifestPath, tmpDir)
	if err != nil {
		t.Fatalf("VerifyBuild failed: %v", err)
	}

	// Verify test failed
	if result.TestPassed {
		t.Errorf("expected TestPassed=false, got true")
	}

	// Verify lint passed
	if !result.LintPassed {
		t.Errorf("expected LintPassed=true, got false. Output: %s", result.LintOutput)
	}

	// Verify output is captured (even if empty)
	if result.TestOutput == "" && result.TestPassed == false {
		// This is acceptable: "false" command exits 1 with no output
		t.Logf("TestOutput is empty (expected for 'false' command)")
	}
}

func TestVerifyBuild_EmptyCommand(t *testing.T) {
	// Create temporary manifest with empty lint command
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL.yaml")

	manifestContent := `
title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
test_command: "echo test output"
lint_command: ""
file_ownership: []
waves: []
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	// Run VerifyBuild
	result, err := VerifyBuild(manifestPath, tmpDir)
	if err != nil {
		t.Fatalf("VerifyBuild failed: %v", err)
	}

	// Verify test passed
	if !result.TestPassed {
		t.Errorf("expected TestPassed=true, got false. Output: %s", result.TestOutput)
	}

	// Verify empty lint command was skipped and marked as passed
	if !result.LintPassed {
		t.Errorf("expected LintPassed=true (skipped), got false")
	}

	// Verify lint output is empty (command was skipped)
	if result.LintOutput != "" {
		t.Errorf("expected empty LintOutput (skipped), got %q", result.LintOutput)
	}

	// Verify test output contains "test output"
	if !strings.Contains(result.TestOutput, "test output") {
		t.Errorf("expected TestOutput to contain 'test output', got %q", result.TestOutput)
	}

	// Verify commands are captured
	if result.TestCommand != "echo test output" {
		t.Errorf("expected TestCommand='echo test output', got %q", result.TestCommand)
	}
	if result.LintCommand != "" {
		t.Errorf("expected empty LintCommand, got %q", result.LintCommand)
	}
}
