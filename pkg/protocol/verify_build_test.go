package protocol

import (
	"context"
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
	res := VerifyBuild(context.Background(), manifestPath, tmpDir)
	if !res.IsSuccess() {
		t.Fatalf("VerifyBuild failed: %v", res.Errors)
	}

	data := res.GetData()

	// Verify both commands passed
	if !data.TestPassed {
		t.Errorf("expected TestPassed=true, got false. Output: %s", data.TestOutput)
	}
	if !data.LintPassed {
		t.Errorf("expected LintPassed=true, got false. Output: %s", data.LintOutput)
	}

	// Verify commands are captured
	if data.TestCommand != "true" {
		t.Errorf("expected TestCommand='true', got %q", data.TestCommand)
	}
	if data.LintCommand != "true" {
		t.Errorf("expected LintCommand='true', got %q", data.LintCommand)
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
	res := VerifyBuild(context.Background(), manifestPath, tmpDir)
	if !res.IsSuccess() {
		t.Fatalf("VerifyBuild failed: %v", res.Errors)
	}

	data := res.GetData()

	// Verify test failed
	if data.TestPassed {
		t.Errorf("expected TestPassed=false, got true")
	}

	// Verify lint passed
	if !data.LintPassed {
		t.Errorf("expected LintPassed=true, got false. Output: %s", data.LintOutput)
	}

	// Verify output is captured (even if empty)
	if data.TestOutput == "" && data.TestPassed == false {
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
	res := VerifyBuild(context.Background(), manifestPath, tmpDir)
	if !res.IsSuccess() {
		t.Fatalf("VerifyBuild failed: %v", res.Errors)
	}

	data := res.GetData()

	// Verify test passed
	if !data.TestPassed {
		t.Errorf("expected TestPassed=true, got false. Output: %s", data.TestOutput)
	}

	// Verify empty lint command was skipped and marked as passed
	if !data.LintPassed {
		t.Errorf("expected LintPassed=true (skipped), got false")
	}

	// Verify lint output is empty (command was skipped)
	if data.LintOutput != "" {
		t.Errorf("expected empty LintOutput (skipped), got %q", data.LintOutput)
	}

	// Verify test output contains "test output"
	if !strings.Contains(data.TestOutput, "test output") {
		t.Errorf("expected TestOutput to contain 'test output', got %q", data.TestOutput)
	}

	// Verify commands are captured
	if data.TestCommand != "echo test output" {
		t.Errorf("expected TestCommand='echo test output', got %q", data.TestCommand)
	}
	if data.LintCommand != "" {
		t.Errorf("expected empty LintCommand, got %q", data.LintCommand)
	}
}
