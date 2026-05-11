// pkg/tools/constraint_enforcer.go
//
// Real middleware implementations for Polywave protocol invariant enforcement.
// Registered via RegisterConstraintMiddleware(), called explicitly from
// workshop_constrained.go's init(), eliminating the fragile dependency on
// Go's alphabetical file init() ordering.
package tools

import (
	"context"
	"fmt"
)

// RegisterConstraintMiddleware sets the package-level middleware constructor
// variables to the real constraint enforcement implementations. Called explicitly
// from workshop_constrained.go's init(), eliminating the fragile dependency on
// Go's alphabetical file init() ordering.
func RegisterConstraintMiddleware() {
	ownershipMiddlewareFn = newOwnershipMiddleware
	freezeMiddlewareFn = newFreezeMiddleware
	rolePathMiddlewareFn = RolePathMiddleware
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

