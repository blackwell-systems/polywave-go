package tools

import (
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

// Execute reads a file from the working directory.
// Preserves path traversal prevention from the reference implementation.
func (e *FileReadExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	abs := filepath.Join(execCtx.WorkDir, path)
	// Path traversal prevention
	if !strings.HasPrefix(abs, execCtx.WorkDir) {
		return "", fmt.Errorf("path traversal denied: %s", path)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Sprintf("error: %v", err), nil // return as string, not error
	}
	return string(data), nil
}

// FileWriteExecutor implements the ToolExecutor interface for writing files.
type FileWriteExecutor struct{}

// Execute writes content to a file, creating parent directories as needed.
// Preserves path traversal prevention from the reference implementation.
func (e *FileWriteExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	content, _ := input["content"].(string)
	abs := filepath.Join(execCtx.WorkDir, path)
	if !strings.HasPrefix(abs, execCtx.WorkDir) {
		return "", fmt.Errorf("path traversal denied: %s", path)
	}
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
// Preserves path traversal prevention from the reference implementation.
func (e *FileListExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}
	abs := filepath.Join(execCtx.WorkDir, path)
	if !strings.HasPrefix(abs, execCtx.WorkDir) {
		return "", fmt.Errorf("path traversal denied: %s", path)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return fmt.Sprintf("error: %v", err), nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return strings.Join(names, "\n"), nil
}

// BashExecutor implements the ToolExecutor interface for executing shell commands.
type BashExecutor struct{}

// Execute runs a shell command in the working directory with a 30-second timeout.
// Preserves timeout and error handling from the reference implementation.
func (e *BashExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	command, _ := input["command"].(string)
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "sh", "-c", command)
	cmd.Dir = execCtx.WorkDir
	out, _ := cmd.CombinedOutput() // ignore exit error, return output + status
	result := string(out)
	if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		result += fmt.Sprintf("\n[exit status %d]", cmd.ProcessState.ExitCode())
	}
	return result, nil
}
