package analyzer

import "context"

// AnalyzeDeps is a convenience wrapper for web CLI delegation.
// It builds a dependency graph and converts to output format.
// Respects context cancellation for long-running analysis.
func AnalyzeDeps(ctx context.Context, repoRoot string, targetFiles []string) (*Output, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	graph, err := BuildGraph(repoRoot, targetFiles)
	if err != nil {
		return nil, err
	}
	output := ToOutput(graph)
	return output, nil
}
