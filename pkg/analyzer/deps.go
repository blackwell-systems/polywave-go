package analyzer

// AnalyzeDeps is a convenience wrapper for web CLI delegation.
// It builds a dependency graph and converts to output format.
func AnalyzeDeps(repoRoot string, targetFiles []string) (*Output, error) {
	graph, err := BuildGraph(repoRoot, targetFiles)
	if err != nil {
		return nil, err
	}
	output := ToOutput(graph)
	return output, nil
}
