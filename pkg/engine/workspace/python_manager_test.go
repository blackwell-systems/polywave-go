package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPythonManager_Detect_TruePyrightConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyrightconfig.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	m := &PythonWorkspaceManager{}
	if !m.Detect(dir) {
		t.Error("expected Detect=true for pyrightconfig.json, got false")
	}
}

func TestPythonManager_Detect_TruePyprojectToml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[build-system]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := &PythonWorkspaceManager{}
	if !m.Detect(dir) {
		t.Error("expected Detect=true for pyproject.toml, got false")
	}
}

func TestPythonManager_Detect_False(t *testing.T) {
	dir := t.TempDir()
	m := &PythonWorkspaceManager{}
	if m.Detect(dir) {
		t.Error("expected Detect=false for empty dir, got true")
	}
}

func TestPythonManager_Setup_PyrightConfig_BacksUpAndAddsExtraPaths(t *testing.T) {
	dir := t.TempDir()
	wt := filepath.Join(dir, "worktrees", "agent-A")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}

	original := map[string]interface{}{
		"extraPaths": []interface{}{"existing"},
	}
	data, _ := json.MarshalIndent(original, "", "  ")
	pyrightPath := filepath.Join(dir, "pyrightconfig.json")
	if err := os.WriteFile(pyrightPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	m := &PythonWorkspaceManager{}
	if err := m.Setup(dir, 1, []string{wt}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Verify backup exists.
	backupPath := filepath.Join(BackupDir(dir, 1), "pyrightconfig.json.backup")
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup not created: %v", err)
	}

	// Verify extraPaths contains both existing and new.
	updated, err := os.ReadFile(pyrightPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(updated, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	eps, ok := cfg["extraPaths"].([]interface{})
	if !ok {
		t.Fatal("extraPaths missing or wrong type")
	}
	found := map[string]bool{}
	for _, p := range eps {
		found[p.(string)] = true
	}
	if !found["existing"] {
		t.Error("original 'existing' path missing from extraPaths")
	}

	rel, _ := filepath.Rel(dir, wt)
	if !found[rel] {
		t.Errorf("new relative path %q missing from extraPaths", rel)
	}
}

func TestPythonManager_Setup_CreatesPyrightConfigFromScratch(t *testing.T) {
	// pyproject.toml only (no pyrightconfig.json): should take pyproject.toml path,
	// create backup for pyproject.toml, and NOT create pyrightconfig.json.
	dir := t.TempDir()
	wt := filepath.Join(dir, "worktrees", "agent-B")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}

	pyprojectPath := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte("[build-system]\nrequires = []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &PythonWorkspaceManager{}
	if err := m.Setup(dir, 2, []string{wt}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// pyrightconfig.json should NOT be created (pyproject.toml path was taken).
	pyrightPath := filepath.Join(dir, "pyrightconfig.json")
	if _, err := os.Stat(pyrightPath); err == nil {
		t.Error("pyrightconfig.json should not have been created when pyproject.toml was present")
	}

	// pyproject.toml backup should exist.
	backupPath := filepath.Join(BackupDir(dir, 2), "pyproject.toml.backup")
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("pyproject.toml backup not created: %v", err)
	}
}

func TestPythonManager_Restore_FromBackup_PyRight(t *testing.T) {
	dir := t.TempDir()

	// Set up a SAW-modified pyrightconfig.json and its backup.
	pyrightPath := filepath.Join(dir, "pyrightconfig.json")
	modified := []byte(`{"extraPaths":["worktree1","worktree2"]}`)
	if err := os.WriteFile(pyrightPath, modified, 0644); err != nil {
		t.Fatal(err)
	}

	backupDir := BackupDir(dir, 3)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"extraPaths":[]}`)
	backupPath := filepath.Join(backupDir, "pyrightconfig.json.backup")
	if err := os.WriteFile(backupPath, original, 0644); err != nil {
		t.Fatal(err)
	}

	m := &PythonWorkspaceManager{}
	if err := m.Restore(dir, 3); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// pyrightconfig.json should contain original content.
	restored, err := os.ReadFile(pyrightPath)
	if err != nil {
		t.Fatalf("pyrightconfig.json missing after restore: %v", err)
	}
	if string(restored) != string(original) {
		t.Errorf("restored content = %q, want %q", restored, original)
	}

	// Backup should be deleted.
	if _, err := os.Stat(backupPath); err == nil {
		t.Error("backup should have been deleted after restore")
	}
}

func TestPythonManager_Restore_DeletesCreatedPyrightConfig(t *testing.T) {
	dir := t.TempDir()

	// pyrightconfig.json exists but no backup (SAW created it from scratch).
	pyrightPath := filepath.Join(dir, "pyrightconfig.json")
	if err := os.WriteFile(pyrightPath, []byte(`{"extraPaths":["wt"]}`), 0644); err != nil {
		t.Fatal(err)
	}

	m := &PythonWorkspaceManager{}
	if err := m.Restore(dir, 4); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// pyrightconfig.json should be deleted.
	if _, err := os.Stat(pyrightPath); err == nil {
		t.Error("pyrightconfig.json should have been deleted by Restore")
	}
}
