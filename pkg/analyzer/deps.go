package analyzer

import (
	"context"
	"fmt"
)

// AnalyzeDeps is a convenience wrapper for web CLI delegation.
// It builds a dependency graph and converts to output format.
// Respects context cancellation for long-running analysis.
// Deprecated: callers should use BuildGraph + ToOutput directly (Agent G wave 3 will remove this).
func AnalyzeDeps(ctx context.Context, repoRoot string, targetFiles []string) (*Output, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	graphResult := BuildGraph(ctx, repoRoot, targetFiles)
	if graphResult.IsFatal() {
		return nil, fmt.Errorf("%s", graphResult.Errors[0].Message)
	}
	output := ToOutput(graphResult.GetData())
	return output, nil
}
