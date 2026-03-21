// pkg/tools/constraint_enforcer.go
//
// Real middleware implementations for SAW protocol invariant enforcement.
// Registered via init() into the package-level middleware variables in
// workshop_constrained.go, replacing the passthrough stubs.
package tools

import (
	"context"
	"fmt"
	"strings"
)

// init registers the real constraint middleware constructors unconditionally,
// overriding the passthrough stubs set by workshop_constrained.go's init().
// Go init() functions run in source-file alphabetical order within a package,
// so constraint_enforcer.go (c) runs after workshop_constrained.go (w).
func init() {
	ownershipMiddlewareFn = newOwnershipMiddleware
	freezeMiddlewareFn = newFreezeMiddleware
	rolePathMiddlewareFn = newRolePathMiddleware
}

// extractFilePath returns the target file path from a tool input map.
// Checks "file_path" first (write_file), then "path" (edit_file fallback).
func extractFilePath(input map[string]interface{}) string {
	if v, ok := input["file_path"].(string); ok && v != "" {
		return v
	}
	if v, ok := input["path"].(string); ok && v != "" {
		return v
	}
	return ""
}

// newOwnershipMiddleware returns middleware that enforces I1 (file ownership).
// Write/edit operations to paths not in c.OwnedFiles are blocked with a
// structured I1_VIOLATION error.
func newOwnershipMiddleware(toolName string, c Constraints) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			filePath := extractFilePath(input)
			if filePath == "" {
				// No path to check; pass through.
				return next.Execute(ctx, execCtx, input)
			}

			if !c.OwnedFiles[filePath] {
				return "", fmt.Errorf("I1_VIOLATION: agent %s (%s) cannot write to %s (not in owned files)",
					c.AgentID, toolName, filePath)
			}

			return next.Execute(ctx, execCtx, input)
		})
	}
}

// newFreezeMiddleware returns middleware that enforces I2 (interface freeze).
// Write/edit operations to frozen paths are blocked with a structured
// I2_VIOLATION error. Freeze is only active when c.FreezeTime is non-nil.
func newFreezeMiddleware(toolName string, c Constraints) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			filePath := extractFilePath(input)
			if filePath == "" {
				return next.Execute(ctx, execCtx, input)
			}

			if c.FrozenPaths[filePath] && c.FreezeTime != nil {
				return "", fmt.Errorf("I2_VIOLATION: agent %s cannot write to frozen path %s (frozen at %s)",
					c.AgentID, filePath, c.FreezeTime.Format("2006-01-02T15:04:05Z07:00"))
			}

			return next.Execute(ctx, execCtx, input)
		})
	}
}

// newRolePathMiddleware returns middleware that enforces I6 (scout write boundaries).
// Write/edit operations to paths that don't match any c.AllowedPathPrefixes are
// blocked with a structured I6_VIOLATION error. If AllowedPathPrefixes is empty,
// this is a passthrough (Wave agents use OwnershipMiddleware instead).
func newRolePathMiddleware(toolName string, c Constraints) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			if len(c.AllowedPathPrefixes) == 0 {
				return next.Execute(ctx, execCtx, input)
			}

			filePath := extractFilePath(input)
			if filePath == "" {
				return next.Execute(ctx, execCtx, input)
			}

			for _, prefix := range c.AllowedPathPrefixes {
				if strings.HasPrefix(filePath, prefix) {
					return next.Execute(ctx, execCtx, input)
				}
			}

			role := c.AgentRole
			if role == "" {
				role = "unknown"
			}
			return "", fmt.Errorf("I6_VIOLATION: %s agent cannot write outside allowed paths %v (attempted: %s)",
				role, c.AllowedPathPrefixes, filePath)
		})
	}
}
