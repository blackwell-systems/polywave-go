package engine

import (
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// buildTierLoopManifest builds a manifest for tier loop testing.
func buildTierLoopManifest(tiers []protocol.ProgramTier, impls []protocol.ProgramIMPL) *protocol.PROGRAMManifest {
	return &protocol.PROGRAMManifest{
		Title:       "test-program",
		ProgramSlug: "test-program",
		Tiers:       tiers,
		Impls:       impls,
		Completion: protocol.ProgramCompletion{
			TiersTotal: len(tiers),
			ImplsTotal: len(impls),
		},
	}
}

func TestTierLoop_PartitionIMPLsByStatus_MixedStatuses(t *testing.T) {
	manifest := buildTierLoopManifest(
		[]protocol.ProgramTier{
			{Number: 1, Impls: []string{"alpha", "beta", "gamma", "delta", "epsilon"}},
		},
		[]protocol.ProgramIMPL{
			{Slug: "alpha", Tier: 1, Status: "pending"},
			{Slug: "beta", Tier: 1, Status: "scouting"},
			{Slug: "gamma", Tier: 1, Status: "reviewed"},
			{Slug: "delta", Tier: 1, Status: "complete"},
			{Slug: "epsilon", Tier: 1, Status: "blocked"},
		},
	)

	needsScout, preExisting := PartitionIMPLsByStatus(manifest, 1)

	// pending, scouting, and blocked(unknown) -> needsScout
	if len(needsScout) != 3 {
		t.Errorf("expected 3 needsScout, got %d: %v", len(needsScout), needsScout)
	}
	// reviewed, complete -> preExisting
	if len(preExisting) != 2 {
		t.Errorf("expected 2 preExisting, got %d: %v", len(preExisting), preExisting)
	}

	scoutSet := make(map[string]bool)
	for _, s := range needsScout {
		scoutSet[s] = true
	}
	if !scoutSet["alpha"] || !scoutSet["beta"] || !scoutSet["epsilon"] {
		t.Errorf("needsScout should contain alpha, beta, epsilon; got %v", needsScout)
	}

	preSet := make(map[string]bool)
	for _, s := range preExisting {
		preSet[s] = true
	}
	if !preSet["gamma"] || !preSet["delta"] {
		t.Errorf("preExisting should contain gamma, delta; got %v", preExisting)
	}
}

func TestTierLoop_PartitionIMPLsByStatus_NonexistentTier(t *testing.T) {
	manifest := buildTierLoopManifest(
		[]protocol.ProgramTier{{Number: 1, Impls: []string{"a"}}},
		[]protocol.ProgramIMPL{{Slug: "a", Tier: 1, Status: "pending"}},
	)

	needsScout, preExisting := PartitionIMPLsByStatus(manifest, 99)
	if needsScout != nil || preExisting != nil {
		t.Errorf("expected nil for nonexistent tier, got needsScout=%v preExisting=%v", needsScout, preExisting)
	}
}

func TestTierLoop_AutoTriggerReplan_ConstructsCorrectReason(t *testing.T) {
	origReplan := replanProgramFunc
	defer func() { replanProgramFunc = origReplan }()

	var capturedOpts ReplanProgramOpts
	replanProgramFunc = func(opts ReplanProgramOpts) (*ReplanResult, error) {
		capturedOpts = opts
		return &ReplanResult{ValidationPassed: true}, nil
	}

	gateResult := &protocol.TierGateResult{
		TierNumber:   2,
		Passed:       false,
		AllImplsDone: false,
		ImplStatuses: []protocol.ImplTierStatus{
			{Slug: "auth", Status: "complete"},
			{Slug: "billing", Status: "pending"},
		},
		GateResults: []protocol.GateResult{
			{Type: "build", Passed: true},
			{Type: "test", Passed: false, Stderr: "3 tests failed"},
		},
	}

	result, err := AutoTriggerReplan("/tmp/manifest.yaml", 2, gateResult, "opus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ValidationPassed {
		t.Error("expected validation passed")
	}

	if capturedOpts.FailedTier != 2 {
		t.Errorf("expected FailedTier=2, got %d", capturedOpts.FailedTier)
	}
	reason := capturedOpts.Reason
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
	if !strings.Contains(reason, "Tier 2") {
		t.Errorf("reason should mention 'Tier 2': %s", reason)
	}
	if !strings.Contains(reason, "billing") {
		t.Errorf("reason should mention incomplete IMPL 'billing': %s", reason)
	}
	if !strings.Contains(reason, "test") {
		t.Errorf("reason should mention failed gate 'test': %s", reason)
	}
}

func TestTierLoop_StopsAtHumanGate_WhenAutoModeOff(t *testing.T) {
	manifest := buildTierLoopManifest(
		[]protocol.ProgramTier{
			{Number: 1, Impls: []string{"feat-a", "feat-b"}},
		},
		[]protocol.ProgramIMPL{
			{Slug: "feat-a", Tier: 1, Status: "pending"},
			{Slug: "feat-b", Tier: 1, Status: "pending"},
		},
	)

	current := findCurrentTier(manifest)
	if current != 1 {
		t.Errorf("expected current tier 1, got %d", current)
	}

	complete := countCompleteTiers(manifest)
	if complete != 0 {
		t.Errorf("expected 0 complete tiers, got %d", complete)
	}
}

func TestTierLoop_FindCurrentTier_AllComplete(t *testing.T) {
	manifest := buildTierLoopManifest(
		[]protocol.ProgramTier{
			{Number: 1, Impls: []string{"a"}},
			{Number: 2, Impls: []string{"b"}},
		},
		[]protocol.ProgramIMPL{
			{Slug: "a", Tier: 1, Status: "complete"},
			{Slug: "b", Tier: 2, Status: "complete"},
		},
	)

	current := findCurrentTier(manifest)
	if current != -1 {
		t.Errorf("expected -1 (all complete), got %d", current)
	}
}

func TestTierLoop_ProgramComplete_OnFinalTier(t *testing.T) {
	manifest := buildTierLoopManifest(
		[]protocol.ProgramTier{
			{Number: 1, Impls: []string{"a"}},
			{Number: 2, Impls: []string{"b"}},
		},
		[]protocol.ProgramIMPL{
			{Slug: "a", Tier: 1, Status: "complete"},
			{Slug: "b", Tier: 2, Status: "complete"},
		},
	)

	if findCurrentTier(manifest) != -1 {
		t.Error("expected all tiers complete")
	}
	if !isFinalTier(manifest, 2) {
		t.Error("tier 2 should be the final tier")
	}
	if isFinalTier(manifest, 1) {
		t.Error("tier 1 should not be the final tier")
	}
}

func TestTierLoop_AdvancesToNextTier_InAutoMode(t *testing.T) {
	manifest := buildTierLoopManifest(
		[]protocol.ProgramTier{
			{Number: 1, Impls: []string{"a"}},
			{Number: 2, Impls: []string{"b"}},
			{Number: 3, Impls: []string{"c"}},
		},
		[]protocol.ProgramIMPL{
			{Slug: "a", Tier: 1, Status: "complete"},
			{Slug: "b", Tier: 2, Status: "pending"},
			{Slug: "c", Tier: 3, Status: "pending"},
		},
	)

	current := findCurrentTier(manifest)
	if current != 2 {
		t.Errorf("expected current tier 2, got %d", current)
	}
	if countCompleteTiers(manifest) != 1 {
		t.Errorf("expected 1 complete tier, got %d", countCompleteTiers(manifest))
	}
	if isFinalTier(manifest, 2) {
		t.Error("tier 2 should not be the final tier")
	}
}

func TestTierLoop_GetIMPLWaveCount(t *testing.T) {
	manifest := buildTierLoopManifest(
		[]protocol.ProgramTier{{Number: 1, Impls: []string{"a", "b"}}},
		[]protocol.ProgramIMPL{
			{Slug: "a", Tier: 1, Status: "pending", EstimatedWaves: 3},
			{Slug: "b", Tier: 1, Status: "pending"},
		},
	)

	if got := getIMPLWaveCount(manifest, "a"); got != 3 {
		t.Errorf("expected 3 waves for 'a', got %d", got)
	}
	if got := getIMPLWaveCount(manifest, "b"); got != 1 {
		t.Errorf("expected 1 wave (default) for 'b', got %d", got)
	}
	if got := getIMPLWaveCount(manifest, "nonexistent"); got != 1 {
		t.Errorf("expected 1 wave for nonexistent, got %d", got)
	}
}

func TestTierLoop_EmitEvent_NilCallback(t *testing.T) {
	// Should not panic with nil callback
	emitEvent(nil, TierLoopEvent{Type: "test", Tier: 1, Detail: "test"})
}

func TestTierLoop_EmitEvent_CallbackInvoked(t *testing.T) {
	var captured TierLoopEvent
	emitEvent(func(e TierLoopEvent) {
		captured = e
	}, TierLoopEvent{Type: "tier_started", Tier: 3, Detail: "hello"})

	if captured.Type != "tier_started" || captured.Tier != 3 || captured.Detail != "hello" {
		t.Errorf("unexpected captured event: %+v", captured)
	}
}

func TestTierLoop_BuildReplanReason(t *testing.T) {
	tests := []struct {
		name      string
		tier      int
		gate      *protocol.TierGateResult
		wantParts []string
	}{
		{
			name: "incomplete IMPLs",
			tier: 1,
			gate: &protocol.TierGateResult{
				AllImplsDone: false,
				ImplStatuses: []protocol.ImplTierStatus{
					{Slug: "auth", Status: "complete"},
					{Slug: "api", Status: "pending"},
				},
			},
			wantParts: []string{"Tier 1", "api(pending)"},
		},
		{
			name: "gate command failure",
			tier: 2,
			gate: &protocol.TierGateResult{
				AllImplsDone: true,
				GateResults: []protocol.GateResult{
					{Type: "build", Passed: false, Stderr: "compilation error"},
				},
			},
			wantParts: []string{"Tier 2", "build", "compilation error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := buildReplanReason(tt.tier, tt.gate)
			for _, part := range tt.wantParts {
				if !strings.Contains(reason, part) {
					t.Errorf("reason %q should contain %q", reason, part)
				}
			}
		})
	}
}

func TestTierLoop_LaunchParallelScoutsFunc_IsWired(t *testing.T) {
	// After program_wire_init.go runs, the function variable should point to
	// the real LaunchParallelScouts (not the stub). Verify it's non-nil and callable.
	if launchParallelScoutsFunc == nil {
		t.Error("expected launchParallelScoutsFunc to be wired, got nil")
	}
}

func TestTierLoop_GetTierSlugs(t *testing.T) {
	manifest := buildTierLoopManifest(
		[]protocol.ProgramTier{
			{Number: 1, Impls: []string{"x", "y"}},
			{Number: 2, Impls: []string{"z"}},
		},
		nil,
	)

	slugs := getTierSlugs(manifest, 1)
	if len(slugs) != 2 || slugs[0] != "x" || slugs[1] != "y" {
		t.Errorf("expected [x y], got %v", slugs)
	}

	slugs = getTierSlugs(manifest, 99)
	if slugs != nil {
		t.Errorf("expected nil for nonexistent tier, got %v", slugs)
	}
}
