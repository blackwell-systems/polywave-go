package errparse

import "github.com/blackwell-systems/scout-and-wave-go/pkg/result"

// ParseResult holds the output of parsing a tool's stdout/stderr.
type ParseResult struct {
	Tool   string            `json:"tool"`
	Errors []result.SAWError `json:"errors"`
	Raw    string            `json:"raw"` // original output preserved
}

// Parser is the interface that tool-specific parsers implement.
type Parser interface {
	// Name returns the tool identifier (e.g., "go-build", "eslint").
	Name() string
	// Parse extracts structured errors from raw tool output.
	Parse(stdout, stderr string) *ParseResult
}
