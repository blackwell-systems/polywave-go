package protocol

import "testing"

func TestValidateReactions_Nil(t *testing.T) {
	m := &IMPLManifest{Reactions: nil}
	errs := ValidateReactions(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for nil reactions, got %d: %v", len(errs), errs)
	}
}

func TestValidateReactions_ValidBlock(t *testing.T) {
	m := &IMPLManifest{
		Reactions: &ReactionsConfig{
			Transient:   &ReactionEntry{Action: "retry", MaxAttempts: 3},
			Timeout:     &ReactionEntry{Action: "send-fix-prompt", MaxAttempts: 2},
			Fixable:     &ReactionEntry{Action: "pause"},
			NeedsReplan: &ReactionEntry{Action: "auto-scout"},
			Escalate:    &ReactionEntry{Action: "pause"},
		},
	}
	errs := ValidateReactions(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid reactions block, got %d: %v", len(errs), errs)
	}
}

func TestValidateReactions_InvalidAction(t *testing.T) {
	m := &IMPLManifest{
		Reactions: &ReactionsConfig{
			Transient: &ReactionEntry{Action: "explode"},
		},
	}
	errs := ValidateReactions(m)
	if len(errs) == 0 {
		t.Fatal("expected an error for invalid action, got none")
	}
	found := false
	for _, e := range errs {
		if e.Code == SV01InvalidEnum && e.Field == "reactions.transient.action" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SV01_INVALID_ENUM for reactions.transient.action, got: %v", errs)
	}
}

func TestValidateReactions_NegativeMaxAttempts(t *testing.T) {
	m := &IMPLManifest{
		Reactions: &ReactionsConfig{
			Fixable: &ReactionEntry{Action: "retry", MaxAttempts: -1},
		},
	}
	errs := ValidateReactions(m)
	if len(errs) == 0 {
		t.Fatal("expected an error for negative max_attempts, got none")
	}
	found := false
	for _, e := range errs {
		if e.Code == SV01RequiredField && e.Field == "reactions.fixable.max_attempts" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SV01_REQUIRED_FIELD for reactions.fixable.max_attempts, got: %v", errs)
	}
}

func TestValidateReactions_PartialBlock(t *testing.T) {
	m := &IMPLManifest{
		Reactions: &ReactionsConfig{
			Transient: &ReactionEntry{Action: "retry", MaxAttempts: 5},
			// all other entries nil
		},
	}
	errs := ValidateReactions(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for partial block with only transient set, got %d: %v", len(errs), errs)
	}
}

func TestMaxRetriesWithReactions_NilReactions(t *testing.T) {
	tests := []struct {
		ft       FailureTypeEnum
		expected int
	}{
		{FailureTransient, 2},
		{FailureFixable, 2},
		{FailureTimeout, 1},
		{FailureNeedsReplan, 0},
		{FailureEscalate, 0},
	}
	for _, tt := range tests {
		t.Run(string(tt.ft), func(t *testing.T) {
			got := MaxRetriesWithReactions(tt.ft, nil)
			if got != tt.expected {
				t.Errorf("MaxRetriesWithReactions(%s, nil) = %d, want %d", tt.ft, got, tt.expected)
			}
		})
	}
}

func TestMaxRetriesWithReactions_Override(t *testing.T) {
	reactions := &ReactionsConfig{
		Transient: &ReactionEntry{Action: "retry", MaxAttempts: 5},
	}
	got := MaxRetriesWithReactions(FailureTransient, reactions)
	if got != 5 {
		t.Errorf("MaxRetriesWithReactions(transient, reactions) = %d, want 5", got)
	}
}

func TestMaxRetriesWithReactions_ZeroMaxAttempts(t *testing.T) {
	reactions := &ReactionsConfig{
		Transient: &ReactionEntry{Action: "retry", MaxAttempts: 0},
	}
	// Zero means "use E19 default", so should return MaxRetries(transient) = 2
	got := MaxRetriesWithReactions(FailureTransient, reactions)
	if got != 2 {
		t.Errorf("MaxRetriesWithReactions(transient, reactions{MaxAttempts:0}) = %d, want 2 (E19 default)", got)
	}
}

func TestShouldRetryWithReactions_NilReactions(t *testing.T) {
	tests := []struct {
		ft       FailureTypeEnum
		expected bool
	}{
		{FailureTransient, true},
		{FailureFixable, true},
		{FailureTimeout, true},
		{FailureNeedsReplan, false},
		{FailureEscalate, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.ft), func(t *testing.T) {
			got := ShouldRetryWithReactions(tt.ft, nil)
			if got != tt.expected {
				t.Errorf("ShouldRetryWithReactions(%s, nil) = %v, want %v", tt.ft, got, tt.expected)
			}
		})
	}
}

func TestShouldRetryWithReactions_PauseAction(t *testing.T) {
	reactions := &ReactionsConfig{
		Transient: &ReactionEntry{Action: "pause"},
	}
	got := ShouldRetryWithReactions(FailureTransient, reactions)
	if got {
		t.Error("ShouldRetryWithReactions(transient, {action:pause}) should return false")
	}
}
