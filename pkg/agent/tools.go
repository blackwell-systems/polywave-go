package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Tool defines one capability an agent may invoke during a session.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{} // JSON Schema for tool input (Anthropic API format)
	Execute     func(input map[string]interface{}, workDir string) (string, error)
}

// StandardTools returns the four standard tools for SAW agents.
// workDir scopes all file operations; bash commands run with workDir as CWD.
func StandardTools(workDir string) []Tool {
	return []Tool{
		readFileTool(workDir),
		writeFileTool(workDir),
		listDirectoryTool(workDir),
		bashTool(workDir),
	}
}

func readFileTool(workDir string) Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read the contents of a file. Path is relative to the working directory.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path relative to working directory",
				},
			},
			"required": []string{"path"},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			path, _ := input["path"].(string)
			abs := filepath.Join(wd, path)
			// Path traversal prevention
			if !strings.HasPrefix(abs, wd) {
				return "", fmt.Errorf("path traversal denied: %s", path)
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				return fmt.Sprintf("error: %v", err), nil // return as string, not error
			}
			return string(data), nil
		},
	}
}

func writeFileTool(workDir string) Tool {
	return Tool{
		Name:        "write_file",
		Description: "Write content to a file. Creates parent directories as needed.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
			},
			"required": []string{"path", "content"},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			path, _ := input["path"].(string)
			content, _ := input["content"].(string)
			abs := filepath.Join(wd, path)
			if !strings.HasPrefix(abs, wd) {
				return "", fmt.Errorf("path traversal denied: %s", path)
			}
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return fmt.Sprintf("error creating dirs: %v", err), nil
			}
			if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
				return fmt.Sprintf("error writing: %v", err), nil
			}
			return "ok", nil
		},
	}
}

func listDirectoryTool(workDir string) Tool {
	return Tool{
		Name:        "list_directory",
		Description: "List files in a directory. Path is relative to working directory.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":    "string",
					"default": ".",
				},
			},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			path, _ := input["path"].(string)
			if path == "" {
				path = "."
			}
			abs := filepath.Join(wd, path)
			if !strings.HasPrefix(abs, wd) {
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
		},
	}
}

func bashTool(workDir string) Tool {
	return Tool{
		Name:        "bash",
		Description: "Run a shell command in the working directory. Returns combined stdout+stderr.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Shell command to run",
				},
			},
			"required": []string{"command"},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			command, _ := input["command"].(string)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			cmd.Dir = wd
			out, _ := cmd.CombinedOutput() // ignore exit error, return output + status
			result := string(out)
			if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
				result += fmt.Sprintf("\n[exit status %d]", cmd.ProcessState.ExitCode())
			}
			return result, nil
		},
	}
}
