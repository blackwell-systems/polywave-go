package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreCommitCheck_SilentPass(t *testing.T) {
	// Use a temp dir with no IMPL docs and no polywave.config.json.
	// DiscoverLintGate should return ("", nil) → silent pass.
	tmpDir := t.TempDir()

	err := RunPreCommitCheck(tmpDir)
	if err != nil {
		t.Fatalf("expected silent pass (nil error), got: %v", err)
	}
}

func TestPreCommitCheck_RunsLintGate(t *testing.T) {
	// Create a temp repo with an IMPL doc that defines a lint gate.
	tmpDir := t.TempDir()
	implDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a minimal IMPL doc with a lint gate that always succeeds.
	implContent := `feature: test-lint-gate
status: in_progress
quality_gates:
  level: standard
  gates:
    - type: lint
      command: "true"
      required: true
`
	implPath := filepath.Join(implDir, "IMPL-test-lint.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RunPreCommitCheck(tmpDir)
	if err != nil {
		t.Fatalf("expected lint gate to pass, got: %v", err)
	}
}

func TestPreCommitCheck_FailingGate(t *testing.T) {
	// Create a temp repo with an IMPL doc that defines a failing lint gate.
	tmpDir := t.TempDir()
	implDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0o755); err != nil {
		t.Fatal(err)
	}

	implContent := `feature: test-lint-gate-fail
status: in_progress
quality_gates:
  level: standard
  gates:
    - type: lint
      command: "exit 1"
      required: true
`
	implPath := filepath.Join(implDir, "IMPL-test-fail.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RunPreCommitCheck(tmpDir)
	if err == nil {
		t.Fatal("expected error for failing lint gate, got nil")
	}
}
