package result

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestNewSuccess verifies NewSuccess constructor creates a successful Result.
func TestNewSuccess(t *testing.T) {
	data := "test data"
	r := NewSuccess(data)

	if r.Data == nil {
		t.Fatal("Data should not be nil")
	}
	if *r.Data != data {
		t.Errorf("Data = %q, want %q", *r.Data, data)
	}
	if r.Code != "SUCCESS" {
		t.Errorf("Code = %q, want %q", r.Code, "SUCCESS")
	}
	if !r.IsSuccess() {
		t.Error("IsSuccess() should return true")
	}
	if len(r.Errors) != 0 {
		t.Errorf("Errors should be empty, got %d errors", len(r.Errors))
	}
}

// TestNewPartial verifies NewPartial constructor creates a partial success Result.
func TestNewPartial(t *testing.T) {
	data := 42
	warnings := []PolywaveError{
		{
			Code:     "W001",
			Message:  "Non-critical warning",
			Severity: "warning",
		},
	}
	r := NewPartial(data, warnings)

	if r.Data == nil {
		t.Fatal("Data should not be nil")
	}
	if *r.Data != data {
		t.Errorf("Data = %d, want %d", *r.Data, data)
	}
	if r.Code != "PARTIAL" {
		t.Errorf("Code = %q, want %q", r.Code, "PARTIAL")
	}
	if !r.IsPartial() {
		t.Error("IsPartial() should return true")
	}
	if len(r.Errors) != 1 {
		t.Errorf("Errors length = %d, want 1", len(r.Errors))
	}
	if !r.HasErrors() {
		t.Error("HasErrors() should return true")
	}
}

// TestNewFailure verifies NewFailure constructor creates a failed Result.
func TestNewFailure(t *testing.T) {
	errors := []PolywaveError{
		{
			Code:     "E001",
			Message:  "Fatal error occurred",
			Severity: "fatal",
		},
	}
	r := NewFailure[string](errors)

	if r.Data != nil {
		t.Error("Data should be nil for failure")
	}
	if r.Code != "FATAL" {
		t.Errorf("Code = %q, want %q", r.Code, "FATAL")
	}
	if !r.IsFatal() {
		t.Error("IsFatal() should return true")
	}
	if len(r.Errors) != 1 {
		t.Errorf("Errors length = %d, want 1", len(r.Errors))
	}
}

// TestIsSuccess verifies IsSuccess method behavior.
func TestIsSuccess(t *testing.T) {
	tests := []struct {
		name string
		r    Result[string]
		want bool
	}{
		{
			name: "success with data",
			r:    NewSuccess("data"),
			want: true,
		},
		{
			name: "partial success",
			r: NewPartial("data", []PolywaveError{
				{Code: "W001", Message: "warning", Severity: "warning"},
			}),
			want: false,
		},
		{
			name: "failure",
			r: NewFailure[string]([]PolywaveError{
				{Code: "E001", Message: "error", Severity: "fatal"},
			}),
			want: false,
		},
		{
			name: "success code but has errors",
			r: Result[string]{
				Data:   ptr("data"),
				Code:   "SUCCESS",
				Errors: []PolywaveError{{Code: "E001"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsSuccess(); got != tt.want {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsFatal verifies IsFatal method behavior.
func TestIsFatal(t *testing.T) {
	tests := []struct {
		name string
		r    Result[string]
		want bool
	}{
		{
			name: "fatal result",
			r: NewFailure[string]([]PolywaveError{
				{Code: "E001", Message: "fatal", Severity: "fatal"},
			}),
			want: true,
		},
		{
			name: "success result",
			r:    NewSuccess("data"),
			want: false,
		},
		{
			name: "partial result",
			r: NewPartial("data", []PolywaveError{
				{Code: "W001", Message: "warning", Severity: "warning"},
			}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsFatal(); got != tt.want {
				t.Errorf("IsFatal() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsPartial verifies IsPartial method behavior.
func TestIsPartial(t *testing.T) {
	tests := []struct {
		name string
		r    Result[int]
		want bool
	}{
		{
			name: "partial result",
			r: NewPartial(123, []PolywaveError{
				{Code: "W001", Message: "warning", Severity: "warning"},
			}),
			want: true,
		},
		{
			name: "success result",
			r:    NewSuccess(456),
			want: false,
		},
		{
			name: "fatal result",
			r: NewFailure[int]([]PolywaveError{
				{Code: "E001", Message: "error", Severity: "fatal"},
			}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsPartial(); got != tt.want {
				t.Errorf("IsPartial() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHasErrors verifies HasErrors method behavior.
func TestHasErrors(t *testing.T) {
	tests := []struct {
		name string
		r    Result[string]
		want bool
	}{
		{
			name: "no errors",
			r:    NewSuccess("data"),
			want: false,
		},
		{
			name: "has warnings",
			r: NewPartial("data", []PolywaveError{
				{Code: "W001", Message: "warning", Severity: "warning"},
			}),
			want: true,
		},
		{
			name: "has fatal errors",
			r: NewFailure[string]([]PolywaveError{
				{Code: "E001", Message: "error", Severity: "fatal"},
			}),
			want: true,
		},
		{
			name: "empty errors slice",
			r: Result[string]{
				Data:   ptr("data"),
				Code:   "SUCCESS",
				Errors: []PolywaveError{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.HasErrors(); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetData verifies GetData method returns data when present.
func TestGetData(t *testing.T) {
	t.Run("returns data when present", func(t *testing.T) {
		expected := "test data"
		r := NewSuccess(expected)
		got := r.GetData()
		if got != expected {
			t.Errorf("GetData() = %q, want %q", got, expected)
		}
	})
}

// TestGetDataZeroOnNilData verifies GetData returns the zero value instead of
// panicking when called on a Result with nil Data (e.g. a FATAL result).
func TestGetDataZeroOnNilData(t *testing.T) {
	r := NewFailure[string]([]PolywaveError{
		{Code: "E001", Message: "error", Severity: "fatal"},
	})

	// Must not panic
	var got string
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Errorf("GetData() panicked unexpectedly: %v", rec)
			}
		}()
		got = r.GetData()
	}()

	// Zero value of string is ""
	if got != "" {
		t.Errorf("GetData() = %q, want zero value %q", got, "")
	}
}

// TestGetDataSuccess verifies GetData returns the correct value on a SUCCESS result.
func TestGetDataSuccess(t *testing.T) {
	expected := "hello world"
	r := NewSuccess(expected)
	got := r.GetData()
	if got != expected {
		t.Errorf("GetData() = %q, want %q", got, expected)
	}
}

// TestJSONMarshalUnmarshal verifies JSON serialization round-trip.
func TestJSONMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		r    Result[string]
	}{
		{
			name: "success result",
			r:    NewSuccess("test data"),
		},
		{
			name: "partial result with warnings",
			r: NewPartial("partial data", []PolywaveError{
				{
					Code:       "W001",
					Message:    "Warning message",
					Severity:   "warning",
					File:       "test.go",
					Line:       42,
					Suggestion: "Fix this",
				},
			}),
		},
		{
			name: "failure result with errors",
			r: NewFailure[string]([]PolywaveError{
				{
					Code:     "E001",
					Message:  "Fatal error",
					Severity: "fatal",
					Context: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			jsonData, err := json.Marshal(tt.r)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Unmarshal back
			var decoded Result[string]
			err = json.Unmarshal(jsonData, &decoded)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			// Verify Code
			if decoded.Code != tt.r.Code {
				t.Errorf("Code = %q, want %q", decoded.Code, tt.r.Code)
			}

			// Verify Data
			if (decoded.Data == nil) != (tt.r.Data == nil) {
				t.Errorf("Data nil mismatch: got %v, want %v", decoded.Data == nil, tt.r.Data == nil)
			}
			if decoded.Data != nil && tt.r.Data != nil && *decoded.Data != *tt.r.Data {
				t.Errorf("Data = %q, want %q", *decoded.Data, *tt.r.Data)
			}

			// Verify Errors length
			if len(decoded.Errors) != len(tt.r.Errors) {
				t.Errorf("Errors length = %d, want %d", len(decoded.Errors), len(tt.r.Errors))
			}
		})
	}
}

// TestGenericTypeInstantiation verifies Result works with different types.
func TestGenericTypeInstantiation(t *testing.T) {
	t.Run("Result[string]", func(t *testing.T) {
		r := NewSuccess("string data")
		if !r.IsSuccess() {
			t.Error("Result[string] should be success")
		}
	})

	t.Run("Result[int]", func(t *testing.T) {
		r := NewSuccess(42)
		if r.GetData() != 42 {
			t.Error("Result[int] data mismatch")
		}
	})

	t.Run("Result[struct]", func(t *testing.T) {
		type TestStruct struct {
			Name  string
			Value int
		}
		data := TestStruct{Name: "test", Value: 100}
		r := NewSuccess(data)
		got := r.GetData()
		if got.Name != data.Name || got.Value != data.Value {
			t.Error("Result[struct] data mismatch")
		}
	})

	t.Run("Result[*struct]", func(t *testing.T) {
		type TestStruct struct {
			Field string
		}
		data := &TestStruct{Field: "value"}
		r := NewSuccess(data)
		got := r.GetData()
		if got.Field != data.Field {
			t.Error("Result[*struct] data mismatch")
		}
	})

	t.Run("Result[[]string]", func(t *testing.T) {
		data := []string{"a", "b", "c"}
		r := NewSuccess(data)
		got := r.GetData()
		if len(got) != len(data) {
			t.Error("Result[[]string] data length mismatch")
		}
	})
}

// TestPolywaveErrorFields verifies PolywaveError fields serialize correctly.
func TestPolywaveErrorFields(t *testing.T) {
	err := PolywaveError{
		Code:       "E042",
		Message:    "Test error",
		Severity:   "error",
		File:       "test.go",
		Line:       123,
		Field:      "fieldName",
		Tool:       "validator",
		Suggestion: "Fix the field",
		Context: map[string]string{
			"repo":  "test-repo",
			"agent": "A",
		},
	}

	r := NewFailure[string]([]PolywaveError{err})

	// Marshal and unmarshal
	jsonData, marshalErr := json.Marshal(r)
	if marshalErr != nil {
		t.Fatalf("Marshal() error = %v", marshalErr)
	}

	var decoded Result[string]
	unmarshalErr := json.Unmarshal(jsonData, &decoded)
	if unmarshalErr != nil {
		t.Fatalf("Unmarshal() error = %v", unmarshalErr)
	}

	if len(decoded.Errors) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(decoded.Errors))
	}

	got := decoded.Errors[0]
	if got.Code != err.Code {
		t.Errorf("Code = %q, want %q", got.Code, err.Code)
	}
	if got.Message != err.Message {
		t.Errorf("Message = %q, want %q", got.Message, err.Message)
	}
	if got.Severity != err.Severity {
		t.Errorf("Severity = %q, want %q", got.Severity, err.Severity)
	}
	if got.File != err.File {
		t.Errorf("File = %q, want %q", got.File, err.File)
	}
	if got.Line != err.Line {
		t.Errorf("Line = %d, want %d", got.Line, err.Line)
	}
	if got.Field != err.Field {
		t.Errorf("Field = %q, want %q", got.Field, err.Field)
	}
	if got.Tool != err.Tool {
		t.Errorf("Tool = %q, want %q", got.Tool, err.Tool)
	}
	if got.Suggestion != err.Suggestion {
		t.Errorf("Suggestion = %q, want %q", got.Suggestion, err.Suggestion)
	}
	if len(got.Context) != len(err.Context) {
		t.Errorf("Context length = %d, want %d", len(got.Context), len(err.Context))
	}
}

// TestPolywaveErrorInterface verifies PolywaveError implements the error interface.
func TestPolywaveErrorInterface(t *testing.T) {
	var _ error = PolywaveError{} // compile-time check

	tests := []struct {
		name string
		err  PolywaveError
		want string
	}{
		{
			name: "with severity",
			err:  PolywaveError{Code: "E001", Message: "something broke", Severity: "fatal"},
			want: "[fatal] E001: something broke",
		},
		{
			name: "without severity",
			err:  PolywaveError{Code: "E002", Message: "unknown issue"},
			want: "[E002] unknown issue",
		},
		{
			name: "warning severity",
			err:  NewWarning("W001", "heads up"),
			want: "[warning] W001: heads up",
		},
		{
			name: "info severity",
			err:  PolywaveError{Code: "I001", Message: "fyi", Severity: "info"},
			want: "[info] I001: fyi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPolywaveErrorUnwrap verifies Unwrap returns the wrapped cause.
func TestPolywaveErrorUnwrap(t *testing.T) {
	t.Run("nil cause", func(t *testing.T) {
		e := NewError("E001", "test")
		if e.Unwrap() != nil {
			t.Error("Unwrap() should return nil when no cause")
		}
	})

	t.Run("with cause", func(t *testing.T) {
		cause := fmt.Errorf("root cause")
		e := NewError("E001", "test").WithCause(cause)
		if e.Unwrap() != cause {
			t.Error("Unwrap() should return the cause")
		}
	})

	t.Run("errors.Is chain", func(t *testing.T) {
		sentinel := fmt.Errorf("sentinel")
		e := NewError("E001", "wrapper").WithCause(sentinel)
		if !errors.Is(e, sentinel) {
			t.Error("errors.Is should find sentinel through Unwrap chain")
		}
	})
}

// TestPolywaveErrorIsFatal verifies IsFatal method on PolywaveError.
func TestPolywaveErrorIsFatal(t *testing.T) {
	if !NewFatal("E001", "fatal").IsFatal() {
		t.Error("NewFatal should create a fatal error")
	}
	if NewError("E001", "error").IsFatal() {
		t.Error("NewError should not be fatal")
	}
	if NewWarning("W001", "warning").IsFatal() {
		t.Error("NewWarning should not be fatal")
	}
	if (PolywaveError{Code: "I001", Severity: "info"}).IsFatal() {
		t.Error("info severity should not be fatal")
	}
}

// TestPolywaveErrorWithContext verifies WithContext adds context.
func TestPolywaveErrorWithContext(t *testing.T) {
	t.Run("adds to nil context", func(t *testing.T) {
		e := NewError("E001", "test").WithContext("key", "value")
		if e.Context == nil {
			t.Fatal("Context should not be nil")
		}
		if e.Context["key"] != "value" {
			t.Errorf("Context[key] = %q, want %q", e.Context["key"], "value")
		}
	})

	t.Run("adds to existing context", func(t *testing.T) {
		e := NewError("E001", "test").
			WithContext("k1", "v1").
			WithContext("k2", "v2")
		if len(e.Context) != 2 {
			t.Errorf("Context length = %d, want 2", len(e.Context))
		}
		if e.Context["k1"] != "v1" || e.Context["k2"] != "v2" {
			t.Error("Context values mismatch")
		}
	})

	t.Run("does not mutate original", func(t *testing.T) {
		original := NewError("E001", "test")
		_ = original.WithContext("key", "value")
		if original.Context != nil {
			t.Error("WithContext should not mutate the original error")
		}
	})
}

// TestPolywaveErrorWithCause verifies WithCause attaches a cause.
func TestPolywaveErrorWithCause(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	e := NewError("E001", "test").WithCause(cause)
	if e.Cause != cause {
		t.Error("WithCause should set the Cause field")
	}

	// Original should not be mutated
	original := NewError("E001", "test")
	_ = original.WithCause(cause)
	if original.Cause != nil {
		t.Error("WithCause should not mutate the original error")
	}
}

// TestNewErrorConstructors verifies the convenience constructors.
func TestNewErrorConstructors(t *testing.T) {
	e := NewError("E001", "error msg")
	if e.Code != "E001" || e.Message != "error msg" || e.Severity != "error" {
		t.Errorf("NewError = %+v", e)
	}

	f := NewFatal("F001", "fatal msg")
	if f.Code != "F001" || f.Message != "fatal msg" || f.Severity != "fatal" {
		t.Errorf("NewFatal = %+v", f)
	}

	w := NewWarning("W001", "warning msg")
	if w.Code != "W001" || w.Message != "warning msg" || w.Severity != "warning" {
		t.Errorf("NewWarning = %+v", w)
	}

	i := PolywaveError{Code: "I001", Message: "info msg", Severity: "info"}
	if i.Code != "I001" || i.Message != "info msg" || i.Severity != "info" {
		t.Errorf("info PolywaveError = %+v", i)
	}
}

// TestPolywaveErrorCauseNotSerialized verifies Cause is excluded from JSON.
func TestPolywaveErrorCauseNotSerialized(t *testing.T) {
	e := NewError("E001", "test").WithCause(fmt.Errorf("secret cause"))
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	// Cause should not appear in JSON
	jsonStr := string(data)
	if strings.Contains(jsonStr, "secret cause") {
		t.Error("Cause should not be serialized to JSON")
	}
	if strings.Contains(jsonStr, "cause") {
		t.Error("Cause field should not appear in JSON")
	}
}

// TestNewFailurePanicsOnEmptySlice verifies NewFailure panics when called with an empty errors slice.
func TestNewFailurePanicsOnEmptySlice(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if msg != "result.NewFailure called with empty errors slice" {
			t.Errorf("panic message = %q, want %q", msg, "result.NewFailure called with empty errors slice")
		}
	}()
	_ = NewFailure[string]([]PolywaveError{})
}

// TestToErrors verifies the ToErrors helper converts []PolywaveError to []error.
func TestToErrors(t *testing.T) {
	t.Run("nil input returns empty slice", func(t *testing.T) {
		got := ToErrors(nil)
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
	t.Run("single error round-trips", func(t *testing.T) {
		errs := []PolywaveError{{Code: "X", Message: "msg", Severity: "error"}}
		got := ToErrors(errs)
		if len(got) != 1 {
			t.Fatalf("expected 1, got %d", len(got))
		}
		want := "[error] X: msg"
		if got[0].Error() != want {
			t.Errorf("got %q, want %q", got[0].Error(), want)
		}
	})
	t.Run("compatible with errors.Join", func(t *testing.T) {
		errs := []PolywaveError{
			{Code: "A", Message: "first", Severity: "error"},
			{Code: "B", Message: "second", Severity: "error"},
		}
		joined := errors.Join(ToErrors(errs)...)
		if joined == nil {
			t.Error("expected non-nil joined error")
		}
	})
}

// TestWithContextForkSafety verifies that forking from the same base does not corrupt either fork.
func TestWithContextForkSafety(t *testing.T) {
	base := NewError("E001", "base").WithContext("shared", "original")
	a := base.WithContext("k1", "v1")
	b := base.WithContext("k2", "v2")

	// a should have shared+k1, b should have shared+k2
	if a.Context["k2"] != "" {
		t.Errorf("fork a leaked k2: got %q", a.Context["k2"])
	}
	if b.Context["k1"] != "" {
		t.Errorf("fork b leaked k1: got %q", b.Context["k1"])
	}
	if a.Context["shared"] != "original" {
		t.Errorf("a lost shared: got %q", a.Context["shared"])
	}
	if b.Context["shared"] != "original" {
		t.Errorf("b lost shared: got %q", b.Context["shared"])
	}
	// base should be unchanged
	if len(base.Context) != 1 {
		t.Errorf("base context mutated: len=%d, want 1", len(base.Context))
	}
}


// ptr is a helper function to create a pointer to a value.
func ptr[T any](v T) *T {
	return &v
}
