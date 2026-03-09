package tools

import (
	"context"
	"fmt"
	"time"
)

// executorFunc is an adapter to use a function as a ToolExecutor.
// This enables middleware to wrap executors using closures.
type executorFunc func(context.Context, ExecutionContext, map[string]interface{}) (string, error)

func (f executorFunc) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	return f(ctx, execCtx, input)
}

// Apply wraps an executor with a stack of middleware functions.
// Middleware are applied right-to-left: the last middleware in the list
// becomes the innermost wrapper (executes first in the call chain).
//
// Example:
//
//	executor := Apply(baseExecutor, LoggingMiddleware(nil), TimingMiddleware(onDuration))
//	// Execution order: LoggingMiddleware -> TimingMiddleware -> baseExecutor
func Apply(executor ToolExecutor, middlewares ...Middleware) ToolExecutor {
	// Apply middlewares in reverse order so the first middleware in the list
	// is the outermost (last to execute)
	result := executor
	for i := len(middlewares) - 1; i >= 0; i-- {
		result = middlewares[i](result)
	}
	return result
}

// LoggingMiddleware returns middleware that logs tool execution.
// If logger is nil, logs are written to stdout using fmt.Printf.
// Logs include: tool name, input keys, execution time, and error status.
func LoggingMiddleware(logger interface{}) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			start := time.Now()

			// Extract tool name from context metadata if available
			toolName := "unknown"
			if execCtx.Metadata != nil {
				if name, ok := execCtx.Metadata["tool_name"].(string); ok {
					toolName = name
				}
			}

			// Log input keys
			keys := make([]string, 0, len(input))
			for k := range input {
				keys = append(keys, k)
			}

			if logger == nil {
				fmt.Printf("[tool] executing %s with input keys: %v\n", toolName, keys)
			}

			// Execute the wrapped executor
			result, err := next.Execute(ctx, execCtx, input)

			// Log result
			duration := time.Since(start)
			if err != nil {
				if logger == nil {
					fmt.Printf("[tool] %s failed after %v: %v\n", toolName, duration, err)
				}
			} else {
				if logger == nil {
					fmt.Printf("[tool] %s completed in %v\n", toolName, duration)
				}
			}

			return result, err
		})
	}
}

// TimingMiddleware returns middleware that measures execution time.
// The onDuration callback is invoked after execution with the tool name
// and duration. This is useful for Observatory SSE events and metrics.
func TimingMiddleware(onDuration func(toolName string, dur time.Duration)) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			start := time.Now()

			// Execute the wrapped executor
			result, err := next.Execute(ctx, execCtx, input)

			// Report timing
			duration := time.Since(start)
			if onDuration != nil {
				// Extract tool name from context metadata if available
				toolName := "unknown"
				if execCtx.Metadata != nil {
					if name, ok := execCtx.Metadata["tool_name"].(string); ok {
						toolName = name
					}
				}
				onDuration(toolName, duration)
			}

			return result, err
		})
	}
}

// ValidationMiddleware returns middleware that validates input against
// the tool's InputSchema. Performs basic type checks on required fields.
// Full JSON Schema validation is optional and not implemented here.
func ValidationMiddleware() Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			// Extract schema from context metadata if available
			// The calling code should populate this when wrapping tools
			var schema map[string]interface{}
			if execCtx.Metadata != nil {
				if s, ok := execCtx.Metadata["input_schema"].(map[string]interface{}); ok {
					schema = s
				}
			}

			// If no schema available, skip validation
			if schema == nil {
				return next.Execute(ctx, execCtx, input)
			}

			// Check required fields
			if required, ok := schema["required"].([]interface{}); ok {
				for _, field := range required {
					fieldName, ok := field.(string)
					if !ok {
						continue
					}
					if _, exists := input[fieldName]; !exists {
						return "", fmt.Errorf("validation error: required field %q missing", fieldName)
					}
				}
			}

			// Basic type validation would go here, but we'll keep it simple
			// and just check required fields for now

			return next.Execute(ctx, execCtx, input)
		})
	}
}
