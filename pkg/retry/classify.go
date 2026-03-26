// Package retry provides unified retry orchestration and error classification
// for agent retry workflows.
package retry

import (
	"regexp"
	"strings"
)

// ErrorClass categorises the type of error encountered in a prior attempt.
type ErrorClass string

const (
	// ErrorClassImport indicates a missing or unresolvable import.
	ErrorClassImport ErrorClass = "import_error"
	// ErrorClassType indicates a type mismatch or undefined symbol.
	ErrorClassType ErrorClass = "type_error"
	// ErrorClassTest indicates a failing test or panic in test code.
	ErrorClassTest ErrorClass = "test_failure"
	// ErrorClassBuild indicates a general build failure.
	ErrorClassBuild ErrorClass = "build_error"
	// ErrorClassLint indicates a lint / vet violation.
	ErrorClassLint ErrorClass = "lint_error"
	// ErrorClassUnknown is the fallback when no pattern matches.
	ErrorClassUnknown ErrorClass = "unknown"
)

// importPatterns are plain-string patterns that indicate import errors.
var importPatterns = []string{
	"could not import",
	"cannot find package",
	"no required module",
}

// typePatterns are regexp patterns that indicate type errors.
var typePatterns = []*regexp.Regexp{
	regexp.MustCompile(`cannot use .* as type`),
	regexp.MustCompile(`undefined:`),
	regexp.MustCompile(`has no field or method`),
	regexp.MustCompile(`not enough arguments`),
	regexp.MustCompile(`too many arguments`),
}

// testPatterns are plain-string patterns that indicate test failures.
var testPatterns = []string{
	"FAIL",
	"--- FAIL:",
	"panic: test",
}

// buildPatterns are plain-string patterns that indicate build errors.
var buildPatterns = []string{
	"cannot find module",
	"build constraints exclude",
	"syntax error",
}

// lintPatterns are regexp patterns that indicate lint errors.
var lintPatterns = []*regexp.Regexp{
	regexp.MustCompile(`go vet`),
	regexp.MustCompile(`should have comment`),
	regexp.MustCompile(`exported .* should`),
}

// ClassifyError inspects the error output string and returns the most specific
// ErrorClass that matches. Patterns are checked in priority order:
// import → type → test → build → lint → unknown.
func ClassifyError(output string) ErrorClass {
	// Import errors
	for _, p := range importPatterns {
		if strings.Contains(output, p) {
			return ErrorClassImport
		}
	}

	// Type errors
	for _, re := range typePatterns {
		if re.MatchString(output) {
			return ErrorClassType
		}
	}

	// Test failures
	for _, p := range testPatterns {
		if strings.Contains(output, p) {
			return ErrorClassTest
		}
	}

	// Build errors
	for _, p := range buildPatterns {
		if strings.Contains(output, p) {
			return ErrorClassBuild
		}
	}

	// Lint errors
	for _, re := range lintPatterns {
		if re.MatchString(output) {
			return ErrorClassLint
		}
	}

	return ErrorClassUnknown
}

// SuggestFixes returns a slice of actionable fix suggestions for the given error class.
func SuggestFixes(class ErrorClass) []string {
	switch class {
	case ErrorClassImport:
		return []string{
			"Check import paths match go.mod module name",
			"Run go mod tidy to resolve missing dependencies",
			"Verify the package exists at the expected path",
		}
	case ErrorClassType:
		return []string{
			"Check function signatures match interface contracts",
			"Verify struct field types match expected types",
			"Check for missing type conversions",
		}
	case ErrorClassTest:
		return []string{
			"Review test expectations against actual behavior",
			"Check for changed function signatures in test assertions",
			"Look for race conditions with -race flag",
		}
	case ErrorClassBuild:
		return []string{
			"Check for syntax errors in recent changes",
			"Verify all referenced packages exist",
			"Run go mod tidy",
		}
	case ErrorClassLint:
		return []string{
			"Add missing documentation comments to exported symbols",
			"Fix formatting with gofmt -w",
			"Address go vet warnings",
		}
	default:
		return []string{}
	}
}
