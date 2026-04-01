package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// TypeScriptWorkspaceManager implements WorkspaceManager for TypeScript/JavaScript repos.
// For TypeScript repos (tsconfig.json), it uses project references.
// For JavaScript-only repos (package.json only), it adds worktree paths to workspaces.
type TypeScriptWorkspaceManager struct{}

func (m *TypeScriptWorkspaceManager) Language() string {
	return "typescript"
}

// Detect returns true if tsconfig.json or package.json exists at repoRoot.
func (m *TypeScriptWorkspaceManager) Detect(repoRoot string) bool {
	if fileExists(filepath.Join(repoRoot, "tsconfig.json")) {
		return true
	}
	if fileExists(filepath.Join(repoRoot, "package.json")) {
		return true
	}
	return false
}

// Setup modifies workspace configuration to include all worktreePaths for LSP resolution.
// If tsconfig.json exists, uses TypeScript project references.
// Otherwise if package.json exists, adds paths to workspaces array.
func (m *TypeScriptWorkspaceManager) Setup(repoRoot string, waveNum int, worktreePaths []string) error {
	backupDir := BackupDir(repoRoot, waveNum)

	if fileExists(filepath.Join(repoRoot, "tsconfig.json")) {
		return m.setupTypeScript(repoRoot, backupDir, worktreePaths)
	}
	if fileExists(filepath.Join(repoRoot, "package.json")) {
		return m.setupJavaScript(repoRoot, backupDir, worktreePaths)
	}
	return nil
}

func (m *TypeScriptWorkspaceManager) setupTypeScript(repoRoot, backupDir string, worktreePaths []string) error {
	tsconfigPath := filepath.Join(repoRoot, "tsconfig.json")

	// 1. Create backup directory
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}

	// 2. Back up tsconfig.json
	data, err := os.ReadFile(tsconfigPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(backupDir, "tsconfig.json.backup"), data, 0o644); err != nil {
		return err
	}

	// 3. For each worktreePath, create tsconfig.worktree.json
	for _, worktreePath := range worktreePaths {
		rel, err := filepath.Rel(worktreePath, filepath.Join(repoRoot, "tsconfig.json"))
		if err != nil {
			return err
		}
		wtConfig := map[string]interface{}{
			"extends": rel,
			"compilerOptions": map[string]interface{}{
				"composite": true,
			},
		}
		wtData, err := json.MarshalIndent(wtConfig, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(worktreePath, "tsconfig.worktree.json"), wtData, 0o644); err != nil {
			return err
		}
	}

	// 4. Parse root tsconfig.json and append to "references" array
	var tsconfig map[string]interface{}
	if err := json.Unmarshal(data, &tsconfig); err != nil {
		return err
	}

	refs, _ := tsconfig["references"].([]interface{})
	if refs == nil {
		refs = []interface{}{}
	}
	for _, worktreePath := range worktreePaths {
		rel, err := filepath.Rel(repoRoot, worktreePath)
		if err != nil {
			return err
		}
		refs = append(refs, map[string]interface{}{"path": rel})
	}
	tsconfig["references"] = refs

	// 5. Marshal and write back
	out, err := json.MarshalIndent(tsconfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tsconfigPath, out, 0o644)
}

func (m *TypeScriptWorkspaceManager) setupJavaScript(repoRoot, backupDir string, worktreePaths []string) error {
	pkgPath := filepath.Join(repoRoot, "package.json")

	// 1. Create backup directory
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}

	// 2. Back up package.json
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(backupDir, "package.json.backup"), data, 0o644); err != nil {
		return err
	}

	// 3. Parse package.json and append relative paths to "workspaces" array
	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return err
	}

	workspaces, _ := pkg["workspaces"].([]interface{})
	if workspaces == nil {
		workspaces = []interface{}{}
	}
	for _, worktreePath := range worktreePaths {
		rel, err := filepath.Rel(repoRoot, worktreePath)
		if err != nil {
			return err
		}
		workspaces = append(workspaces, rel)
	}
	pkg["workspaces"] = workspaces

	out, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pkgPath, out, 0o644)
}

// Restore reverts workspace configuration to its pre-wave state.
// If tsconfig.json.backup exists, restores tsconfig.json and deletes backup.
// If package.json.backup exists, restores package.json and deletes backup.
func (m *TypeScriptWorkspaceManager) Restore(repoRoot string, waveNum int) error {
	backupDir := BackupDir(repoRoot, waveNum)

	tsconfigBackup := filepath.Join(backupDir, "tsconfig.json.backup")
	if fileExists(tsconfigBackup) {
		data, err := os.ReadFile(tsconfigBackup)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "tsconfig.json"), data, 0o644); err != nil {
			return err
		}
		if err := os.Remove(tsconfigBackup); err != nil {
			return err
		}
	}

	pkgBackup := filepath.Join(backupDir, "package.json.backup")
	if fileExists(pkgBackup) {
		data, err := os.ReadFile(pkgBackup)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "package.json"), data, 0o644); err != nil {
			return err
		}
		if err := os.Remove(pkgBackup); err != nil {
			return err
		}
	}

	return nil
}

// fileExists returns true if the path exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
