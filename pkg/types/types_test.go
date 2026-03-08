package types

import "testing"

// TestStateString verifies that State.String() returns correct strings for all states.
func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{ScoutPending, "ScoutPending"},
		{ScoutValidating, "ScoutValidating"},
		{NotSuitable, "NotSuitable"},
		{Reviewed, "Reviewed"},
		{ScaffoldPending, "ScaffoldPending"},
		{WavePending, "WavePending"},
		{WaveExecuting, "WaveExecuting"},
		{WaveMerging, "WaveMerging"},
		{WaveVerified, "WaveVerified"},
		{Blocked, "Blocked"},
		{Complete, "Complete"},
		{State(999), "Unknown"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.expected {
			t.Errorf("State(%d).String() = %q, want %q", int(tt.state), got, tt.expected)
		}
	}
}

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
