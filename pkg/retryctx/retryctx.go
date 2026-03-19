// Package retryctx provides structured failure context for agent retries.
// It classifies errors from prior attempts and builds retry prompts with
// specific fix suggestions.
package retryctx

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
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

// RetryContext holds all structured information about a retry attempt for an agent.
type RetryContext struct {
	AttemptNumber  int        `json:"attempt_number"`
	AgentID        string     `json:"agent_id"`
	ErrorClass     ErrorClass `json:"error_class"`
	ErrorExcerpt   string     `json:"error_excerpt"`
	GateResults    []string   `json:"gate_results"`
	SuggestedFixes []string   `json:"suggested_fixes"`
	PriorNotes     string     `json:"prior_notes"`
	PromptText     string     `json:"prompt_text"`
	FailureType    string     `json:"failure_type,omitempty"` // "transient"|"fixable"|"needs_replan"|"escalate"|"timeout"
}

const maxExcerptLen = 2000

// BuildRetryContext reads the agent's completion report from the manifest at
// manifestPath, classifies the error, and returns a populated *RetryContext.
//
// Returns an error when:
//   - the manifest cannot be loaded
//   - the agent has no completion report in the manifest
func BuildRetryContext(manifestPath string, agentID string, attemptNum int) (*RetryContext, error) {
	m, err := protocol.Load(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("retryctx: failed to load manifest: %w", err)
	}

	report, ok := m.CompletionReports[agentID]
	if !ok {
		return nil, fmt.Errorf("retryctx: no completion report found for agent %q", agentID)
	}

	// Combine Notes and Verification as the error output source.
	rawOutput := report.Notes + "\n" + report.Verification

	// Truncate to 2000 chars max.
	excerpt := rawOutput
	if len(excerpt) > maxExcerptLen {
		excerpt = excerpt[:maxExcerptLen]
	}

	class := ClassifyError(rawOutput)
	fixes := SuggestFixes(class)

	// Collect gate result summaries.
	var gateResults []string
	if m.QualityGates != nil {
		for _, gate := range m.QualityGates.Gates {
			status := "unknown"
			gateResults = append(gateResults, fmt.Sprintf("%s: %s", gate.Type, status))
		}
	}

	promptText := buildPromptText(attemptNum, class, excerpt, fixes, report.Notes, report.FailureType)

	rc := &RetryContext{
		AttemptNumber:  attemptNum,
		AgentID:        agentID,
		ErrorClass:     class,
		ErrorExcerpt:   excerpt,
		GateResults:    gateResults,
		SuggestedFixes: fixes,
		PriorNotes:     report.Notes,
		PromptText:     promptText,
		FailureType:    report.FailureType,
	}
	return rc, nil
}

// buildPromptText formats a markdown retry prompt for the agent.
// When failureType is "fixable" and priorNotes is non-empty, a "## Fix Required"
// section is prepended to guide the agent to apply the identified fix first.
func buildPromptText(attemptNum int, class ErrorClass, excerpt string, fixes []string, priorNotes string, failureType string) string {
	var sb strings.Builder

	// For fixable failures, surface the specific fix prominently at the top.
	if failureType == "fixable" && priorNotes != "" {
		sb.WriteString("## Fix Required\n\n")
		sb.WriteString("The previous attempt identified a specific fix needed:\n\n")
		sb.WriteString(priorNotes)
		sb.WriteString("\n\nApply this fix before proceeding with your task.\n\n")
	}

	fmt.Fprintf(&sb, "## Retry Context (Attempt %d)\n\n", attemptNum)
	fmt.Fprintf(&sb, "### Error Classification: %s\n\n", string(class))

	sb.WriteString("### Error Output\n\n")
	sb.WriteString("```\n")
	sb.WriteString(excerpt)
	sb.WriteString("\n```\n\n")

	sb.WriteString("### Suggested Fixes\n\n")
	for _, fix := range fixes {
		fmt.Fprintf(&sb, "- %s\n", fix)
	}
	sb.WriteString("\n")

	sb.WriteString("### Prior Attempt Notes\n\n")
	sb.WriteString(priorNotes)
	sb.WriteString("\n")

	return sb.String()
}
