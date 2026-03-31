package backend

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// ExecuteTool looks up and executes a tool from the Workshop.
// Returns (resultString, isError). isError is true when tool lookup fails
// or execution returns an error; in that case resultString contains the error message.
func ExecuteTool(ctx context.Context, workshop tools.Workshop, name string, input map[string]interface{}, workDir string) (string, bool) {
	tool, found := workshop.Get(name)
	if !found {
		return fmt.Sprintf("error: unknown tool %q", name), true
	}
	execCtx := tools.ExecutionContext{WorkDir: workDir}
	result, err := tool.Executor.Execute(ctx, execCtx, input)
	if err != nil {
		return fmt.Sprintf("error: %v", err), true
	}
	return result, false
}
