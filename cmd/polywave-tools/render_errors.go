package main

import (
	"fmt"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// PrintPolywaveErrors prints []result.PolywaveError to stdout.
func PrintPolywaveErrors(errs []result.PolywaveError) {
	for _, e := range errs {
		fmt.Println(FormatPolywaveError(e))
	}
}

// FormatPolywaveError formats a single PolywaveError for CLI display.
func FormatPolywaveError(e result.PolywaveError) string {
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
