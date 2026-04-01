package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTypeScriptManager_Detect_TsConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &TypeScriptWorkspaceManager{}
	if !m.Detect(dir) {
		t.Error("expected Detect to return true for dir with tsconfig.json")
	}
}

func TestTypeScriptManager_Detect_PackageJson(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &TypeScriptWorkspaceManager{}
	if !m.Detect(dir) {
		t.Error("expected Detect to return true for dir with package.json")
	}
}

func TestTypeScriptManager_Detect_Neither(t *testing.T) {
	dir := t.TempDir()
	m := &TypeScriptWorkspaceManager{}
	if m.Detect(dir) {
		t.Error("expected Detect to return false for empty dir")
	}
}

func TestTypeScriptManager_Setup_TSPath_AddsReferences(t *testing.T) {
	dir := t.TempDir()

	// Create tsconfig.json with compilerOptions but no references
	tsconfigContent := `{"compilerOptions":{}}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create one worktree subdir
	worktreeDir := filepath.Join(dir, "worktrees", "agent-A")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := &TypeScriptWorkspaceManager{}
	if err := m.Setup(dir, 1, []string{worktreeDir}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Verify tsconfig.json has "references" array with one entry
	data, err := os.ReadFile(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tsconfig map[string]interface{}
	if err := json.Unmarshal(data, &tsconfig); err != nil {
		t.Fatalf("failed to parse updated tsconfig.json: %v", err)
	}
	refs, ok := tsconfig["references"].([]interface{})
	if !ok {
		t.Fatal("expected 'references' key in tsconfig.json")
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(refs))
	}

	// Verify backup exists
	backupDir := BackupDir(dir, 1)
	if _, err := os.Stat(filepath.Join(backupDir, "tsconfig.json.backup")); os.IsNotExist(err) {
		t.Error("expected tsconfig.json.backup to exist in backup dir")
	}
}

func TestTypeScriptManager_Setup_JSPath_AddsWorkspaces(t *testing.T) {
	dir := t.TempDir()

	// Create package.json only (no tsconfig.json)
	pkgContent := `{"name":"my-app","version":"1.0.0"}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create one worktree subdir
	worktreeDir := filepath.Join(dir, "worktrees", "agent-A")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := &TypeScriptWorkspaceManager{}
	if err := m.Setup(dir, 1, []string{worktreeDir}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Verify package.json has "workspaces" key with relative path
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatalf("failed to parse updated package.json: %v", err)
	}
	workspaces, ok := pkg["workspaces"].([]interface{})
	if !ok {
		t.Fatal("expected 'workspaces' key in package.json")
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace entry, got %d", len(workspaces))
	}
	rel, err := filepath.Rel(dir, worktreeDir)
	if err != nil {
		t.Fatal(err)
	}
	if workspaces[0] != rel {
		t.Errorf("expected workspace path %q, got %q", rel, workspaces[0])
	}
}

func TestTypeScriptManager_Restore_FromBackup(t *testing.T) {
	dir := t.TempDir()

	// Create backup directory and tsconfig.json.backup
	backupDir := BackupDir(dir, 1)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	originalContent := `{"compilerOptions":{"target":"ES6"}}`
	if err := os.WriteFile(filepath.Join(backupDir, "tsconfig.json.backup"), []byte(originalContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a modified tsconfig.json (as if Setup had run)
	modifiedContent := `{"compilerOptions":{"target":"ES6"},"references":[{"path":"worktrees/agent-A"}]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(modifiedContent), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &TypeScriptWorkspaceManager{}
	if err := m.Restore(dir, 1); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify tsconfig.json is restored to original content
	data, err := os.ReadFile(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != originalContent {
		t.Errorf("expected restored content %q, got %q", originalContent, string(data))
	}

	// Verify backup is deleted
	if _, err := os.Stat(filepath.Join(backupDir, "tsconfig.json.backup")); !os.IsNotExist(err) {
		t.Error("expected tsconfig.json.backup to be deleted after restore")
	}
}
