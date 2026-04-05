// Package result provides a unified Result[T] type for consistent error handling
// and success/failure signaling across the scout-and-wave engine.
//
// This package replaces 68 distinct *Result types found throughout the codebase
// with a single generic Result[T] wrapper that supports:
//   - Full success (Code: "SUCCESS", Data present, no errors)
//   - Partial success (Code: "PARTIAL", Data present, errors in Errors)
//   - Total failure (Code: "FATAL", no Data, fatal errors in Errors)
//
// The unified interface eliminates inconsistent success checking patterns
// (IsSuccess vs Success vs Ok vs Error==nil) and provides structured error
// reporting with severity, context, and suggestions for remediation.
package result

import (
	"fmt"
)

// Result wraps operation results with structured error handling and
// consistent success/failure signaling. Replaces 68 distinct *Result types.
type Result[T any] struct {
	Data   *T         `json:"data,omitempty"`
	Errors []SAWError `json:"errors,omitempty"`
	Code   string     `json:"code"` // "SUCCESS" | "PARTIAL" | "FATAL"
}

// SAWError is the unified structured error type for all SAW operations.
// Replaces: protocol.ValidationError, errparse.StructuredError, result.SAWError.
// Implements the error interface. Supports errors.Is/As chains via Unwrap.
type SAWError struct {
	Code       string            `json:"code"`                // e.g. "V001_MANIFEST_INVALID"
	Message    string            `json:"message"`             // human-readable
	Severity   string            `json:"severity"`            // "fatal"|"error"|"warning"|"info"
	File       string            `json:"file,omitempty"`
	Line       int               `json:"line,omitempty"`
	Field      string            `json:"field,omitempty"`     // for validation errors
	Tool       string            `json:"tool,omitempty"`      // for tool/parse errors
	Suggestion string            `json:"suggestion,omitempty"`
	Context    map[string]string `json:"context,omitempty"`   // slug, wave, agent_id, rule, column
	Cause      error             `json:"-"`                   // wrapped error for errors.Is/As
}

// Error implements the error interface for SAWError.
func (e SAWError) Error() string {
	if e.Severity != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Severity, e.Code, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the wrapped cause error for errors.Is/As chain support.
func (e SAWError) Unwrap() error { return e.Cause }

// IsFatal returns true when the error severity is "fatal".
func (e SAWError) IsFatal() bool { return e.Severity == "fatal" }

// WithContext returns a copy of the error with an additional context key-value pair.
func (e SAWError) WithContext(key, value string) SAWError {
	newCtx := make(map[string]string, len(e.Context)+1)
	for k, v := range e.Context {
		newCtx[k] = v
	}
	newCtx[key] = value
	e.Context = newCtx
	return e
}

// WithCause returns a copy of the error with the given cause attached.
func (e SAWError) WithCause(cause error) SAWError {
	e.Cause = cause
	return e
}

// NewError creates a SAWError with severity "error".
func NewError(code, message string) SAWError {
	return SAWError{Code: code, Message: message, Severity: "error"}
}

// NewFatal creates a SAWError with severity "fatal".
func NewFatal(code, message string) SAWError {
	return SAWError{Code: code, Message: message, Severity: "fatal"}
}

// NewWarning creates a SAWError with severity "warning".
func NewWarning(code, message string) SAWError {
	return SAWError{Code: code, Message: message, Severity: "warning"}
}


// IsSuccess returns true when Code is "SUCCESS" and no errors exist.
func (r Result[T]) IsSuccess() bool {
	return r.Code == "SUCCESS" && len(r.Errors) == 0
}

// IsFatal returns true when Code is "FATAL".
func (r Result[T]) IsFatal() bool {
	return r.Code == "FATAL"
}

// IsPartial returns true when Code is "PARTIAL" (operation succeeded with warnings).
func (r Result[T]) IsPartial() bool {
	return r.Code == "PARTIAL"
}

// HasErrors returns true if any errors exist, regardless of severity.
func (r Result[T]) HasErrors() bool {
	return len(r.Errors) > 0
}

// GetData returns the data payload. Returns the zero value of T if Data is nil.
// Prefer checking IsSuccess() before calling to ensure meaningful data.
func (r Result[T]) GetData() T {
	if r.Data == nil {
		var zero T
		return zero
	}
	return *r.Data
}

// NewSuccess creates a successful Result with data.
func NewSuccess[T any](data T) Result[T] {
	return Result[T]{
		Data: &data,
		Code: "SUCCESS",
	}
}

// NewPartial creates a partially successful Result with data and warnings.
// Panics if errs is empty — a PARTIAL result must carry at least one diagnostic.
func NewPartial[T any](data T, errs []SAWError) Result[T] {
	if len(errs) == 0 {
		panic("result.NewPartial called with empty errors slice")
	}
	return Result[T]{
		Data:   &data,
		Errors: errs,
		Code:   "PARTIAL",
	}
}

// NewFailure creates a failed Result with structured errors.
// Panics if errors is empty — passing an empty slice is a programming error.
func NewFailure[T any](errors []SAWError) Result[T] {
	if len(errors) == 0 {
		panic("result.NewFailure called with empty errors slice")
	}
	return Result[T]{
		Errors: errors,
		Code:   "FATAL",
	}
}

// ToErrors converts a slice of SAWError to a slice of error so callers can
// pass the result to errors.Join or range over standard error values.
func ToErrors(errs []SAWError) []error {
	out := make([]error, len(errs))
	for i, e := range errs {
		out[i] = e
	}
	return out
}
