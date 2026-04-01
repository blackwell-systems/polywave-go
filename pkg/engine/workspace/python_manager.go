package workspace

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PythonWorkspaceManager implements WorkspaceManager for Python repos.
// It detects pyrightconfig.json or pyproject.toml at the repo root and
// updates extraPaths so that Pyright can resolve cross-worktree types.
type PythonWorkspaceManager struct{}

func (m *PythonWorkspaceManager) Language() string { return "python" }

// Detect returns true if pyrightconfig.json or pyproject.toml exists at repoRoot.
func (m *PythonWorkspaceManager) Detect(repoRoot string) bool {
	if _, err := os.Stat(filepath.Join(repoRoot, "pyrightconfig.json")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "pyproject.toml")); err == nil {
		return true
	}
	return false
}

// Setup modifies Python workspace configuration to include all worktreePaths
// for Pyright LSP resolution. Backs up existing config before modification.
func (m *PythonWorkspaceManager) Setup(repoRoot string, waveNum int, worktreePaths []string) error {
	// Compute relative paths from repoRoot to each worktree.
	relPaths := make([]string, 0, len(worktreePaths))
	for _, wt := range worktreePaths {
		rel, err := filepath.Rel(repoRoot, wt)
		if err != nil {
			rel = wt
		}
		relPaths = append(relPaths, rel)
	}

	pyrightConfigPath := filepath.Join(repoRoot, "pyrightconfig.json")
	pyprojectPath := filepath.Join(repoRoot, "pyproject.toml")

	pyrightExists := false
	if _, err := os.Stat(pyrightConfigPath); err == nil {
		pyrightExists = true
	}

	pyprojectExists := false
	if _, err := os.Stat(pyprojectPath); err == nil {
		pyprojectExists = true
	}

	backupDir := BackupDir(repoRoot, waveNum)

	if pyrightExists {
		// Backup and update pyrightconfig.json.
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("python_manager: mkdir backup dir: %w", err)
		}
		content, err := os.ReadFile(pyrightConfigPath)
		if err != nil {
			return fmt.Errorf("python_manager: read pyrightconfig.json: %w", err)
		}
		backupPath := filepath.Join(backupDir, "pyrightconfig.json.backup")
		if err := os.WriteFile(backupPath, content, 0644); err != nil {
			return fmt.Errorf("python_manager: backup pyrightconfig.json: %w", err)
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal(content, &cfg); err != nil {
			return fmt.Errorf("python_manager: parse pyrightconfig.json: %w", err)
		}

		// Merge relPaths into extraPaths array.
		existing := []interface{}{}
		if ep, ok := cfg["extraPaths"]; ok {
			if arr, ok := ep.([]interface{}); ok {
				existing = arr
			}
		}
		for _, p := range relPaths {
			existing = append(existing, p)
		}
		cfg["extraPaths"] = existing

		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("python_manager: marshal pyrightconfig.json: %w", err)
		}
		if err := os.WriteFile(pyrightConfigPath, out, 0644); err != nil {
			return fmt.Errorf("python_manager: write pyrightconfig.json: %w", err)
		}
		return nil
	}

	if pyprojectExists {
		// Backup and update pyproject.toml.
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("python_manager: mkdir backup dir: %w", err)
		}
		content, err := os.ReadFile(pyprojectPath)
		if err != nil {
			return fmt.Errorf("python_manager: read pyproject.toml: %w", err)
		}
		backupPath := filepath.Join(backupDir, "pyproject.toml.backup")
		if err := os.WriteFile(backupPath, content, 0644); err != nil {
			return fmt.Errorf("python_manager: backup pyproject.toml: %w", err)
		}

		modified, err := updatePyprojectExtraPaths(content, relPaths)
		if err != nil {
			return fmt.Errorf("python_manager: update pyproject.toml: %w", err)
		}
		if err := os.WriteFile(pyprojectPath, modified, 0644); err != nil {
			return fmt.Errorf("python_manager: write pyproject.toml: %w", err)
		}
		return nil
	}

	// Neither pyrightconfig.json nor pyproject.toml exists.
	// Create pyrightconfig.json from scratch (no backup needed).
	cfg := map[string]interface{}{
		"extraPaths": relPathsToInterface(relPaths),
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("python_manager: marshal new pyrightconfig.json: %w", err)
	}
	if err := os.WriteFile(pyrightConfigPath, out, 0644); err != nil {
		return fmt.Errorf("python_manager: write new pyrightconfig.json: %w", err)
	}
	return nil
}

// Restore reverts Python workspace configuration to its pre-wave state.
func (m *PythonWorkspaceManager) Restore(repoRoot string, waveNum int) error {
	backupDir := BackupDir(repoRoot, waveNum)

	// Handle pyrightconfig.json backup or SAW-created file.
	pyrightBackup := filepath.Join(backupDir, "pyrightconfig.json.backup")
	pyrightPath := filepath.Join(repoRoot, "pyrightconfig.json")
	if content, err := os.ReadFile(pyrightBackup); err == nil {
		// Restore from backup.
		if err := os.WriteFile(pyrightPath, content, 0644); err != nil {
			return fmt.Errorf("python_manager: restore pyrightconfig.json: %w", err)
		}
		if err := os.Remove(pyrightBackup); err != nil {
			return fmt.Errorf("python_manager: remove pyrightconfig.json.backup: %w", err)
		}
	} else if _, statErr := os.Stat(pyrightPath); statErr == nil {
		// No backup means SAW created the file; delete it.
		if err := os.Remove(pyrightPath); err != nil {
			return fmt.Errorf("python_manager: remove SAW-created pyrightconfig.json: %w", err)
		}
	}

	// Handle pyproject.toml backup.
	pyprojectBackup := filepath.Join(backupDir, "pyproject.toml.backup")
	pyprojectPath := filepath.Join(repoRoot, "pyproject.toml")
	if content, err := os.ReadFile(pyprojectBackup); err == nil {
		if err := os.WriteFile(pyprojectPath, content, 0644); err != nil {
			return fmt.Errorf("python_manager: restore pyproject.toml: %w", err)
		}
		if err := os.Remove(pyprojectBackup); err != nil {
			return fmt.Errorf("python_manager: remove pyproject.toml.backup: %w", err)
		}
	}

	return nil
}

// updatePyprojectExtraPaths performs a line-by-line edit to add relPaths to
// the [tool.pyright] extraPaths in pyproject.toml content.
func updatePyprojectExtraPaths(content []byte, relPaths []string) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Find [tool.pyright] section.
	pyrightSection := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[tool.pyright]" {
			pyrightSection = i
			break
		}
	}

	if pyrightSection == -1 {
		// [tool.pyright] absent: append entire section at end.
		pathLiterals := buildPathLiterals(relPaths)
		lines = append(lines,
			"",
			"[tool.pyright]",
			fmt.Sprintf("extraPaths = [%s]", pathLiterals),
			"# SAW-managed: restored by finalize-wave",
		)
		return joinLines(lines), nil
	}

	// [tool.pyright] present: find extraPaths key within section.
	extraPathsLine := -1
	for i := pyrightSection + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		// Stop at next section header.
		if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "#") {
			break
		}
		if strings.HasPrefix(trimmed, "extraPaths") {
			extraPathsLine = i
			break
		}
	}

	if extraPathsLine != -1 {
		// Append paths to existing extraPaths line.
		line := lines[extraPathsLine]
		for _, p := range relPaths {
			line = appendToTOMLArray(line, p)
		}
		lines[extraPathsLine] = line
	} else {
		// [tool.pyright] present but no extraPaths: insert after section header.
		pathLiterals := buildPathLiterals(relPaths)
		newLine := fmt.Sprintf("extraPaths = [%s]", pathLiterals)
		// Insert after pyrightSection.
		rest := make([]string, len(lines[pyrightSection+1:]))
		copy(rest, lines[pyrightSection+1:])
		lines = append(lines[:pyrightSection+1], append([]string{newLine}, rest...)...)
	}

	return joinLines(lines), nil
}

// appendToTOMLArray appends a string value to a TOML array line like:
// extraPaths = ["a", "b"]  →  extraPaths = ["a", "b", "c"]
func appendToTOMLArray(line, value string) string {
	closeIdx := strings.LastIndex(line, "]")
	if closeIdx == -1 {
		return line
	}
	before := line[:closeIdx]
	after := line[closeIdx+1:]
	// Check if array is empty.
	openIdx := strings.Index(before, "[")
	if openIdx == -1 {
		return line
	}
	inner := strings.TrimSpace(before[openIdx+1:])
	if inner == "" {
		return before + fmt.Sprintf(`"%s"]`, value) + after
	}
	return before + fmt.Sprintf(`, "%s"]`, value) + after
}

// buildPathLiterals produces a comma-separated list of quoted path strings.
func buildPathLiterals(paths []string) string {
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = fmt.Sprintf(`"%s"`, p)
	}
	return strings.Join(quoted, ", ")
}

// relPathsToInterface converts []string to []interface{} for JSON marshaling.
func relPathsToInterface(paths []string) []interface{} {
	out := make([]interface{}, len(paths))
	for i, p := range paths {
		out[i] = p
	}
	return out
}

// joinLines joins lines with newline, preserving trailing newline if original had one.
func joinLines(lines []string) []byte {
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
