package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/polywave-go/internal/git"
)

// extractStringInput retrieves a string value from an input map by key.
// Returns (value, true) if the key exists and contains a non-empty string.
// Returns ("", false) if the key is absent, the value is the wrong type, or
// the string is empty.
func extractStringInput(input map[string]interface{}, key string) (string, bool) {
	if input == nil {
		return "", false
	}
	v, ok := input[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

// FileReadExecutor implements the ToolExecutor interface for reading files.
type FileReadExecutor struct{}

// Execute reads a file. Absolute paths pass through; relative paths resolve
// against the working directory.
func (e *FileReadExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	path, ok := extractStringInput(input, "file_path")
	if !ok {
		path, ok = extractStringInput(input, "path")
	}
	if !ok {
		return "input validation failed: 'file_path' must be a non-empty string", nil
	}
	abs := resolvePath(execCtx.WorkDir, path)
	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Sprintf("error: %v", err), nil
	}
	return string(data), nil
}

// FileWriteExecutor implements the ToolExecutor interface for writing files.
type FileWriteExecutor struct{}

// Execute writes content to a file, creating parent directories as needed.
// Absolute paths pass through; relative paths resolve against working directory.
func (e *FileWriteExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	path, ok := extractStringInput(input, "file_path")
	if !ok {
		path, ok = extractStringInput(input, "path")
	}
	if !ok {
		return "input validation failed: 'file_path' must be a non-empty string", nil
	}
	content, _ := input["content"].(string) // content may be empty; that is valid
	abs := resolvePath(execCtx.WorkDir, path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Sprintf("error creating dirs: %v", err), nil
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return fmt.Sprintf("error writing: %v", err), nil
	}
	return "ok", nil
}

// FileListExecutor implements the ToolExecutor interface for listing directories.
type FileListExecutor struct{}

// Execute lists files in a directory.
// Absolute paths pass through; relative paths resolve against working directory.
func (e *FileListExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string) // empty path defaults to "." below; intentional
	if path == "" {
		path = "."
	}
	abs := resolvePath(execCtx.WorkDir, path)
	entries, err := os.ReadDir(abs)
	if err != nil {
		return fmt.Sprintf("error: %v", err), nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return strings.Join(names, "\n"), nil
}

// BashExecutor implements the ToolExecutor interface for executing shell commands.
type BashExecutor struct{}

// Execute runs a shell command in the working directory with a 60-second timeout.
func (e *BashExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	command, ok := extractStringInput(input, "command")
	if !ok {
		return "input validation failed: 'command' must be a non-empty string", nil
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "bash", "-c", command)
	cmd.Dir = execCtx.WorkDir
	out, _ := cmd.CombinedOutput() // ignore exit error, return output + status
	result := string(out)
	if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		result += fmt.Sprintf("\n[exit status %d]", cmd.ProcessState.ExitCode())
	}

	// I1 layer 2: after git commands that can modify files, check for ownership violations.
	// Reads .polywave-ownership.json from working directory (same as CLI hook approach).
	if isGitModifyCommand(command) {
		if ownedFiles := loadOwnershipFromWorkDir(execCtx.WorkDir); len(ownedFiles) > 0 {
			if warning := checkGitOwnershipViolations(execCtx.WorkDir, ownedFiles); warning != "" {
				result += "\n" + warning
			}
		}
	}

	return result, nil
}

// isGitModifyCommand returns true if the command contains a git operation that can modify files.
func isGitModifyCommand(command string) bool {
	for _, pattern := range []string{"git checkout", "git merge", "git rebase", "git cherry-pick", "git stash", "git reset", "git restore"} {
		if strings.Contains(command, pattern) {
			return true
		}
	}
	return false
}

// checkGitOwnershipViolations runs git diff to find files modified outside the ownership list.
// Returns a warning string if violations found, empty string otherwise.
func checkGitOwnershipViolations(workDir string, ownedFiles map[string]bool) string {
	unstaged, err := git.DiffNameOnlyHEAD(workDir)
	if err != nil {
		return fmt.Sprintf("WARNING: ownership check could not run (git diff failed: %v). Verify manually that no unowned files were modified.", err)
	}
	staged, _ := git.DiffNameOnlyStaged(workDir)

	// Collect all changed files, deduplicating.
	seen := make(map[string]bool)
	var violations []string
	for _, file := range append(unstaged, staged...) {
		file = strings.TrimSpace(file)
		if file == "" || seen[file] {
			continue
		}
		seen[file] = true
		if !ownedFiles[file] {
			violations = append(violations, file)
		}
	}

	if len(violations) == 0 {
		return ""
	}

	return fmt.Sprintf(
		"WARNING: Git operation modified files outside your ownership list:\n  - %s\n"+
			"Do NOT commit these changes. Run: git checkout HEAD -- <file> for each.\n"+
			"Committing unowned files can silently revert other agents' work.",
		strings.Join(violations, "\n  - "),
	)
}

// loadOwnershipFromWorkDir reads .polywave-ownership.json from the working directory
// and returns the owned files map. Returns nil if not in a Polywave context.
func loadOwnershipFromWorkDir(workDir string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(workDir, ".polywave-ownership.json"))
	if err != nil {
		return nil // Not in a Polywave worktree
	}
	var manifest struct {
		OwnedFiles []string `json:"owned_files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	if len(manifest.OwnedFiles) == 0 {
		return nil
	}
	owned := make(map[string]bool, len(manifest.OwnedFiles))
	for _, f := range manifest.OwnedFiles {
		owned[f] = true
	}
	return owned
}

// EditExecutor implements the ToolExecutor interface for search-and-replace edits.
type EditExecutor struct{}

// Execute replaces old_string with new_string in a file. Fails if old_string is not found.
func (e *EditExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	path, ok := extractStringInput(input, "file_path")
	if !ok {
		return "input validation failed: 'file_path' must be a non-empty string", nil
	}
	oldStr, _ := input["old_string"].(string) // empty caught by Contains check below
	newStr, _ := input["new_string"].(string) // empty string is a valid replacement
	abs := resolvePath(execCtx.WorkDir, path)
	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Sprintf("error: reading file: %v", err), nil
	}
	contents := string(data)
	if !strings.Contains(contents, oldStr) {
		return fmt.Sprintf("error: old_string not found in %s", path), nil
	}
	updated := strings.Replace(contents, oldStr, newStr, 1)
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return fmt.Sprintf("error: writing file: %v", err), nil
	}
	return "ok", nil
}

// GlobExecutor implements the ToolExecutor interface for file pattern matching.
type GlobExecutor struct{}

// Execute finds files matching a glob pattern. Returns one match per line.
func (e *GlobExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	pattern, _ := input["pattern"].(string)
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(execCtx.WorkDir, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Sprintf("error: %v", err), nil
	}
	return strings.Join(matches, "\n"), nil
}

// GrepExecutor implements the ToolExecutor interface for content search.
type GrepExecutor struct{}

// Execute searches for a pattern in files using rg (ripgrep) if available,
// falling back to a line-by-line scan.
func (e *GrepExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	pattern, _ := input["pattern"].(string)
	searchPath, _ := input["path"].(string)
	if searchPath == "" {
		searchPath = "."
	}
	abs := resolvePath(execCtx.WorkDir, searchPath)

	// Try rg first.
	rgPath, err := exec.LookPath("rg")
	if err == nil {
		timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(timeoutCtx, rgPath, pattern, abs)
		cmd.Dir = execCtx.WorkDir
		out, _ := cmd.CombinedOutput()
		return string(out), nil
	}

	// Fallback: line-scan with strings.Contains.
	return grepFallback(abs, pattern), nil
}

// grepFallback scans files under root for lines containing substr.
func grepFallback(root, substr string) string {
	var results strings.Builder
	if walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(line, substr) {
				results.WriteString(fmt.Sprintf("%s:%d:%s\n", path, lineNum, line))
			}
		}
		if err := scanner.Err(); err != nil {
			results.WriteString(fmt.Sprintf("[warning: read error in %s: %v]\n", path, err))
		}
		return nil
	}); walkErr != nil {
		results.WriteString(fmt.Sprintf("[warning: walk error: %v]\n", walkErr))
	}
	return results.String()
}

// resolvePath returns an absolute path: if p is already absolute, return it;
// otherwise join with workDir.
func resolvePath(workDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(workDir, p)
}
