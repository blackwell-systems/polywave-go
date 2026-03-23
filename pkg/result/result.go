package result

// Result wraps operation results with structured error handling and
// consistent success/failure signaling. Replaces 68 distinct *Result types.
type Result[T any] struct {
	Data   *T               `json:"data,omitempty"`
	Errors []StructuredError `json:"errors,omitempty"`
	Code   string           `json:"code"` // "SUCCESS" | "PARTIAL" | "FATAL"
}

// StructuredError provides consistent error reporting across all operations.
type StructuredError struct {
	Code       string            `json:"code"`        // Error code (E001-E999)
	Message    string            `json:"message"`     // Human-readable message
	Severity   string            `json:"severity"`    // "fatal" | "error" | "warning" | "info"
	File       string            `json:"file,omitempty"`
	Line       int               `json:"line,omitempty"`
	Field      string            `json:"field,omitempty"`
	Tool       string            `json:"tool,omitempty"`
	Suggestion string            `json:"suggestion,omitempty"`
	Context    map[string]string `json:"context,omitempty"`
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

// GetData returns the data payload, panicking if result is not successful.
// Use IsSuccess() check before calling.
func (r Result[T]) GetData() T {
	if r.Data == nil {
		panic("GetData called on Result with nil Data")
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
func NewPartial[T any](data T, warnings []StructuredError) Result[T] {
	return Result[T]{
		Data:   &data,
		Errors: warnings,
		Code:   "PARTIAL",
	}
}

// NewFailure creates a failed Result with structured errors.
func NewFailure[T any](errors []StructuredError) Result[T] {
	code := "FATAL"
	if len(errors) > 0 && errors[0].Severity != "fatal" {
		code = "FATAL" // Still fatal if operation failed
	}
	return Result[T]{
		Errors: errors,
		Code:   code,
	}
}
