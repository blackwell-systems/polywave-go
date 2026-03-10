package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FileReadExecutor implements the ToolExecutor interface for reading files.
type FileReadExecutor struct{}

// Execute reads a file. Absolute paths pass through; relative paths resolve
// against the working directory.
func (e *FileReadExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	path, _ := input["file_path"].(string)
	if path == "" {
		path, _ = input["path"].(string) // fallback for legacy callers
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
	path, _ := input["file_path"].(string)
	if path == "" {
		path, _ = input["path"].(string)
	}
	content, _ := input["content"].(string)
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
	path, _ := input["path"].(string)
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
	command, _ := input["command"].(string)
	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "bash", "-c", command)
	cmd.Dir = execCtx.WorkDir
	out, _ := cmd.CombinedOutput() // ignore exit error, return output + status
	result := string(out)
	if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		result += fmt.Sprintf("\n[exit status %d]", cmd.ProcessState.ExitCode())
	}
	return result, nil
}

// EditExecutor implements the ToolExecutor interface for search-and-replace edits.
type EditExecutor struct{}

// Execute replaces old_string with new_string in a file. Fails if old_string is not found.
func (e *EditExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	path, _ := input["file_path"].(string)
	oldStr, _ := input["old_string"].(string)
	newStr, _ := input["new_string"].(string)
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
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
		return nil
	})
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
