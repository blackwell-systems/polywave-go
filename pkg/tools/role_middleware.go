package tools

import (
	"context"
	"fmt"
	"strings"
)

// writeTools is the set of tool names that RolePathMiddleware restricts.
var writeTools = map[string]bool{
	"write_file": true,
	"edit_file":  true,
}

// RolePathMiddleware blocks write_file/edit_file for paths that don't match
// any of the AllowedPathPrefixes. If AllowedPathPrefixes is empty, the
// middleware is a passthrough (Wave agents use OwnershipMiddleware instead).
func RolePathMiddleware(toolName string, c Constraints) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			// Only restrict write tools
			if !writeTools[toolName] {
				return next.Execute(ctx, execCtx, input)
			}

			// Empty prefixes = passthrough (Wave agents use OwnershipMiddleware)
			if len(c.AllowedPathPrefixes) == 0 {
				return next.Execute(ctx, execCtx, input)
			}

			// Extract file path from input (checks "file_path" then "path" fallback)
			filePath := extractFilePath(input)
			if filePath == "" {
				return next.Execute(ctx, execCtx, input)
			}

			// Check if file_path matches any allowed prefix
			for _, prefix := range c.AllowedPathPrefixes {
				if strings.HasPrefix(filePath, prefix) {
					// For Scout role: additionally require .yaml suffix.
					// Break (not continue) so a second matching prefix cannot
					// rescue a file that fails the extension restriction.
					if c.AgentRole == "scout" && !strings.HasSuffix(filePath, ".yaml") {
						break
					}
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
