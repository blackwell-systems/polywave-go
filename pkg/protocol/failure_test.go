package protocol

import "testing"

func TestShouldRetry_Transient(t *testing.T) {
	if !ShouldRetry(FailureTransient) {
		t.Error("transient failures should be retryable")
	}
}

func TestShouldRetry_Fixable(t *testing.T) {
	if !ShouldRetry(FailureFixable) {
		t.Error("fixable failures should be retryable")
	}
}

func TestShouldRetry_Timeout(t *testing.T) {
	if !ShouldRetry(FailureTimeout) {
		t.Error("timeout failures should be retryable")
	}
}

func TestShouldRetry_NeedsReplan(t *testing.T) {
	if ShouldRetry(FailureNeedsReplan) {
		t.Error("needs_replan failures should not be retryable")
	}
}

func TestShouldRetry_Escalate(t *testing.T) {
	if ShouldRetry(FailureEscalate) {
		t.Error("escalate failures should not be retryable")
	}
}

func TestShouldRetry_Unknown(t *testing.T) {
	if ShouldRetry(FailureTypeEnum("unknown")) {
		t.Error("unknown failure types should not be retryable")
	}
}

func TestMaxRetries_Values(t *testing.T) {
	tests := []struct {
		failureType FailureTypeEnum
		expected    int
	}{
		{FailureTransient, 2},
		{FailureFixable, 2},
		{FailureTimeout, 1},
		{FailureNeedsReplan, 0},
		{FailureEscalate, 0},
		{FailureTypeEnum("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.failureType), func(t *testing.T) {
			got := MaxRetries(tt.failureType)
			if got != tt.expected {
				t.Errorf("MaxRetries(%s) = %d, want %d", tt.failureType, got, tt.expected)
			}
		})
	}
}

func TestActionRequired_AllTypes(t *testing.T) {
	tests := []struct {
		failureType FailureTypeEnum
		expected    string
	}{
		{FailureTransient, "Retry automatically"},
		{FailureFixable, "Apply agent's suggested fix, then retry"},
		{FailureTimeout, "Retry with scope-reduction instructions"},
		{FailureNeedsReplan, "Re-engage Scout with agent's completion report"},
		{FailureEscalate, "Surface to human immediately"},
		{FailureTypeEnum("unknown"), "Unknown failure type: escalate to human"},
	}

	for _, tt := range tests {
		t.Run(string(tt.failureType), func(t *testing.T) {
			got := ActionRequired(tt.failureType)
			if got != tt.expected {
				t.Errorf("ActionRequired(%s) = %q, want %q", tt.failureType, got, tt.expected)
			}
			if got == "" {
				t.Errorf("ActionRequired(%s) returned empty string", tt.failureType)
			}
		})
	}
}

func TestValidFailureType(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"transient", true},
		{"fixable", true},
		{"needs_replan", true},
		{"escalate", true},
		{"timeout", true},
		{"unknown", false},
		{"", false},
		{"TRANSIENT", false},
		{"Transient", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ValidFailureType(tt.input)
			if got != tt.expected {
				t.Errorf("ValidFailureType(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestReactionEntryFor_AllFailureTypes verifies that all 5 failure types route
// to the correct field in ReactionsConfig.
func TestReactionEntryFor_AllFailureTypes(t *testing.T) {
	transientEntry := &ReactionEntry{Action: "retry"}
	timeoutEntry := &ReactionEntry{Action: "send-fix-prompt"}
	fixableEntry := &ReactionEntry{Action: "pause"}
	needsReplanEntry := &ReactionEntry{Action: "auto-scout"}
	escalateEntry := &ReactionEntry{Action: "pause"}

	reactions := &ReactionsConfig{
		Transient:   transientEntry,
		Timeout:     timeoutEntry,
		Fixable:     fixableEntry,
		NeedsReplan: needsReplanEntry,
		Escalate:    escalateEntry,
	}

	tests := []struct {
		ft       FailureTypeEnum
		expected *ReactionEntry
	}{
		{FailureTransient, transientEntry},
		{FailureTimeout, timeoutEntry},
		{FailureFixable, fixableEntry},
		{FailureNeedsReplan, needsReplanEntry},
		{FailureEscalate, escalateEntry},
		{FailureTypeEnum("unknown"), nil},
	}

	for _, tt := range tests {
		t.Run(string(tt.ft), func(t *testing.T) {
			got := reactionEntryFor(tt.ft, reactions)
			if got != tt.expected {
				t.Errorf("reactionEntryFor(%s) = %v, want %v", tt.ft, got, tt.expected)
			}
		})
	}

	// Also verify nil reactions returns nil for all types
	for _, tt := range tests {
		t.Run("nil_reactions_"+string(tt.ft), func(t *testing.T) {
			got := reactionEntryFor(tt.ft, nil)
			if got != nil {
				t.Errorf("reactionEntryFor(%s, nil) = %v, want nil", tt.ft, got)
			}
		})
	}
}

// TestFailureTypeDecisionTree validates the complete decision tree logic (E19).
func TestFailureTypeDecisionTree(t *testing.T) {
	tests := []struct {
		failureType   FailureTypeEnum
		shouldRetry   bool
		maxRetries    int
		actionPrefix  string
		validType     bool
	}{
		{
			failureType:   FailureTransient,
			shouldRetry:   true,
			maxRetries:    2,
			actionPrefix:  "Retry automatically",
			validType:     true,
		},
		{
			failureType:   FailureFixable,
			shouldRetry:   true,
			maxRetries:    2,
			actionPrefix:  "Apply agent's suggested fix",
			validType:     true,
		},
		{
			failureType:   FailureTimeout,
			shouldRetry:   true,
			maxRetries:    1,
			actionPrefix:  "Retry with scope-reduction",
			validType:     true,
		},
		{
			failureType:   FailureNeedsReplan,
			shouldRetry:   false,
			maxRetries:    0,
			actionPrefix:  "Re-engage Scout",
			validType:     true,
		},
		{
			failureType:   FailureEscalate,
			shouldRetry:   false,
			maxRetries:    0,
			actionPrefix:  "Surface to human",
			validType:     true,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.failureType), func(t *testing.T) {
			// Test ShouldRetry
			if got := ShouldRetry(tt.failureType); got != tt.shouldRetry {
				t.Errorf("ShouldRetry(%s) = %v, want %v", tt.failureType, got, tt.shouldRetry)
			}

			// Test MaxRetries
			if got := MaxRetries(tt.failureType); got != tt.maxRetries {
				t.Errorf("MaxRetries(%s) = %d, want %d", tt.failureType, got, tt.maxRetries)
			}

			// Test ActionRequired (check prefix to avoid repeating full strings)
			action := ActionRequired(tt.failureType)
			if len(action) == 0 {
				t.Errorf("ActionRequired(%s) returned empty string", tt.failureType)
			}

			// Test ValidFailureType
			if got := ValidFailureType(string(tt.failureType)); got != tt.validType {
				t.Errorf("ValidFailureType(%s) = %v, want %v", tt.failureType, got, tt.validType)
			}
		})
	}
}
