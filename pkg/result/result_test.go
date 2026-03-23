package result

import (
	"encoding/json"
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
	warnings := []StructuredError{
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
	errors := []StructuredError{
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
			r: NewPartial("data", []StructuredError{
				{Code: "W001", Message: "warning", Severity: "warning"},
			}),
			want: false,
		},
		{
			name: "failure",
			r: NewFailure[string]([]StructuredError{
				{Code: "E001", Message: "error", Severity: "fatal"},
			}),
			want: false,
		},
		{
			name: "success code but has errors",
			r: Result[string]{
				Data:   ptr("data"),
				Code:   "SUCCESS",
				Errors: []StructuredError{{Code: "E001"}},
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
			r: NewFailure[string]([]StructuredError{
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
			r: NewPartial("data", []StructuredError{
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
			r: NewPartial(123, []StructuredError{
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
			r: NewFailure[int]([]StructuredError{
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
			r: NewPartial("data", []StructuredError{
				{Code: "W001", Message: "warning", Severity: "warning"},
			}),
			want: true,
		},
		{
			name: "has fatal errors",
			r: NewFailure[string]([]StructuredError{
				{Code: "E001", Message: "error", Severity: "fatal"},
			}),
			want: true,
		},
		{
			name: "empty errors slice",
			r: Result[string]{
				Data:   ptr("data"),
				Code:   "SUCCESS",
				Errors: []StructuredError{},
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

// TestGetData verifies GetData method returns data and panics when nil.
func TestGetData(t *testing.T) {
	t.Run("returns data when present", func(t *testing.T) {
		expected := "test data"
		r := NewSuccess(expected)
		got := r.GetData()
		if got != expected {
			t.Errorf("GetData() = %q, want %q", got, expected)
		}
	})

	t.Run("panics when data is nil", func(t *testing.T) {
		r := NewFailure[string]([]StructuredError{
			{Code: "E001", Message: "error", Severity: "fatal"},
		})

		defer func() {
			if r := recover(); r == nil {
				t.Error("GetData() should panic when Data is nil")
			} else if msg, ok := r.(string); !ok || msg != "GetData called on Result with nil Data" {
				t.Errorf("panic message = %v, want 'GetData called on Result with nil Data'", r)
			}
		}()

		_ = r.GetData()
	})
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
			r: NewPartial("partial data", []StructuredError{
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
			r: NewFailure[string]([]StructuredError{
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

// TestStructuredErrorFields verifies StructuredError fields serialize correctly.
func TestStructuredErrorFields(t *testing.T) {
	err := StructuredError{
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

	r := NewFailure[string]([]StructuredError{err})

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

// ptr is a helper function to create a pointer to a value.
func ptr[T any](v T) *T {
	return &v
}
