package tools

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
