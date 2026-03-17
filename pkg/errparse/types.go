package errparse

// StructuredError represents a single parsed error from compiler/linter/test output.
type StructuredError struct {
	File       string `json:"file"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Severity   string `json:"severity"`   // "error" | "warning" | "info"
	Message    string `json:"message"`
	Rule       string `json:"rule,omitempty"`       // linter rule name if applicable
	Suggestion string `json:"suggestion,omitempty"` // auto-fix suggestion if available
	Tool       string `json:"tool"`                 // which tool produced this error
}

// ParseResult holds the output of parsing a tool's stdout/stderr.
type ParseResult struct {
	Tool   string            `json:"tool"`
	Errors []StructuredError `json:"errors"`
	Raw    string            `json:"raw"` // original output preserved
}

// Parser is the interface that tool-specific parsers implement.
type Parser interface {
	// Name returns the tool identifier (e.g., "go-build", "eslint").
	Name() string
	// Parse extracts structured errors from raw tool output.
	Parse(stdout, stderr string) *ParseResult
}
