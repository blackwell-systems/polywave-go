package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRustManager_Detect_TrueWithWorkspaceSection verifies that Detect returns true
// when Cargo.toml exists and contains a [workspace] section.
func TestRustManager_Detect_TrueWithWorkspaceSection(t *testing.T) {
	dir := t.TempDir()
	cargoPath := filepath.Join(dir, "Cargo.toml")
	content := "[workspace]\nmembers = [\n  \"crate_a\",\n]\n"
	if err := os.WriteFile(cargoPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := &RustWorkspaceManager{}
	if !m.Detect(dir) {
		t.Error("expected Detect to return true for Cargo.toml with [workspace]")
	}
}

// TestRustManager_Detect_FalseWithoutWorkspaceSection verifies that Detect returns
// false when Cargo.toml exists but has no [workspace] section.
func TestRustManager_Detect_FalseWithoutWorkspaceSection(t *testing.T) {
	dir := t.TempDir()
	cargoPath := filepath.Join(dir, "Cargo.toml")
	content := "[package]\nname = \"my_crate\"\nversion = \"0.1.0\"\n"
	if err := os.WriteFile(cargoPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := &RustWorkspaceManager{}
	if m.Detect(dir) {
		t.Error("expected Detect to return false for Cargo.toml without [workspace]")
	}
}

// TestRustManager_Detect_FalseNoFile verifies that Detect returns false when
// Cargo.toml does not exist.
func TestRustManager_Detect_FalseNoFile(t *testing.T) {
	dir := t.TempDir()

	m := &RustWorkspaceManager{}
	if m.Detect(dir) {
		t.Error("expected Detect to return false when Cargo.toml does not exist")
	}
}

// TestRustManager_Setup_BacksUpCargoToml verifies that Setup creates a backup of
// the original Cargo.toml before modification.
func TestRustManager_Setup_BacksUpCargoToml(t *testing.T) {
	dir := t.TempDir()
	cargoPath := filepath.Join(dir, "Cargo.toml")
	original := "[workspace]\nmembers = [\n  \"crate_a\",\n]\n"
	if err := os.WriteFile(cargoPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	worktreePath := filepath.Join(dir, ".claude", "worktrees", "wave1-agent-A")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatal(err)
	}

	m := &RustWorkspaceManager{}
	if err := m.Setup(dir, 1, []string{worktreePath}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	backupPath := filepath.Join(BackupDir(dir, 1), "Cargo.toml.backup")
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup not found at %s: %v", backupPath, err)
	}
	if string(backupContent) != original {
		t.Errorf("backup content mismatch\ngot:  %q\nwant: %q", string(backupContent), original)
	}
}

// TestRustManager_Setup_AddsMembers_MultiLine verifies that Setup correctly inserts
// new member paths into a multi-line members array.
func TestRustManager_Setup_AddsMembers_MultiLine(t *testing.T) {
	dir := t.TempDir()
	cargoPath := filepath.Join(dir, "Cargo.toml")
	original := "[workspace]\nmembers = [\n  \"crate_a\",\n]\n"
	if err := os.WriteFile(cargoPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	// Worktree path nested inside the repo root.
	worktreePath := filepath.Join(dir, ".claude", "worktrees", "wave1-agent-A")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatal(err)
	}

	m := &RustWorkspaceManager{}
	if err := m.Setup(dir, 1, []string{worktreePath}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	modifiedBytes, err := os.ReadFile(cargoPath)
	if err != nil {
		t.Fatalf("failed to read modified Cargo.toml: %v", err)
	}
	modified := string(modifiedBytes)

	// Verify relative path is present.
	rel, err := filepath.Rel(dir, worktreePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(modified, rel) {
		t.Errorf("expected Cargo.toml to contain %q\ngot:\n%s", rel, modified)
	}

	// Verify SAW marker is present.
	if !strings.Contains(modified, "SAW-managed") {
		t.Errorf("expected Cargo.toml to contain SAW-managed comment\ngot:\n%s", modified)
	}

	// Verify original member is still present.
	if !strings.Contains(modified, "crate_a") {
		t.Errorf("expected original member crate_a to still be present\ngot:\n%s", modified)
	}
}

// TestRustManager_Restore_FromBackup verifies that Restore overwrites Cargo.toml
// with the backup content and removes the backup file.
func TestRustManager_Restore_FromBackup(t *testing.T) {
	dir := t.TempDir()
	cargoPath := filepath.Join(dir, "Cargo.toml")
	original := "[workspace]\nmembers = [\n  \"crate_a\",\n]\n"
	if err := os.WriteFile(cargoPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	worktreePath := filepath.Join(dir, ".claude", "worktrees", "wave1-agent-A")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatal(err)
	}

	m := &RustWorkspaceManager{}

	// Run Setup to modify Cargo.toml and create a backup.
	if err := m.Setup(dir, 1, []string{worktreePath}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Verify Cargo.toml was modified.
	afterSetup, _ := os.ReadFile(cargoPath)
	if string(afterSetup) == original {
		t.Error("Cargo.toml should have been modified by Setup")
	}

	// Run Restore.
	if err := m.Restore(dir, 1); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify Cargo.toml is restored.
	afterRestore, err := os.ReadFile(cargoPath)
	if err != nil {
		t.Fatalf("failed to read Cargo.toml after restore: %v", err)
	}
	if string(afterRestore) != original {
		t.Errorf("Cargo.toml not restored correctly\ngot:  %q\nwant: %q", string(afterRestore), original)
	}

	// Verify backup was deleted.
	backupPath := filepath.Join(BackupDir(dir, 1), "Cargo.toml.backup")
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Errorf("expected backup to be deleted after restore, but it still exists at %s", backupPath)
	}
}
