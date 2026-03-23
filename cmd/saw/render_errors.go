package main

// render_errors.go provides shared helpers for rendering []result.SAWError to CLI output.
// Replaces ad-hoc error formatting in validate_cmd.go, prepare_wave.go, run_scout_cmd.go.
//
// Wave agents will implement the function bodies. This scaffold defines the signatures
// so that multiple agents can depend on these functions without conflicts.

import (
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// PrintSAWErrors writes a formatted list of SAWErrors to stderr.
// Groups errors by severity (fatal first, then error, warning, info).
func PrintSAWErrors(errs []result.StructuredError) {
	// TODO: implement in Wave 3 (Agent I)
	for _, e := range errs {
		fmt.Println(FormatSAWError(e))
	}
}

// FormatSAWError returns a single-line human-readable representation of a SAWError.
// Format: "[SEVERITY] CODE: message (file:line)"
func FormatSAWError(e result.StructuredError) string {
	// TODO: implement full formatting in Wave 3 (Agent I)
	var parts []string
	if e.Severity != "" {
		parts = append(parts, "["+strings.ToUpper(e.Severity)+"]")
	}
	if e.Code != "" {
		parts = append(parts, e.Code+":")
	}
	parts = append(parts, e.Message)
	if e.File != "" {
		loc := e.File
		if e.Line > 0 {
			loc += fmt.Sprintf(":%d", e.Line)
		}
		parts = append(parts, "("+loc+")")
	}
	return strings.Join(parts, " ")
}
