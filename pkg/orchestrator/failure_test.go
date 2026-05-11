package orchestrator

import (
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// TestRouteFailureAllTypes verifies that all five known failure types route
// to their expected OrchestratorAction per E19.
func TestRouteFailureAllTypes(t *testing.T) {
	tests := []struct {
		name        string
		failureType protocol.FailureTypeEnum
		want        OrchestratorAction
	}{
		{
			name:        "transient routes to ActionRetry",
			failureType: protocol.FailureTransient,
			want:        ActionRetry,
		},
		{
			name:        "fixable routes to ActionApplyAndRelaunch",
			failureType: protocol.FailureFixable,
			want:        ActionApplyAndRelaunch,
		},
		{
			name:        "needs_replan routes to ActionReplan",
			failureType: protocol.FailureNeedsReplan,
			want:        ActionReplan,
		},
		{
			name:        "escalate routes to ActionEscalate",
			failureType: protocol.FailureEscalate,
			want:        ActionEscalate,
		},
		{
			name:        "timeout routes to ActionRetryWithScope",
			failureType: protocol.FailureTimeout,
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
	unknownValues := []protocol.FailureTypeEnum{
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

// TestRouteFailureWithReactions_NilReactions verifies that nil reactions falls
// back to the same result as RouteFailure for all failure types.
func TestRouteFailureWithReactions_NilReactions(t *testing.T) {
	failureTypes := []protocol.FailureTypeEnum{
		protocol.FailureTransient,
		protocol.FailureFixable,
		protocol.FailureNeedsReplan,
		protocol.FailureEscalate,
		protocol.FailureTimeout,
		"",
	}
	for _, ft := range failureTypes {
		got := RouteFailureWithReactions(ft, nil)
		want := RouteFailure(ft)
		if got != want {
			t.Errorf("RouteFailureWithReactions(%q, nil) = %d, want %d (same as RouteFailure)", ft, got, want)
		}
	}
}

// TestRouteFailureWithReactions_RetryOverride verifies that transient with
// action:retry returns ActionRetry.
func TestRouteFailureWithReactions_RetryOverride(t *testing.T) {
	reactions := &protocol.ReactionsConfig{
		Transient: &protocol.ReactionEntry{Action: "retry", MaxAttempts: 3},
	}
	got := RouteFailureWithReactions(protocol.FailureTransient, reactions)
	if got != ActionRetry {
		t.Errorf("RouteFailureWithReactions(transient, retry) = %d, want ActionRetry (%d)", got, ActionRetry)
	}
}

// TestRouteFailureWithReactions_PauseOverride verifies that escalate with
// action:pause returns ActionEscalate (pause surfaces to human).
func TestRouteFailureWithReactions_PauseOverride(t *testing.T) {
	reactions := &protocol.ReactionsConfig{
		Escalate: &protocol.ReactionEntry{Action: "pause"},
	}
	got := RouteFailureWithReactions(protocol.FailureEscalate, reactions)
	if got != ActionEscalate {
		t.Errorf("RouteFailureWithReactions(escalate, pause) = %d, want ActionEscalate (%d)", got, ActionEscalate)
	}
}

// TestRouteFailureWithReactions_AutoScoutOverride verifies that needs_replan with
// action:auto-scout returns ActionReplan.
func TestRouteFailureWithReactions_AutoScoutOverride(t *testing.T) {
	reactions := &protocol.ReactionsConfig{
		NeedsReplan: &protocol.ReactionEntry{Action: "auto-scout"},
	}
	got := RouteFailureWithReactions(protocol.FailureNeedsReplan, reactions)
	if got != ActionReplan {
		t.Errorf("RouteFailureWithReactions(needs_replan, auto-scout) = %d, want ActionReplan (%d)", got, ActionReplan)
	}
}

// TestRouteFailureWithReactions_SendFixPrompt verifies that fixable with
// action:send-fix-prompt returns ActionApplyAndRelaunch.
func TestRouteFailureWithReactions_SendFixPrompt(t *testing.T) {
	reactions := &protocol.ReactionsConfig{
		Fixable: &protocol.ReactionEntry{Action: "send-fix-prompt", MaxAttempts: 1},
	}
	got := RouteFailureWithReactions(protocol.FailureFixable, reactions)
	if got != ActionApplyAndRelaunch {
		t.Errorf("RouteFailureWithReactions(fixable, send-fix-prompt) = %d, want ActionApplyAndRelaunch (%d)", got, ActionApplyAndRelaunch)
	}
}

// TestRouteFailureWithReactions_UnsetEntryFallsThrough verifies that a reactions
// block with no entry for a given failure type falls through to E19 defaults.
func TestRouteFailureWithReactions_UnsetEntryFallsThrough(t *testing.T) {
	// Only transient is set; timeout should fall through to ActionRetryWithScope.
	reactions := &protocol.ReactionsConfig{
		Transient: &protocol.ReactionEntry{Action: "retry"},
	}
	got := RouteFailureWithReactions(protocol.FailureTimeout, reactions)
	if got != ActionRetryWithScope {
		t.Errorf("RouteFailureWithReactions(timeout, reactions without timeout entry) = %d, want ActionRetryWithScope (%d)", got, ActionRetryWithScope)
	}
}

// TestMaxAttemptsFor_NilReactions verifies that nil reactions falls back to
// defaultMaxAttempts for all retryable types.
func TestMaxAttemptsFor_NilReactions(t *testing.T) {
	tests := []struct {
		ft   protocol.FailureTypeEnum
		want int
	}{
		{protocol.FailureTransient, 2},
		{protocol.FailureFixable, 2},
		{protocol.FailureTimeout, 1},
		{protocol.FailureNeedsReplan, 0},
		{protocol.FailureEscalate, 0},
	}
	for _, tt := range tests {
		got := MaxAttemptsFor(tt.ft, nil)
		if got != tt.want {
			t.Errorf("MaxAttemptsFor(%q, nil) = %d, want %d", tt.ft, got, tt.want)
		}
	}
}

// TestMaxAttemptsFor_Override verifies that a non-zero MaxAttempts in the
// reactions entry is returned instead of the E19 default.
func TestMaxAttemptsFor_Override(t *testing.T) {
	reactions := &protocol.ReactionsConfig{
		Transient: &protocol.ReactionEntry{Action: "retry", MaxAttempts: 5},
	}
	got := MaxAttemptsFor(protocol.FailureTransient, reactions)
	if got != 5 {
		t.Errorf("MaxAttemptsFor(transient, reactions{max:5}) = %d, want 5", got)
	}
}

// TestMaxAttemptsFor_ZeroMaxAttempts verifies that MaxAttempts=0 is treated as
// "use E19 default" — not zero retries.
func TestMaxAttemptsFor_ZeroMaxAttempts(t *testing.T) {
	reactions := &protocol.ReactionsConfig{
		Transient: &protocol.ReactionEntry{Action: "retry", MaxAttempts: 0},
	}
	got := MaxAttemptsFor(protocol.FailureTransient, reactions)
	want := defaultMaxAttempts(protocol.FailureTransient) // 2
	if got != want {
		t.Errorf("MaxAttemptsFor(transient, reactions{max:0}) = %d, want %d (E19 default)", got, want)
	}
}

// TestMaxAttemptsFor_TransientDefault verifies that transient with nil reactions
// returns 2.
func TestMaxAttemptsFor_TransientDefault(t *testing.T) {
	got := MaxAttemptsFor(protocol.FailureTransient, nil)
	if got != 2 {
		t.Errorf("MaxAttemptsFor(transient, nil) = %d, want 2", got)
	}
}

// TestActionFromReactionEntry_UnknownAction verifies that an unknown action string
// returns ActionEscalate (conservative fallback).
func TestActionFromReactionEntry_UnknownAction(t *testing.T) {
	entry := &protocol.ReactionEntry{Action: "something-made-up"}
	got := actionFromReactionEntry(entry)
	if got != ActionEscalate {
		t.Errorf("actionFromReactionEntry({action:\"something-made-up\"}) = %d, want ActionEscalate (%d)", got, ActionEscalate)
	}
}
