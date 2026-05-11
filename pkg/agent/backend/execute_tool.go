package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/tools"
)

// ErrToolNotFound is returned when a tool name is not registered in the Workshop.
var ErrToolNotFound = errors.New("unknown tool")

// ExecuteTool looks up and executes a tool from the Workshop.
// Returns the result string and any error. Returns ErrToolNotFound (wrapped)
// when the tool name is not registered. Execution errors are returned
// with the error message as the result string for backward compatibility
// with tool_result blocks that expect a string.
func ExecuteTool(ctx context.Context, workshop tools.Workshop, name string, input map[string]interface{}, workDir string) (string, error) {
	tool, found := workshop.Get(name)
	if !found {
		return fmt.Sprintf("error: unknown tool %q", name), fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
	execCtx := tools.ExecutionContext{WorkDir: workDir}
	result, err := tool.Executor.Execute(ctx, execCtx, input)
	if err != nil {
		return fmt.Sprintf("error: %v", err), err
	}
	return result, nil
}
