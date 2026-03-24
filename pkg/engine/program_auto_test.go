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

// TestScoreTierIMPLs_SinglePendingNoDownstream verifies that a single pending IMPL
// with no downstream dependents gets score=0 and appears in the returned slice.
func TestScoreTierIMPLs_SinglePendingNoDownstream(t *testing.T) {
	manifest := &protocol.PROGRAMManifest{
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"impl-a"}},
		},
		Impls: []protocol.ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "pending"},
		},
	}

	result := ScoreTierIMPLs(manifest, 1)
	if len(result) != 1 {
		t.Fatalf("expected 1 slug, got %d", len(result))
	}
	if result[0] != "impl-a" {
		t.Errorf("expected 'impl-a', got %q", result[0])
	}
	// Score should be 0 — no downstream dependents
	if manifest.Impls[0].PriorityScore != 0 {
		t.Errorf("expected PriorityScore=0, got %d", manifest.Impls[0].PriorityScore)
	}
}

// TestScoreTierIMPLs_UnblockingIMPLAppearsFirst verifies that when one IMPL
// unblocks another, the unblocking IMPL appears first in the ordered result.
func TestScoreTierIMPLs_UnblockingIMPLAppearsFirst(t *testing.T) {
	// impl-a unblocks impl-c (impl-c depends on impl-a)
	// impl-b has no downstream dependents
	// Both impl-a and impl-b are pending in tier 1
	manifest := &protocol.PROGRAMManifest{
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
		},
		Impls: []protocol.ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "pending"},
			{Slug: "impl-b", Tier: 1, Status: "pending"},
			{Slug: "impl-c", Tier: 2, Status: "pending", DependsOn: []string{"impl-a"}},
		},
	}

	result := ScoreTierIMPLs(manifest, 1)
	if len(result) != 2 {
		t.Fatalf("expected 2 slugs, got %d", len(result))
	}
	if result[0] != "impl-a" {
		t.Errorf("expected impl-a first (higher unblocking score), got %q", result[0])
	}
	if result[1] != "impl-b" {
		t.Errorf("expected impl-b second, got %q", result[1])
	}
}

// TestScoreTierIMPLs_MutatesManifestImplsPriorityScore verifies that ScoreTierIMPLs
// writes PriorityScore back into manifest.Impls in place.
func TestScoreTierIMPLs_MutatesManifestImplsPriorityScore(t *testing.T) {
	manifest := &protocol.PROGRAMManifest{
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"impl-a"}},
		},
		Impls: []protocol.ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "pending"},
			{Slug: "impl-b", Tier: 2, Status: "pending", DependsOn: []string{"impl-a"}},
		},
	}

	_ = ScoreTierIMPLs(manifest, 1)

	// impl-a should have PriorityScore > 0 because impl-b depends on it
	var implA *protocol.ProgramIMPL
	for i := range manifest.Impls {
		if manifest.Impls[i].Slug == "impl-a" {
			implA = &manifest.Impls[i]
			break
		}
	}
	if implA == nil {
		t.Fatal("impl-a not found in manifest.Impls")
	}
	if implA.PriorityScore <= 0 {
		t.Errorf("expected PriorityScore > 0, got %d", implA.PriorityScore)
	}
	if implA.PriorityReasoning == "" {
		t.Error("expected PriorityReasoning to be set")
	}
}

// TestScoreTierIMPLs_ExcludesNonPending verifies that non-pending IMPLs (e.g. "complete")
// are not included in the scored output.
func TestScoreTierIMPLs_ExcludesNonPending(t *testing.T) {
	manifest := &protocol.PROGRAMManifest{
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
		},
		Impls: []protocol.ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "complete"},
			{Slug: "impl-b", Tier: 1, Status: "pending"},
		},
	}

	result := ScoreTierIMPLs(manifest, 1)
	if len(result) != 1 {
		t.Fatalf("expected 1 slug (only pending), got %d: %v", len(result), result)
	}
	if result[0] != "impl-b" {
		t.Errorf("expected 'impl-b', got %q", result[0])
	}
}

// TestScoreTierIMPLs_TierNotFound verifies that ScoreTierIMPLs returns nil (not panic)
// when the requested tier does not exist.
func TestScoreTierIMPLs_TierNotFound(t *testing.T) {
	manifest := buildTestManifest(2)

	result := ScoreTierIMPLs(manifest, 99)
	if result != nil {
		t.Errorf("expected nil for missing tier, got %v", result)
	}
}

// TestAdvanceTierAutomatically_ScoredIMPLOrder_PopulatedOnAdvance verifies that
// ScoredIMPLOrder is populated in the result when AdvancedToNext=true.
func TestAdvanceTierAutomatically_ScoredIMPLOrder_PopulatedOnAdvance(t *testing.T) {
	repoPath := t.TempDir()

	// Build a 2-tier manifest where tier 2 has a pending IMPL
	manifest := &protocol.PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-program",
		State:       protocol.ProgramStateTierExecuting,
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"impl-tier1"}},
			{Number: 2, Impls: []string{"impl-tier2"}},
		},
		Impls: []protocol.ProgramIMPL{
			{Slug: "impl-tier1", Title: "IMPL Tier 1", Tier: 1, Status: "complete"},
			{Slug: "impl-tier2", Title: "IMPL Tier 2", Tier: 2, Status: "pending"},
		},
		Completion: protocol.ProgramCompletion{
			TiersTotal: 2,
			ImplsTotal: 2,
		},
	}

	result, err := AdvanceTierAutomatically(manifest, 1, repoPath, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.AdvancedToNext {
		t.Fatalf("expected AdvancedToNext=true")
	}
	if len(result.ScoredIMPLOrder) == 0 {
		t.Error("expected ScoredIMPLOrder to be populated when AdvancedToNext=true")
	}
	if result.ScoredIMPLOrder[0] != "impl-tier2" {
		t.Errorf("expected 'impl-tier2' in ScoredIMPLOrder, got %v", result.ScoredIMPLOrder)
	}
}

// TestAdvanceTierAutomatically_ScoredIMPLOrder_EmptyOnGateFail verifies that
// ScoredIMPLOrder is empty when the gate fails (RequiresReview=true).
func TestAdvanceTierAutomatically_ScoredIMPLOrder_EmptyOnGateFail(t *testing.T) {
	repoPath := t.TempDir()
	manifest := buildTestManifest(2)

	// Add a required gate that will fail
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
		t.Error("expected AdvancedToNext=false when gate fails")
	}
	if len(result.ScoredIMPLOrder) != 0 {
		t.Errorf("expected ScoredIMPLOrder to be empty when gate fails, got %v", result.ScoredIMPLOrder)
	}
}

// TestReplanProgram_ValidRevision tests that ReplanProgram successfully launches
// the Planner agent and returns a result with no error when given a valid manifest.
func TestReplanProgram_ValidRevision(t *testing.T) {
	t.Skip("Skipping test that requires live Claude API - causes rate limit failures in CI")

	// Write a minimal PROGRAM manifest file at <repo>/docs/PROGRAM/PROGRAM.yaml
	dir := t.TempDir()
	programDir := filepath.Join(dir, "docs", "PROGRAM")
	if err := os.MkdirAll(programDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(programDir, "PROGRAM.yaml")
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

	result, err := ReplanProgram(opts)
	if err != nil {
		t.Fatalf("ReplanProgram returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.RevisedManifestPath != manifestPath {
		t.Errorf("expected manifest path %q, got %q", manifestPath, result.RevisedManifestPath)
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
