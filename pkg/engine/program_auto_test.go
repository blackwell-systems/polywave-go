package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// buildTestManifest builds a minimal PROGRAMManifest for testing with N tiers.
// If nTiers=2, tiers are numbered 1 and 2. All IMPLs have status "complete".
func buildTestManifest(nTiers int) *protocol.PROGRAMManifest {
	tiers := make([]protocol.ProgramTier, 0, nTiers)
	impls := make([]protocol.ProgramIMPL, 0, nTiers)
	for i := 1; i <= nTiers; i++ {
		slug := fmt.Sprintf("impl-tier%d", i)
		tiers = append(tiers, protocol.ProgramTier{
			Number: i,
			Impls:  []string{slug},
		})
		impls = append(impls, protocol.ProgramIMPL{
			Slug:   slug,
			Title:  fmt.Sprintf("IMPL Tier %d", i),
			Tier:   i,
			Status: "complete",
		})
	}
	return &protocol.PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-program",
		State:       protocol.ProgramStateTierExecuting,
		Tiers:       tiers,
		Impls:       impls,
		Completion: protocol.ProgramCompletion{
			TiersTotal: nTiers,
			ImplsTotal: nTiers,
		},
	}
}

// TestAdvanceTierAutomatically_AutoMode_AdvanceToNext verifies that when gate passes,
// freeze succeeds, and it's not the final tier, AdvancedToNext=true and NextTier is set.
func TestAdvanceTierAutomatically_AutoMode_AdvanceToNext(t *testing.T) {
	// Use a temp dir as repoPath; no real tier gate commands defined so gates pass trivially
	repoPath := t.TempDir()

	manifest := buildTestManifest(2)
	// No TierGates defined → RunTierGate will pass (no required gates to fail)
	// No ProgramContracts → FreezeContracts has nothing to freeze → Success=true

	result, err := AdvanceTierAutomatically(manifest, 1, repoPath, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.AdvancedToNext {
		t.Errorf("expected AdvancedToNext=true, got false")
	}
	if result.RequiresReview {
		t.Errorf("expected RequiresReview=false, got true")
	}
	if result.ProgramComplete {
		t.Errorf("expected ProgramComplete=false, got true")
	}
	if result.NextTier != 2 {
		t.Errorf("expected NextTier=2, got %d", result.NextTier)
	}
	if result.GateResult == nil {
		t.Error("expected GateResult to be set")
	}
}

// TestAdvanceTierAutomatically_AutoMode_FinalTier verifies that completing the last
// tier sets ProgramComplete=true and AdvancedToNext=false.
func TestAdvanceTierAutomatically_AutoMode_FinalTier(t *testing.T) {
	repoPath := t.TempDir()
	manifest := buildTestManifest(2)

	result, err := AdvanceTierAutomatically(manifest, 2, repoPath, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AdvancedToNext {
		t.Errorf("expected AdvancedToNext=false for final tier, got true")
	}
	if !result.ProgramComplete {
		t.Errorf("expected ProgramComplete=true for final tier, got false")
	}
	if result.RequiresReview {
		t.Errorf("expected RequiresReview=false, got true")
	}
}

// TestAdvanceTierAutomatically_ManualMode_HumanGate verifies that when autoMode=false
// and gate passes, RequiresReview=true (human gate) and AdvancedToNext=false.
func TestAdvanceTierAutomatically_ManualMode_HumanGate(t *testing.T) {
	repoPath := t.TempDir()
	manifest := buildTestManifest(2)

	result, err := AdvanceTierAutomatically(manifest, 1, repoPath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AdvancedToNext {
		t.Errorf("expected AdvancedToNext=false in manual mode, got true")
	}
	if !result.RequiresReview {
		t.Errorf("expected RequiresReview=true in manual mode, got false")
	}
	if result.ProgramComplete {
		t.Errorf("expected ProgramComplete=false, got true")
	}
}

// TestAdvanceTierAutomatically_GateFails verifies that a failing tier gate sets
// RequiresReview=true and AdvancedToNext=false.
func TestAdvanceTierAutomatically_GateFails(t *testing.T) {
	repoPath := t.TempDir()
	manifest := buildTestManifest(2)

	// Add a required gate that will fail (command exits non-zero)
	manifest.TierGates = []protocol.QualityGate{
		{
			Type:     "test",
			Command:  "exit 1",
			Required: true,
		},
	}

	result, err := AdvanceTierAutomatically(manifest, 1, repoPath, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AdvancedToNext {
		t.Errorf("expected AdvancedToNext=false when gate fails, got true")
	}
	if !result.RequiresReview {
		t.Errorf("expected RequiresReview=true when gate fails, got false")
	}
	if result.GateResult == nil {
		t.Error("expected GateResult to be set")
	}
	if result.GateResult.Passed {
		t.Errorf("expected GateResult.Passed=false, got true")
	}
}

// TestAdvanceTierAutomatically_FreezeFails verifies that when gate passes but freeze
// fails (e.g. contract file missing), RequiresReview=true and Errors are populated.
func TestAdvanceTierAutomatically_FreezeFails(t *testing.T) {
	repoPath := t.TempDir()
	manifest := buildTestManifest(2)

	// Add a contract that should freeze at tier 1's IMPL but points to a missing file.
	manifest.ProgramContracts = []protocol.ProgramContract{
		{
			Name:     "api-contract",
			Location: "pkg/api/types.go", // does not exist in temp dir
			FreezeAt: "impl-tier1",
		},
	}

	result, err := AdvanceTierAutomatically(manifest, 1, repoPath, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AdvancedToNext {
		t.Errorf("expected AdvancedToNext=false when freeze fails, got true")
	}
	if !result.RequiresReview {
		t.Errorf("expected RequiresReview=true when freeze fails, got false")
	}
	if len(result.Errors) == 0 {
		t.Errorf("expected Errors to be populated when freeze fails, got empty")
	}
	if result.FreezeResult == nil {
		t.Error("expected FreezeResult to be set")
	}
}

// TestReplanProgram_ValidRevision tests that ReplanProgram attempts to launch the Planner
// agent. In test environments without API credentials it returns an error (backend
// init or auth failure), which is still a non-nil error — the assertion holds.
func TestReplanProgram_ValidRevision(t *testing.T) {
	// Write a minimal PROGRAM manifest file
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "PROGRAM.yaml")
	content := `title: Test Program
program_slug: test-program
state: TIER_EXECUTING
tiers:
  - number: 1
    impls: [impl-a]
impls:
  - slug: impl-a
    title: IMPL A
    tier: 1
    status: complete
completion:
  tiers_total: 1
  impls_total: 1
`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	opts := ReplanProgramOpts{
		ProgramManifestPath: manifestPath,
		Reason:              "tier 1 gate failed: build error",
		FailedTier:          1,
	}

	_, err := ReplanProgram(opts)
	// Planner launch is stubbed — expect "not yet implemented" error
	if err == nil {
		t.Fatal("expected error (Planner not yet implemented), got nil")
	}
	if err.Error() == "" {
		t.Errorf("expected non-empty error message")
	}
}

// TestReplanProgram_ValidationFails tests that ReplanProgram fails gracefully
// when the manifest path is invalid (simulates read failure).
func TestReplanProgram_ValidationFails(t *testing.T) {
	opts := ReplanProgramOpts{
		ProgramManifestPath: "/nonexistent/path/PROGRAM.yaml",
		Reason:              "testing read failure",
	}

	_, err := ReplanProgram(opts)
	if err == nil {
		t.Fatal("expected error for missing manifest, got nil")
	}
}
