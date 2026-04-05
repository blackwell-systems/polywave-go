// Package tools provides a flexible, extensible tool system for Scout-and-Wave agents.
//
// # Architecture
//
// The tool system has four main components:
//
// 1. Workshop - A registry for tool registration and namespace-based lookup
// 2. ToolExecutor - Interface for executing tools with context and input
// 3. Middleware - Composable wrappers for cross-cutting concerns
// 4. ToolAdapter - Backend-specific serialization for API calls
//
// # Workshop
//
// The Workshop interface manages tool registration and provides namespace filtering:
//
//	workshop := tools.NewWorkshop()
//	workshop.Register(tools.Tool{
//	    Name:        "file:read",
//	    Description: "Read file contents",
//	    Namespace:   "file",
//	    InputSchema: map[string]interface{}{ /* JSON Schema */ },
//	    Executor:    &FileReadExecutor{},
//	})
//
//	// Get all file tools
//	fileTools := workshop.Namespace("file:")
//
//	// Get specific tool
//	tool, found := workshop.Get("file:read")
//
// # Executors
//
// Tools implement the ToolExecutor interface:
//
//	type FileReadExecutor struct{}
//
//	func (e *FileReadExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
//	    path := input["path"].(string)
//	    abs := filepath.Join(execCtx.WorkDir, path)
//	    data, err := os.ReadFile(abs)
//	    if err != nil {
//	        return fmt.Sprintf("error: %v", err), nil
//	    }
//	    return string(data), nil
//	}
//
// Executors receive:
// - ctx: Go context for cancellation and deadlines
// - execCtx: Execution context with WorkDir and Metadata
// - input: Tool-specific input parameters
//
// # Middleware
//
// Middleware wraps executors to add logging, timing, validation, etc.:
//
//	executor := tools.Apply(
//	    baseExecutor,
//	    tools.LoggingMiddleware(nil),           // logs to stdout
//	    tools.TimingMiddleware(recordMetric),   // reports duration
//	    tools.ValidationMiddleware(),           // validates input
//	)
//
// Middleware are applied right-to-left: the last middleware in the list
// becomes the innermost wrapper. Execution order matches list order — the
// first middleware listed executes first (outermost call).
//
// # Adapters
//
// ToolAdapter implementations serialize tools for specific backends:
//
//	adapter := tools.NewAnthropicAdapter()
//	serialized := adapter.Serialize(workshop.All())
//	// Returns []map[string]interface{} in Anthropic Messages API format
//
// Each backend (Anthropic, OpenAI, Bedrock) has its own adapter that produces
// the correct tool format for that API.
//
// # Namespaces
//
// Tools use namespaced names for organization and filtering:
//
//	file:read, file:write, file:list  - File operations
//	bash                               - Shell execution (no namespace)
//	git:commit, git:diff               - Git operations
//
// Scout agents receive only read-only tools (file:read, file:list) via
// namespace filtering. Wave agents receive the full tool set.
package tools
