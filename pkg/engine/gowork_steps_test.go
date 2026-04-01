package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestStepGoWorkSetup_SkipsNonGoRepo verifies that the step skips when go.mod is absent.
func TestStepGoWorkSetup_SkipsNonGoRepo(t *testing.T) {
	repoRoot := t.TempDir()

	result := StepGoWorkSetup(context.Background(), repoRoot, 1, nil, nil, nil)

	if result.Status != "success" {
		t.Errorf("expected status=success, got %q", result.Status)
	}
	if !strings.Contains(result.Detail, "not a Go repo") {
		t.Errorf("expected detail to contain 'not a Go repo', got %q", result.Detail)
	}

	// go.work should NOT exist
	if _, err := os.Stat(filepath.Join(repoRoot, "go.work")); !os.IsNotExist(err) {
		t.Error("expected go.work to not be created for non-Go repo")
	}
}

// TestStepGoWorkSetup_CreatesGoWork verifies go.work creation when go.mod is present.
func TestStepGoWorkSetup_CreatesGoWork(t *testing.T) {
	// Check that the go binary is available
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	repoRoot := t.TempDir()

	// Write a minimal go.mod
	goModContent := "module example.com/testrepo\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create fake worktree dirs so `go work use` can find them
	wt1 := filepath.Join(repoRoot, "wt1")
	wt2 := filepath.Join(repoRoot, "wt2")
	for _, p := range []string{wt1, wt2} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatalf("failed to create worktree dir: %v", err)
		}
		// Each worktree needs a go.mod for `go work use` to succeed
		if err := os.WriteFile(filepath.Join(p, "go.mod"), []byte("module example.com/wt\n\ngo 1.21\n"), 0644); err != nil {
			t.Fatalf("failed to write go.mod in worktree: %v", err)
		}
	}

	worktrees := []protocol.WorktreeInfo{
		{Agent: "A", Path: wt1, Branch: "wave1-agent-A"},
		{Agent: "B", Path: wt2, Branch: "wave1-agent-B"},
	}

	result := StepGoWorkSetup(context.Background(), repoRoot, 1, worktrees, nil, nil)

	// go work init may fail if environment is restricted; non-fatal means status is warning or success
	if result.Status != "success" && result.Status != "warning" {
		t.Errorf("expected status=success or warning, got %q (detail: %s)", result.Status, result.Detail)
	}

	// If success, go.work should exist
	if result.Status == "success" {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.work")); os.IsNotExist(err) {
			t.Error("expected go.work to be created")
		}
	}
}

// TestStepGoWorkSetup_BacksUpExistingGoWork verifies backup of a pre-existing go.work.
func TestStepGoWorkSetup_BacksUpExistingGoWork(t *testing.T) {
	repoRoot := t.TempDir()

	// Write go.mod so it's treated as a Go repo
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Write a pre-existing go.work
	originalContent := "// original"
	if err := os.WriteFile(filepath.Join(repoRoot, "go.work"), []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to write go.work: %v", err)
	}

	onEvent, _ := collectStepEvents()
	StepGoWorkSetup(context.Background(), repoRoot, 1, nil, onEvent, nil)

	// Verify backup was created with original content
	backupPath := filepath.Join(protocol.SAWStateDir(repoRoot), "wave1", "go.work.backup")
	content, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("expected backup file to exist at %s: %v", backupPath, err)
	}
	if string(content) != originalContent {
		t.Errorf("expected backup content %q, got %q", originalContent, string(content))
	}
}

// TestStepGoWorkRestore_RestoresFromBackup verifies that go.work is restored from backup.
func TestStepGoWorkRestore_RestoresFromBackup(t *testing.T) {
	repoRoot := t.TempDir()

	// Create the backup directory and backup file
	backupDir := filepath.Join(protocol.SAWStateDir(repoRoot), "wave1")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("failed to create backup dir: %v", err)
	}
	backupContent := "// backed-up"
	backupPath := filepath.Join(backupDir, "go.work.backup")
	if err := os.WriteFile(backupPath, []byte(backupContent), 0644); err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	result := StepGoWorkRestore(context.Background(), repoRoot, 1, nil, nil)

	if result.Status != "success" {
		t.Errorf("expected status=success, got %q (detail: %s)", result.Status, result.Detail)
	}

	// Verify go.work at repoRoot has the restored content
	goWorkPath := filepath.Join(repoRoot, "go.work")
	content, err := os.ReadFile(goWorkPath)
	if err != nil {
		t.Fatalf("expected go.work to be restored at %s: %v", goWorkPath, err)
	}
	if string(content) != backupContent {
		t.Errorf("expected go.work content %q, got %q", backupContent, string(content))
	}

	// Verify backup file is removed
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("expected backup file to be removed after restore")
	}
}

// TestStepGoWorkRestore_DeletesGoWorkWhenNoBackup verifies that go.work is deleted
// when no backup exists (it was created by setup but didn't exist before).
func TestStepGoWorkRestore_DeletesGoWorkWhenNoBackup(t *testing.T) {
	repoRoot := t.TempDir()

	// Create a go.work that the setup would have created
	goWorkPath := filepath.Join(repoRoot, "go.work")
	if err := os.WriteFile(goWorkPath, []byte("// saw-managed go.work"), 0644); err != nil {
		t.Fatalf("failed to write go.work: %v", err)
	}

	result := StepGoWorkRestore(context.Background(), repoRoot, 1, nil, nil)

	if result.Status != "success" {
		t.Errorf("expected status=success, got %q (detail: %s)", result.Status, result.Detail)
	}

	// go.work should be gone
	if _, err := os.Stat(goWorkPath); !os.IsNotExist(err) {
		t.Error("expected go.work to be deleted when no backup exists")
	}
}

// TestStepGoWorkRestore_NilCallback verifies that nil onEvent does not cause a panic.
func TestStepGoWorkRestore_NilCallback(t *testing.T) {
	repoRoot := t.TempDir()

	// Should not panic regardless of state
	result := StepGoWorkRestore(context.Background(), repoRoot, 1, nil, nil)

	if result == nil {
		t.Fatal("expected non-nil StepResult")
	}
}
