package main

import (
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// PrintSAWErrors prints []result.SAWError to stdout.
func PrintSAWErrors(errs []result.SAWError) {
	for _, e := range errs {
		fmt.Println(FormatSAWError(e))
	}
}

// FormatSAWError formats a single SAWError for CLI display.
func FormatSAWError(e result.SAWError) string {
	var parts []string
	sev := strings.ToUpper(e.Severity)
	if sev == "" {
		sev = "ERROR"
	}
	parts = append(parts, fmt.Sprintf("[%s] %s: %s", sev, e.Code, e.Message))
	if e.Field != "" {
		parts = append(parts, fmt.Sprintf("  field: %s", e.Field))
	}
	if e.File != "" {
		loc := e.File
		if e.Line > 0 {
			loc = fmt.Sprintf("%s:%d", e.File, e.Line)
		}
		parts = append(parts, fmt.Sprintf("  location: %s", loc))
	}
	if e.Suggestion != "" {
		parts = append(parts, fmt.Sprintf("  suggestion: %s", e.Suggestion))
	}
	return strings.Join(parts, "\n")
}
