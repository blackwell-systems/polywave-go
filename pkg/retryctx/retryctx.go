// Package retryctx is a compatibility shim that re-exports types and functions
// from pkg/retry. External callers that import pkg/retryctx continue to compile
// without changes; all logic is implemented in pkg/retry.
package retryctx

import "github.com/blackwell-systems/scout-and-wave-go/pkg/retry"

// ErrorClass is a type alias for retry.ErrorClass.
// Callers can use retryctx.ErrorClass and retry.ErrorClass interchangeably.
type ErrorClass = retry.ErrorClass

// Constant re-exports from pkg/retry.
const (
	ErrorClassImport  = retry.ErrorClassImport
	ErrorClassType    = retry.ErrorClassType
	ErrorClassTest    = retry.ErrorClassTest
	ErrorClassBuild   = retry.ErrorClassBuild
	ErrorClassLint    = retry.ErrorClassLint
	ErrorClassUnknown = retry.ErrorClassUnknown
)

// RetryContext is a type alias for retry.RetryAttempt.
// JSON serialisation tags are identical, so no conversion is needed.
type RetryContext = retry.RetryAttempt

// ClassifyError delegates to retry.ClassifyError.
func ClassifyError(output string) ErrorClass { return retry.ClassifyError(output) }

// SuggestFixes delegates to retry.SuggestFixes.
func SuggestFixes(class ErrorClass) []string { return retry.SuggestFixes(class) }

// BuildRetryContext delegates to retry.BuildRetryAttempt.
func BuildRetryContext(manifestPath, agentID string, attemptNum int) (*RetryContext, error) {
	return retry.BuildRetryAttempt(manifestPath, agentID, attemptNum)
}
