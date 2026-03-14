package suitability

// AnalyzeSuitability is a convenience wrapper for web CLI delegation.
// It calls the existing ScanPreImplementation() function with the provided parameters.
func AnalyzeSuitability(requirementsFile string, repoRoot string) (*Result, error) {
	return ScanPreImplementation(requirementsFile, repoRoot)
}
