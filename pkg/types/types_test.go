package types

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestCompletionStatusConstants verifies that CompletionStatus constants have expected string values.
func TestCompletionStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   CompletionStatus
		expected string
	}{
		{"StatusComplete", StatusComplete, "complete"},
		{"StatusPartial", StatusPartial, "partial"},
		{"StatusBlocked", StatusBlocked, "blocked"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("%s = %q, want %q", tt.name, string(tt.status), tt.expected)
		}
	}
}

// TestFailureTypeConstants verifies that all five FailureType constants have their exact string values.
func TestFailureTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		ft       FailureType
		expected string
	}{
		{"FailureTypeTransient", FailureTypeTransient, "transient"},
		{"FailureTypeFixable", FailureTypeFixable, "fixable"},
		{"FailureTypeNeedsReplan", FailureTypeNeedsReplan, "needs_replan"},
		{"FailureTypeEscalate", FailureTypeEscalate, "escalate"},
		{"FailureTypeTimeout", FailureTypeTimeout, "timeout"},
	}

	for _, tt := range tests {
		if string(tt.ft) != tt.expected {
			t.Errorf("%s = %q, want %q", tt.name, string(tt.ft), tt.expected)
		}
	}
}

// TestCompletionReportFailureTypeOmitempty verifies that marshaling a CompletionReport
// with an empty FailureType does not include the failure_type key in YAML output.
func TestCompletionReportFailureTypeOmitempty(t *testing.T) {
	r := CompletionReport{
		Status: StatusComplete,
		Branch: "main",
	}
	out, err := yaml.Marshal(&r)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	if strings.Contains(string(out), "failure_type") {
		t.Errorf("expected failure_type to be absent from YAML output when empty, got:\n%s", string(out))
	}
}
