package autonomy

import (
	"testing"
)

// TestShouldAutoApprove_Gated verifies that every stage returns false when
// the autonomy level is gated.
func TestShouldAutoApprove_Gated(t *testing.T) {
	stages := []Stage{
		StageIMPLReview,
		StageWaveAdvance,
		StageGateFailure,
		StageQueueAdvance,
	}
	for _, s := range stages {
		if got := ShouldAutoApprove(LevelGated, s); got != false {
			t.Errorf("ShouldAutoApprove(gated, %s) = %v, want false", s, got)
		}
	}
}

// TestShouldAutoApprove_Supervised verifies the per-stage matrix for the
// supervised level.
func TestShouldAutoApprove_Supervised(t *testing.T) {
	tests := []struct {
		stage Stage
		want  bool
	}{
		{StageIMPLReview, false},
		{StageWaveAdvance, true},
		{StageGateFailure, true},
		{StageQueueAdvance, true},
	}
	for _, tc := range tests {
		got := ShouldAutoApprove(LevelSupervised, tc.stage)
		if got != tc.want {
			t.Errorf("ShouldAutoApprove(supervised, %s) = %v, want %v", tc.stage, got, tc.want)
		}
	}
}

// TestShouldAutoApprove_Autonomous verifies that every stage returns true when
// the autonomy level is autonomous.
func TestShouldAutoApprove_Autonomous(t *testing.T) {
	stages := []Stage{
		StageIMPLReview,
		StageWaveAdvance,
		StageGateFailure,
		StageQueueAdvance,
	}
	for _, s := range stages {
		if got := ShouldAutoApprove(LevelAutonomous, s); got != true {
			t.Errorf("ShouldAutoApprove(autonomous, %s) = %v, want true", s, got)
		}
	}
}

// TestEffectiveLevel_Override verifies that a valid override string takes
// precedence over the config level.
func TestEffectiveLevel_Override(t *testing.T) {
	cfg := Config{Level: LevelGated}

	got := EffectiveLevel(cfg, "supervised")
	if got != LevelSupervised {
		t.Errorf("EffectiveLevel(gated, \"supervised\") = %q, want %q", got, LevelSupervised)
	}

	got = EffectiveLevel(cfg, "autonomous")
	if got != LevelAutonomous {
		t.Errorf("EffectiveLevel(gated, \"autonomous\") = %q, want %q", got, LevelAutonomous)
	}
}

// TestEffectiveLevel_NoOverride verifies that an empty (or invalid) override
// falls back to cfg.Level.
func TestEffectiveLevel_NoOverride(t *testing.T) {
	cfg := Config{Level: LevelSupervised}

	// Empty override → use config level.
	got := EffectiveLevel(cfg, "")
	if got != LevelSupervised {
		t.Errorf("EffectiveLevel(supervised, \"\") = %q, want %q", got, LevelSupervised)
	}

	// Invalid override → use config level.
	got = EffectiveLevel(cfg, "unknown")
	if got != LevelSupervised {
		t.Errorf("EffectiveLevel(supervised, \"unknown\") = %q, want %q", got, LevelSupervised)
	}
}

// TestParseLevel_Valid verifies that all three canonical level strings parse
// without error.
func TestParseLevel_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"gated", LevelGated},
		{"supervised", LevelSupervised},
		{"autonomous", LevelAutonomous},
	}
	for _, tc := range tests {
		got, err := ParseLevel(tc.input)
		if err != nil {
			t.Errorf("ParseLevel(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseLevel(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestParseLevel_Invalid verifies that an unknown string returns an error.
func TestParseLevel_Invalid(t *testing.T) {
	invalids := []string{"", "GATED", "fully_auto", "none", "  supervised"}
	for _, s := range invalids {
		_, err := ParseLevel(s)
		if err == nil {
			t.Errorf("ParseLevel(%q) expected error, got nil", s)
		}
	}
}
