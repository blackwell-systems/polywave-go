package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHooks_FreshInstall(t *testing.T) {
	// Create a synthetic repo with .git/hooks/
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := runInstallHooks(tmpDir); err != nil {
		t.Fatalf("runInstallHooks failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook: %v", err)
	}

	hookStr := string(content)

	// Verify shebang
	if !strings.HasPrefix(hookStr, "#!/bin/sh\n") {
		t.Errorf("expected shebang, got: %s", hookStr[:20])
	}

	// Verify set -e
	if !strings.Contains(hookStr, "set -e") {
		t.Error("expected set -e in hook")
	}

	// Verify snippet
	if !strings.Contains(hookStr, "polywave-tools pre-commit-check") {
		t.Error("expected polywave-tools pre-commit-check in hook")
	}

	if !strings.Contains(hookStr, "# Polywave pre-commit quality gate (M4)") {
		t.Error("expected M4 comment in hook")
	}

	// Verify executable
	stat, err := os.Stat(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode()&0111 == 0 {
		t.Error("hook should be executable")
	}
}

func TestInstallHooks_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Install once
	if err := runInstallHooks(tmpDir); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	firstContent, _ := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))

	// Install again
	if err := runInstallHooks(tmpDir); err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	secondContent, _ := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))

	if string(firstContent) != string(secondContent) {
		t.Errorf("idempotency failed: content changed on second install\nfirst:\n%s\nsecond:\n%s",
			firstContent, secondContent)
	}
}

func TestInstallHooks_AppendMode(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write an existing hook
	existingHook := "#!/bin/sh\necho 'existing hook'\n"
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte(existingHook), 0755); err != nil {
		t.Fatal(err)
	}

	if err := runInstallHooks(tmpDir); err != nil {
		t.Fatalf("runInstallHooks failed: %v", err)
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}

	hookStr := string(content)

	// Verify existing content preserved
	if !strings.Contains(hookStr, "echo 'existing hook'") {
		t.Error("existing hook content was not preserved")
	}

	// Verify snippet appended
	if !strings.Contains(hookStr, "polywave-tools pre-commit-check") {
		t.Error("snippet was not appended")
	}

	// Verify existing shebang is not duplicated
	if strings.Count(hookStr, "#!/bin/sh") != 1 {
		t.Error("shebang should not be duplicated in append mode")
	}
}
