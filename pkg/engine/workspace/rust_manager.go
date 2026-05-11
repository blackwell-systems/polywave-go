package workspace

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RustWorkspaceManager implements WorkspaceManager for Rust repos.
// Detect: Cargo.toml at repoRoot containing [workspace] section.
// Setup: surgically appends worktree paths to the members array in Cargo.toml.
// Backup: Cargo.toml backed up to .polywave-state/wave{N}/Cargo.toml.backup.
type RustWorkspaceManager struct{}

// Language returns the canonical name for this manager.
func (m *RustWorkspaceManager) Language() string {
	return "rust"
}

// Detect returns true if Cargo.toml exists at repoRoot and contains a [workspace] section.
func (m *RustWorkspaceManager) Detect(repoRoot string) bool {
	cargoPath := filepath.Join(repoRoot, "Cargo.toml")
	content, err := os.ReadFile(cargoPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "[workspace]")
}

// Setup modifies Cargo.toml to include all worktreePaths in the members array,
// enabling rust-analyzer cross-package type resolution during agent execution.
// Backs up the existing Cargo.toml to BackupDir(repoRoot, waveNum) before modification.
func (m *RustWorkspaceManager) Setup(repoRoot string, waveNum int, worktreePaths []string) error {
	cargoPath := filepath.Join(repoRoot, "Cargo.toml")

	// Step 1: Read Cargo.toml bytes.
	content, err := os.ReadFile(cargoPath)
	if err != nil {
		return fmt.Errorf("rust_manager: failed to read Cargo.toml: %w", err)
	}

	// Step 2: Create backup directory.
	backupDir := BackupDir(repoRoot, waveNum)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("rust_manager: failed to create backup dir: %w", err)
	}

	// Step 3: Back up Cargo.toml.
	backupPath := filepath.Join(backupDir, "Cargo.toml.backup")
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return fmt.Errorf("rust_manager: failed to write Cargo.toml backup: %w", err)
	}

	// Step 4: Line-by-line editing to append members (no TOML library).
	modified, err := insertMembersIntoCargoToml(content, repoRoot, worktreePaths)
	if err != nil {
		return fmt.Errorf("rust_manager: %w", err)
	}

	// Step 5: Write modified bytes back to Cargo.toml.
	if err := os.WriteFile(cargoPath, modified, 0644); err != nil {
		return fmt.Errorf("rust_manager: failed to write Cargo.toml: %w", err)
	}

	return nil
}

// Restore reverts Cargo.toml to its pre-wave state.
// If backupDir/Cargo.toml.backup exists, it overwrites Cargo.toml and deletes the backup.
// If no backup exists, this is a no-op.
func (m *RustWorkspaceManager) Restore(repoRoot string, waveNum int) error {
	backupDir := BackupDir(repoRoot, waveNum)
	backupPath := filepath.Join(backupDir, "Cargo.toml.backup")

	content, err := os.ReadFile(backupPath)
	if err != nil {
		// No backup: no-op (Cargo.toml was not modified).
		return nil
	}

	// Restore Cargo.toml from backup.
	cargoPath := filepath.Join(repoRoot, "Cargo.toml")
	if err := os.WriteFile(cargoPath, content, 0644); err != nil {
		return fmt.Errorf("rust_manager: failed to restore Cargo.toml: %w", err)
	}

	// Delete backup.
	_ = os.Remove(backupPath)

	return nil
}

// insertMembersIntoCargoToml performs a surgical line-by-line edit to insert
// worktree member paths before the closing ] of the members array.
func insertMembersIntoCargoToml(content []byte, repoRoot string, worktreePaths []string) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan Cargo.toml: %w", err)
	}

	// Find the members array: locate a line containing "members" and "[",
	// then find the closing "]".
	membersStart := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "members") && strings.Contains(trimmed, "[") {
			membersStart = i
			break
		}
	}

	if membersStart == -1 {
		return nil, fmt.Errorf("no members array found in Cargo.toml")
	}

	// Find the closing ] of the members array, starting from membersStart.
	closingBracket := -1
	for i := membersStart; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		// The closing bracket line contains "]" but not "[" (to avoid matching
		// a one-liner like members = ["crate_a"]).
		if strings.Contains(trimmed, "]") {
			// Check if closing bracket is on the same line as the opening.
			if i == membersStart {
				// One-liner: members = ["crate_a"] — closing bracket is on same line.
				closingBracket = i
			} else {
				closingBracket = i
			}
			break
		}
	}

	if closingBracket == -1 {
		return nil, fmt.Errorf("could not find closing ] for members array in Cargo.toml")
	}

	// Build the lines to insert before the closing bracket.
	var insertLines []string
	for _, wt := range worktreePaths {
		rel, err := filepath.Rel(repoRoot, wt)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative path for %q: %w", wt, err)
		}
		insertLines = append(insertLines, fmt.Sprintf("  %q,", rel))
	}
	insertLines = append(insertLines, "  # SAW-managed: restored by finalize-wave")

	// If the closing bracket is on the same line as members (one-liner), we need
	// to split it into a multi-line form.
	closingLine := lines[closingBracket]
	bracketIdx := strings.LastIndex(closingLine, "]")
	if closingBracket == membersStart {
		// One-liner: split it.
		// e.g. `members = ["crate_a",]` → insert before the ]
		before := closingLine[:bracketIdx]
		after := closingLine[bracketIdx:]
		var result []string
		result = append(result, lines[:closingBracket]...)
		result = append(result, before)
		result = append(result, insertLines...)
		result = append(result, after)
		result = append(result, lines[closingBracket+1:]...)
		return []byte(strings.Join(result, "\n")+endingNewline(content)), nil
	}

	// Multi-line members array: insert lines before the closing bracket line.
	// But we insert before the ] character position on that line.
	before := closingLine[:bracketIdx]
	after := closingLine[bracketIdx:]

	var result []string
	result = append(result, lines[:closingBracket]...)
	if strings.TrimSpace(before) != "" {
		result = append(result, before)
	}
	result = append(result, insertLines...)
	result = append(result, after)
	result = append(result, lines[closingBracket+1:]...)

	return []byte(strings.Join(result, "\n") + endingNewline(content)), nil
}

// endingNewline returns "\n" if the original content ended with a newline, else "".
func endingNewline(content []byte) string {
	if len(content) > 0 && content[len(content)-1] == '\n' {
		return "\n"
	}
	return ""
}
