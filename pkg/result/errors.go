package result

import "fmt"

// Errorf creates a new SAWError with the given code and formatted message.
// The severity is set to "fatal". This is a convenience function for creating
// errors that implement the standard error interface.
//
// Example:
//
//	return result.Errorf(result.CodeRetryReportMissing, "no report for agent %s", agentID)
func Errorf(code, format string, args ...interface{}) error {
	return SAWError{
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		Severity: "fatal",
	}
}

// WrapCode wraps an existing error with a SAWError containing the given code
// and formatted message. The original error is preserved in the Cause field,
// allowing errors.Is and errors.As to work correctly.
//
// Example:
//
//	if err := protocol.Load(ctx, path); err != nil {
//	    return result.WrapCode(err, result.CodeRetryLoadManifestFailed, "failed to load manifest at %s", path)
//	}
func WrapCode(err error, code, format string, args ...interface{}) error {
	return SAWError{
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		Severity: "fatal",
		Cause:    err,
	}
}
