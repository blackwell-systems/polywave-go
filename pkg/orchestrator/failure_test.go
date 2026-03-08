package orchestrator

import "testing"

// TestRouteFailureAllTypes verifies that all five known failure types route
// to their expected OrchestratorAction per E19.
func TestRouteFailureAllTypes(t *testing.T) {
	tests := []struct {
		name        string
		failureType FailureType
		want        OrchestratorAction
	}{
		{
			name:        "transient routes to ActionRetry",
			failureType: FailureTypeTransient,
			want:        ActionRetry,
		},
		{
			name:        "fixable routes to ActionApplyAndRelaunch",
			failureType: FailureTypeFixable,
			want:        ActionApplyAndRelaunch,
		},
		{
			name:        "needs_replan routes to ActionReplan",
			failureType: FailureTypeNeedsReplan,
			want:        ActionReplan,
		},
		{
			name:        "escalate routes to ActionEscalate",
			failureType: FailureTypeEscalate,
			want:        ActionEscalate,
		},
		{
			name:        "timeout routes to ActionRetryWithScope",
			failureType: FailureTypeTimeout,
			want:        ActionRetryWithScope,
		},
		{
			name:        "empty string routes to ActionEscalate (backward compat)",
			failureType: "",
			want:        ActionEscalate,
		},
		{
			name:        "unknown value routes to ActionEscalate",
			failureType: "something_unknown",
			want:        ActionEscalate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RouteFailure(tt.failureType)
			if got != tt.want {
				t.Errorf("RouteFailure(%q) = %d, want %d", tt.failureType, got, tt.want)
			}
		})
	}
}

// TestRouteFailureEmptyIsEscalate explicitly validates E19 backward-compatibility:
// an absent failure_type field (represented as empty string) must return ActionEscalate.
func TestRouteFailureEmptyIsEscalate(t *testing.T) {
	got := RouteFailure("")
	if got != ActionEscalate {
		t.Errorf("RouteFailure(\"\") = %d, want ActionEscalate (%d)", got, ActionEscalate)
	}
}

// TestRouteFailureUnknownIsEscalate verifies that any unrecognized failure type
// defaults to ActionEscalate rather than silently picking an incorrect action.
func TestRouteFailureUnknownIsEscalate(t *testing.T) {
	unknownValues := []FailureType{
		"TRANSIENT",        // case-sensitive check
		"Fixable",          // mixed case
		"partial",          // completion status, not failure type
		"blocked",          // completion status, not failure type
		"retry",            // action name, not failure type
		"unknown_new_type", // hypothetical future type
	}

	for _, v := range unknownValues {
		got := RouteFailure(v)
		if got != ActionEscalate {
			t.Errorf("RouteFailure(%q) = %d, want ActionEscalate (%d)", v, got, ActionEscalate)
		}
	}
}
