// pkg/tools/types.go
package tools

import "context"

// Workshop manages tool registration and lookup with namespace filtering.
// This implements the Registry design pattern with domain-friendly naming.
type Workshop interface {
	Register(tool Tool) error
	Get(name string) (Tool, bool)
	All() []Tool
	Namespace(prefix string) []Tool
}

// ToolExecutor defines the execution interface for all tools.
// Executors receive a context and input map and return a string result.
type ToolExecutor interface {
	Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error)
}

// Middleware wraps a ToolExecutor to add cross-cutting concerns (logging, timing, validation).
// Middleware functions are applied in stack order: outer middleware wraps inner.
type Middleware func(next ToolExecutor) ToolExecutor

// ToolAdapter serializes tools into backend-specific formats and deserializes responses.
type ToolAdapter interface {
	// Serialize converts a slice of Tool into the backend's native tool format.
	// Return type is interface{} because each backend has a different schema.
	Serialize(tools []Tool) interface{}

	// Deserialize extracts tool call name and input from a backend response.
	// Returns (toolName, inputMap, error). Used during tool-use loops.
	Deserialize(response interface{}) (string, map[string]interface{}, error)
}

// Tool represents a single agent capability with namespaced name and executor.
type Tool struct {
	Name        string                 // Namespaced tool name (e.g., "file:read", "git:commit")
	Description string                 // Human-readable description for the AI
	InputSchema map[string]interface{} // JSON Schema for tool input (backend-agnostic)
	Namespace   string                 // Category prefix (e.g., "file", "git", "bash")
	Executor    ToolExecutor           // Execution implementation
}

// ExecutionContext carries per-execution state for tool executors.
type ExecutionContext struct {
	WorkDir    string                 // Working directory for file operations
	Metadata   map[string]interface{} // Arbitrary metadata (e.g., agent ID, wave number)
}
