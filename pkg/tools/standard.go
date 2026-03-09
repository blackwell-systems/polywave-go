package tools

import (
	"fmt"
	"sort"
	"strings"
)

// StandardTools creates a workshop and registers all 4 standard SAW tools.
// Tools are namespaced: file:read, file:write, file:list, bash.
func StandardTools(workDir string) Workshop {
	reg := NewWorkshop()

	// File tools (namespace: "file")
	reg.Register(Tool{
		Name:        "file:read",
		Description: "Read the contents of a file. Path is relative to the working directory.",
		Namespace:   "file",
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
		Executor: &FileReadExecutor{},
	})

	reg.Register(Tool{
		Name:        "file:write",
		Description: "Write content to a file. Creates parent directories as needed.",
		Namespace:   "file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
			},
			"required": []string{"path", "content"},
		},
		Executor: &FileWriteExecutor{},
	})

	reg.Register(Tool{
		Name:        "file:list",
		Description: "List files in a directory. Path is relative to working directory.",
		Namespace:   "file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":    "string",
					"default": ".",
				},
			},
		},
		Executor: &FileListExecutor{},
	})

	// Bash tool (namespace: "bash")
	reg.Register(Tool{
		Name:        "bash",
		Description: "Run a shell command in the working directory. Returns combined stdout+stderr.",
		Namespace:   "bash",
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
		Executor: &BashExecutor{},
	})

	return reg
}

// NewWorkshop creates a new workshop instance.
// This is a temporary implementation for Agent B's worktree.
// Agent A's proper implementation will replace this after merge.
func NewWorkshop() Workshop {
	return &defaultWorkshop{
		tools: make(map[string]Tool),
	}
}

// defaultWorkshop is a minimal implementation of the Workshop interface
// for testing in Agent B's worktree before Agent A's implementation is merged.
type defaultWorkshop struct {
	tools map[string]Tool
}

func (w *defaultWorkshop) Register(tool Tool) error {
	if _, exists := w.tools[tool.Name]; exists {
		return fmt.Errorf("tool already registered: %s", tool.Name)
	}
	w.tools[tool.Name] = tool
	return nil
}

func (w *defaultWorkshop) Get(name string) (Tool, bool) {
	tool, ok := w.tools[name]
	return tool, ok
}

func (w *defaultWorkshop) All() []Tool {
	tools := make([]Tool, 0, len(w.tools))
	for _, t := range w.tools {
		tools = append(tools, t)
	}
	// Sort by name for determinism
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

func (w *defaultWorkshop) Namespace(prefix string) []Tool {
	filtered := make([]Tool, 0)
	for _, t := range w.tools {
		if strings.HasPrefix(t.Name, prefix) {
			filtered = append(filtered, t)
		}
	}
	// Sort by name for determinism
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})
	return filtered
}
