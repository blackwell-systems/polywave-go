package protocol

// wiring_shim.go — Agent D forward-declaration shim
//
// This file provides a stub ValidateWiringDeclarations so that cmd/saw
// (Agent D's work) compiles before Agent B's full AST-based implementation
// lands. At merge time, Agent B's wiring.go replaces this stub with the
// real implementation.
//
// The IMPLManifest.Wiring and WiringValidationReports fields were added to
// types.go by Agent D as a minimal forward-declaration; Agent B takes
// ownership of those fields at merge.
//
// DO NOT add real logic here. All enforcement belongs in Agent B's wiring.go.

// ValidateWiringDeclarations is the stub used by finalize-wave and
// validate-integration while Agent B's implementation is pending merge.
// Returns valid=true with no gaps — correct no-op before Agent B lands.
func ValidateWiringDeclarations(manifest *IMPLManifest, repoPath string) (*WiringValidationResult, error) {
	result := &WiringValidationResult{
		Gaps:    []WiringGap{},
		Valid:   true,
		Summary: "wiring check not yet implemented (stub — Agent B implementation pending)",
	}
	_ = repoPath
	_ = manifest
	return result, nil
}
