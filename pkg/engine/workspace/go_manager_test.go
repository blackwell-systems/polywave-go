package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGoWorkspaceManager_Detect_TrueWhenGoModExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &GoWorkspaceManager{}
	if !m.Detect(dir) {
		t.Error("expected Detect to return true when go.mod exists")
	}
}

func TestGoWorkspaceManager_Detect_FalseWhenNoGoMod(t *testing.T) {
	dir := t.TempDir()

	m := &GoWorkspaceManager{}
	if m.Detect(dir) {
		t.Error("expected Detect to return false when go.mod does not exist")
	}
}

func TestGoWorkspaceManager_Setup_BacksUpExistingGoWork(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not found in PATH")
	}

	dir := t.TempDir()
	// Create go.mod
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an existing go.work with known content
	originalContent := []byte("go 1.21\n\nuse .\n")
	if err := os.WriteFile(filepath.Join(dir, "go.work"), originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	m := &GoWorkspaceManager{}
	// Use a worktree dir with a different module to avoid early return
	wtDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(wtDir, "go.mod"), []byte("module example.com/bar\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.Setup(dir, 1, []string{wtDir}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Verify backup was created with original content
	backupPath := filepath.Join(BackupDir(dir, 1), "go.work.backup")
	got, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup file not found: %v", err)
	}
	if string(got) != string(originalContent) {
		t.Errorf("backup content mismatch: got %q, want %q", string(got), string(originalContent))
	}
}

func TestGoWorkspaceManager_Restore_RestoresFromBackup(t *testing.T) {
	dir := t.TempDir()

	// Create backup directory and backup file
	backupDir := BackupDir(dir, 1)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}

	originalContent := []byte("go 1.21\n\nuse .\n// original\n")
	backupPath := filepath.Join(backupDir, "go.work.backup")
	if err := os.WriteFile(backupPath, originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a current go.work that should be replaced
	goWorkPath := filepath.Join(dir, "go.work")
	if err := os.WriteFile(goWorkPath, []byte("go 1.21\n\nuse .\n// SAW-managed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &GoWorkspaceManager{}
	if err := m.Restore(dir, 1); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify go.work was restored to original content
	got, err := os.ReadFile(goWorkPath)
	if err != nil {
		t.Fatalf("go.work not found after restore: %v", err)
	}
	if string(got) != string(originalContent) {
		t.Errorf("restored content mismatch: got %q, want %q", string(got), string(originalContent))
	}

	// Verify backup was removed
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("expected backup file to be deleted after restore")
	}
}

func TestGoWorkspaceManager_Restore_DeletesGoWorkWhenNoBackup(t *testing.T) {
	dir := t.TempDir()

	// Create a go.work file (no backup exists)
	goWorkPath := filepath.Join(dir, "go.work")
	if err := os.WriteFile(goWorkPath, []byte("go 1.21\n\nuse .\n// SAW-managed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &GoWorkspaceManager{}
	if err := m.Restore(dir, 1); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify go.work was deleted
	if _, err := os.Stat(goWorkPath); !os.IsNotExist(err) {
		t.Error("expected go.work to be deleted when no backup exists")
	}
}

func TestGoWorkspaceManager_Setup_SkipsSameModuleWorktrees(t *testing.T) {
	dir := t.TempDir()

	// Create go.mod with module example.com/foo
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a fake worktree dir with the SAME module path
	wtDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(wtDir, "go.mod"), []byte("module example.com/foo\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &GoWorkspaceManager{}
	if err := m.Setup(dir, 1, []string{wtDir}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Verify go.work was NOT created (early return because same module)
	goWorkPath := filepath.Join(dir, "go.work")
	if _, err := os.Stat(goWorkPath); !os.IsNotExist(err) {
		t.Error("expected go.work NOT to be created when all worktrees share the root module path")
	}
}
