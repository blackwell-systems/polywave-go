// Package openai provides an OpenAI-compatible backend that implements backend.Backend.
// It runs a full tool-use loop against the OpenAI Chat Completions API.
package openai

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

// tool defines one capability an agent may invoke during a session.
// Unexported to avoid circular imports — mirrors the pattern in pkg/agent/backend/api/tools.go.
type tool struct {
	Name        string
	Description string
	// Parameters is the JSON Schema "parameters" object for the function definition.
	Parameters map[string]interface{}
	Execute    func(input map[string]interface{}, workDir string) (string, error)
}

// standardTools returns the 6 standard SAW tools for OpenAI function calling.
// workDir scopes all file operations; bash commands run with workDir as CWD.
func standardTools(workDir string) []tool {
	return []tool{
		bashTool(workDir),
		readTool(workDir),
		writeTool(workDir),
		editTool(workDir),
		globTool(workDir),
		grepTool(workDir),
	}
}

func bashTool(_ string) tool {
	return tool{
		Name:        "Bash",
		Description: "Execute a shell command. Returns combined stdout and stderr.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Shell command to execute",
				},
			},
			"required": []string{"command"},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			command, _ := input["command"].(string)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", command)
			cmd.Dir = wd
			out, _ := cmd.CombinedOutput()
			result := string(out)
			if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
				result += fmt.Sprintf("\n[exit status %d]", cmd.ProcessState.ExitCode())
			}
			return result, nil
		},
	}
}

func readTool(_ string) tool {
	return tool{
		Name:        "Read",
		Description: "Read the contents of a file. Absolute paths pass through unchanged; relative paths are resolved against the working directory.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"file_path"},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			path, _ := input["file_path"].(string)
			abs := resolvePath(wd, path)
			data, err := os.ReadFile(abs)
			if err != nil {
				return fmt.Sprintf("Error: %v", err), nil
			}
			return string(data), nil
		},
	}
}

func writeTool(_ string) tool {
	return tool{
		Name:        "Write",
		Description: "Write content to a file, creating parent directories as needed.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to write",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required": []string{"file_path", "content"},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			path, _ := input["file_path"].(string)
			content, _ := input["content"].(string)
			abs := resolvePath(wd, path)
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return fmt.Sprintf("Error: creating dirs: %v", err), nil
			}
			if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
				return fmt.Sprintf("Error: writing file: %v", err), nil
			}
			return "ok", nil
		},
	}
}

func editTool(_ string) tool {
	return tool{
		Name:        "Edit",
		Description: "Replace old_string with new_string in a file. Fails if old_string is not found.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to edit",
				},
				"old_string": map[string]interface{}{
					"type":        "string",
					"description": "Exact string to find and replace",
				},
				"new_string": map[string]interface{}{
					"type":        "string",
					"description": "Replacement string",
				},
			},
			"required": []string{"file_path", "old_string", "new_string"},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			path, _ := input["file_path"].(string)
			oldStr, _ := input["old_string"].(string)
			newStr, _ := input["new_string"].(string)
			abs := resolvePath(wd, path)
			data, err := os.ReadFile(abs)
			if err != nil {
				return fmt.Sprintf("Error: reading file: %v", err), nil
			}
			contents := string(data)
			if !strings.Contains(contents, oldStr) {
				return fmt.Sprintf("Error: old_string not found in %s", path), nil
			}
			updated := strings.Replace(contents, oldStr, newStr, 1)
			if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
				return fmt.Sprintf("Error: writing file: %v", err), nil
			}
			return "ok", nil
		},
	}
}

func globTool(_ string) tool {
	return tool{
		Name:        "Glob",
		Description: "Find files matching a glob pattern relative to the working directory. Returns one match per line.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Glob pattern (e.g. **/*.go)",
				},
			},
			"required": []string{"pattern"},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			pattern, _ := input["pattern"].(string)
			// If pattern is not absolute, join with workDir.
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(wd, pattern)
			}
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Sprintf("Error: %v", err), nil
			}
			return strings.Join(matches, "\n"), nil
		},
	}
}

func grepTool(_ string) tool {
	return tool{
		Name:        "Grep",
		Description: "Search for a pattern in files. Uses rg (ripgrep) if available, otherwise falls back to a line-by-line scan.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Regular expression pattern to search for",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File or directory to search in (relative to working directory)",
				},
			},
			"required": []string{"pattern"},
		},
		Execute: func(input map[string]interface{}, wd string) (string, error) {
			pattern, _ := input["pattern"].(string)
			searchPath, _ := input["path"].(string)
			if searchPath == "" {
				searchPath = "."
			}
			abs := resolvePath(wd, searchPath)

			// Try rg first.
			rgPath, err := exec.LookPath("rg")
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, rgPath, pattern, abs)
				cmd.Dir = wd
				out, _ := cmd.CombinedOutput()
				return string(out), nil
			}

			// Fallback: line-scan with strings.Contains on files under abs.
			return grepFallback(abs, pattern), nil
		},
	}
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

// toolCallResult holds the result of a single tool call execution.
type toolCallResult struct {
	toolCallID string
	content    string
}

// executeTool finds the named tool and runs it, returning a non-fatal string result.
func executeTool(toolMap map[string]tool, name string, inputMap map[string]interface{}, workDir string) string {
	t, found := toolMap[name]
	if !found {
		return fmt.Sprintf("Error: unknown tool %q", name)
	}
	result, err := t.Execute(inputMap, workDir)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return result
}

// buildToolMap returns a name→tool lookup map for the given tools.
func buildToolMap(tools []tool) map[string]tool {
	m := make(map[string]tool, len(tools))
	for _, t := range tools {
		m[t.Name] = t
	}
	return m
}


