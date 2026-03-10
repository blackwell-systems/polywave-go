package tools

import (
	"context"
	"fmt"
	"time"
)

// executorFunc is an adapter to use a function as a ToolExecutor.
type executorFunc func(context.Context, ExecutionContext, map[string]interface{}) (string, error)

func (f executorFunc) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	return f(ctx, execCtx, input)
}

// ToolCallEvent is emitted by TimingMiddleware after each tool execution.
// Backends convert this to backend.ToolCallEvent for the Observatory SSE pipeline.
type ToolCallEvent struct {
	ToolName   string
	DurationMs int64
	IsError    bool
}

// Apply wraps an executor with a stack of middleware functions.
// Middleware are applied right-to-left: the first middleware in the list
// is the outermost (executes first in the call chain).
func Apply(executor ToolExecutor, middlewares ...Middleware) ToolExecutor {
	result := executor
	for i := len(middlewares) - 1; i >= 0; i-- {
		result = middlewares[i](result)
	}
	return result
}

// TimingMiddleware returns middleware that measures execution time and emits
// a ToolCallEvent. The tool name is baked in at wrap time so it doesn't
// depend on runtime metadata.
func TimingMiddleware(toolName string, onCall func(ToolCallEvent)) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			start := time.Now()
			result, err := next.Execute(ctx, execCtx, input)
			if onCall != nil {
				onCall(ToolCallEvent{
					ToolName:   toolName,
					DurationMs: time.Since(start).Milliseconds(),
					IsError:    err != nil,
				})
			}
			return result, err
		})
	}
}

// PermissionMiddleware returns middleware that blocks tools not in the allowed
// set. Denied tools return a message to the model (not a Go error) so it can
// adjust its approach.
func PermissionMiddleware(toolName string, allowed map[string]bool) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			if !allowed[toolName] {
				return fmt.Sprintf("error: tool %q is not permitted in read-only mode", toolName), nil
			}
			return next.Execute(ctx, execCtx, input)
		})
	}
}

// WithTiming returns a new Workshop where every tool has TimingMiddleware.
// The onCall callback receives a ToolCallEvent after each execution.
func WithTiming(w Workshop, onCall func(ToolCallEvent)) Workshop {
	wrapped := NewWorkshop()
	for _, tool := range w.All() {
		mw := TimingMiddleware(tool.Name, onCall)
		tool.Executor = mw(tool.Executor)
		wrapped.Register(tool)
	}
	return wrapped
}

// WithPermissions returns a new Workshop where every tool has
// PermissionMiddleware. Tools not in the allowed set return a denial message.
func WithPermissions(w Workshop, allowed map[string]bool) Workshop {
	wrapped := NewWorkshop()
	for _, tool := range w.All() {
		mw := PermissionMiddleware(tool.Name, allowed)
		tool.Executor = mw(tool.Executor)
		wrapped.Register(tool)
	}
	return wrapped
}

// ReadOnlyAllowed is the permission set for Scout agents.
// read_file, list_directory, glob, grep, and bash are permitted.
// write_file and edit_file are denied at execution time.
var ReadOnlyAllowed = map[string]bool{
	"read_file":      true,
	"list_directory": true,
	"glob":           true,
	"grep":           true,
	"bash":           true,
}

// ReadOnlyTools returns a Workshop for Scout agents: all 7 tools are
// registered (model sees them in the schema) but write_file and edit_file
// are blocked at execution time via PermissionMiddleware.
func ReadOnlyTools(workDir string) Workshop {
	return WithPermissions(StandardTools(workDir), ReadOnlyAllowed)
}
