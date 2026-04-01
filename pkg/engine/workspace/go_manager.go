package workspace

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoWorkspaceManager implements WorkspaceManager for Go repos using go.work.
type GoWorkspaceManager struct{}

// Language returns the canonical name for this manager.
func (m *GoWorkspaceManager) Language() string {
	return "go"
}

// Detect returns true iff go.mod exists at repoRoot.
func (m *GoWorkspaceManager) Detect(repoRoot string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, "go.mod"))
	return err == nil
}

// Setup creates or updates go.work to include all worktreePaths for LSP resolution.
// It backs up existing go.work and go.work.sum before modification.
// Same-module worktrees are filtered out to avoid "module appears multiple times" errors.
func (m *GoWorkspaceManager) Setup(repoRoot string, waveNum int, worktreePaths []string) error {
	// Ensure backup directory exists.
	backupDir := BackupDir(repoRoot, waveNum)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		// Non-fatal: continue even if backup dir creation fails.
		_ = err
	}

	// Back up go.work if it exists.
	goWorkPath := filepath.Join(repoRoot, "go.work")
	goWorkExists := false
	if content, err := os.ReadFile(goWorkPath); err == nil {
		goWorkExists = true
		backupPath := filepath.Join(backupDir, "go.work.backup")
		_ = os.WriteFile(backupPath, content, 0644)
	}

	// Back up go.work.sum if it exists.
	goWorkSumPath := filepath.Join(repoRoot, "go.work.sum")
	if content, err := os.ReadFile(goWorkSumPath); err == nil {
		backupSumPath := filepath.Join(backupDir, "go.work.sum.backup")
		_ = os.WriteFile(backupSumPath, content, 0644)
	}

	// Parse the root module path from go.mod.
	rootModulePath, err := readModulePath(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		// If we can't parse go.mod, proceed without filtering.
		rootModulePath = ""
	}

	// Filter out worktrees that share the root module path (same-module detection).
	var filteredPaths []string
	for _, wt := range worktreePaths {
		wtModPath, err := readModulePath(filepath.Join(wt, "go.mod"))
		if err != nil || wtModPath == rootModulePath {
			// Cannot read module path or it matches root — skip.
			continue
		}
		filteredPaths = append(filteredPaths, wt)
	}

	// If all worktrees share the root module path, return early — no go.work needed.
	if len(filteredPaths) == 0 {
		return nil
	}

	// Run `go work init .` if go.work did not exist before.
	if !goWorkExists {
		initCmd := exec.Command("go", "work", "init", ".")
		initCmd.Dir = repoRoot
		if out, err := initCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go work init failed: %v: %s", err, string(out))
		}
	}

	// For each filtered path, run `go work use <path>`. Best-effort.
	for _, path := range filteredPaths {
		useCmd := exec.Command("go", "work", "use", path)
		useCmd.Dir = repoRoot
		_, _ = useCmd.CombinedOutput()
	}

	// Append SAW-managed comment to go.work.
	f, err := os.OpenFile(goWorkPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		_, _ = f.WriteString("\n// SAW-managed: restored by finalize-wave\n")
		_ = f.Close()
	}

	return nil
}

// Restore reverts go.work and go.work.sum to their pre-wave state.
func (m *GoWorkspaceManager) Restore(repoRoot string, waveNum int) error {
	backupDir := BackupDir(repoRoot, waveNum)
	goWorkPath := filepath.Join(repoRoot, "go.work")
	goWorkBackup := filepath.Join(backupDir, "go.work.backup")

	// Restore go.work from backup, or delete it if no backup existed.
	if content, err := os.ReadFile(goWorkBackup); err == nil {
		if writeErr := os.WriteFile(goWorkPath, content, 0644); writeErr != nil {
			return writeErr
		}
		_ = os.Remove(goWorkBackup)
	} else {
		if removeErr := os.Remove(goWorkPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
	}

	// Restore go.work.sum from backup, or delete it if no backup existed.
	goWorkSumPath := filepath.Join(repoRoot, "go.work.sum")
	goWorkSumBackup := filepath.Join(backupDir, "go.work.sum.backup")
	if content, err := os.ReadFile(goWorkSumBackup); err == nil {
		if writeErr := os.WriteFile(goWorkSumPath, content, 0644); writeErr != nil {
			return writeErr
		}
		_ = os.Remove(goWorkSumBackup)
	} else {
		if removeErr := os.Remove(goWorkSumPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
	}

	return nil
}

// readModulePath reads the module declaration from a go.mod file.
// Returns the module path string, or an error if not found.
func readModulePath(goModPath string) (string, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("module declaration not found in %s", goModPath)
}

// Ensure GoWorkspaceManager implements WorkspaceManager at compile time.
var _ WorkspaceManager = (*GoWorkspaceManager)(nil)
