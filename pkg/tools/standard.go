package tools

// StandardTools creates a workshop and registers all 7 standard SAW tools.
// Tool names use underscores (OpenAI function name compatible).
func StandardTools(workDir string) Workshop {
	reg := NewWorkshop()

	reg.Register(Tool{
		Name:        "read_file",
		Description: "Read the contents of a file. Absolute paths pass through; relative paths resolve against working directory.",
		Namespace:   "file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"file_path"},
		},
		Executor: &FileReadExecutor{},
	})

	reg.Register(Tool{
		Name:        "write_file",
		Description: "Write content to a file, creating parent directories as needed.",
		Namespace:   "file",
		InputSchema: map[string]interface{}{
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
		Executor: &FileWriteExecutor{},
	})

	reg.Register(Tool{
		Name:        "list_directory",
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

	reg.Register(Tool{
		Name:        "bash",
		Description: "Execute a shell command in the working directory. Returns combined stdout+stderr.",
		Namespace:   "bash",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Shell command to execute",
				},
			},
			"required": []string{"command"},
		},
		Executor: &BashExecutor{},
	})

	reg.Register(Tool{
		Name:        "edit_file",
		Description: "Replace old_string with new_string in a file. Fails if old_string is not found.",
		Namespace:   "file",
		InputSchema: map[string]interface{}{
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
		Executor: &EditExecutor{},
	})

	reg.Register(Tool{
		Name:        "glob",
		Description: "Find files matching a glob pattern relative to the working directory. Returns one match per line.",
		Namespace:   "file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Glob pattern (e.g. **/*.go)",
				},
			},
			"required": []string{"pattern"},
		},
		Executor: &GlobExecutor{},
	})

	reg.Register(Tool{
		Name:        "grep",
		Description: "Search for a pattern in files. Uses rg (ripgrep) if available, otherwise falls back to line-by-line scan.",
		Namespace:   "file",
		InputSchema: map[string]interface{}{
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
		Executor: &GrepExecutor{},
	})

	return reg
}
