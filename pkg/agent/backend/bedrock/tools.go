package bedrock

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// buildToolsJSON converts Workshop tools to the Bedrock/Anthropic Messages API
// JSON format. Each tool becomes a map with name, description, and input_schema keys.
func buildToolsJSON(workshop tools.Workshop) []interface{} {
	allTools := workshop.All()
	result := make([]interface{}, 0, len(allTools))
	for _, t := range allTools {
		result = append(result, map[string]interface{}{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.InputSchema,
		})
	}
	return result
}

// executeTool looks up a tool by name in the Workshop and executes it.
// Returns (resultString, isError). Mirrors the pattern from api/client.go.
func executeTool(ctx context.Context, workshop tools.Workshop, name string, input map[string]interface{}, workDir string) (string, bool) {
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
