package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestGetProgramStatus_AllPending verifies status computation for a fresh program
// where nothing has started yet.
func TestGetProgramStatus_AllPending(t *testing.T) {
	manifest := &PROGRAMManifest{
		ProgramSlug: "test-program",
		Title:       "Test Program",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "pending"},
			{Slug: "impl-b", Tier: 1, Status: "pending"},
			{Slug: "impl-c", Tier: 2, Status: "pending"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
			{Number: 2, Impls: []string{"impl-c"}},
		},
		Completion: ProgramCompletion{
			TiersTotal: 2,
			ImplsTotal: 3,
		},
	}

	res := GetProgramStatus(manifest, ".")
	if res.IsFatal() {
		t.Fatalf("GetProgramStatus failed: %+v", res.Errors)
	}
	status := res.GetData()

	if status.ProgramSlug != "test-program" {
		t.Errorf("expected program_slug=test-program, got %s", status.ProgramSlug)
	}

	if status.CurrentTier != 1 {
		t.Errorf("expected current_tier=1, got %d", status.CurrentTier)
	}

	if status.Completion.TiersComplete != 0 {
		t.Errorf("expected tiers_complete=0, got %d", status.Completion.TiersComplete)
	}

	if status.Completion.ImplsComplete != 0 {
		t.Errorf("expected impls_complete=0, got %d", status.Completion.ImplsComplete)
	}

	if len(status.TierStatuses) != 2 {
		t.Fatalf("expected 2 tier statuses, got %d", len(status.TierStatuses))
	}

	tier1 := status.TierStatuses[0]
	if tier1.Complete {
		t.Errorf("tier 1 should not be complete")
	}
	if len(tier1.ImplStatuses) != 2 {
		t.Errorf("expected 2 impls in tier 1, got %d", len(tier1.ImplStatuses))
	}

	tier2 := status.TierStatuses[1]
	if tier2.Complete {
		t.Errorf("tier 2 should not be complete")
	}
}

// TestGetProgramStatus_Tier1InProgress verifies status when some tier 1 IMPLs are executing.
func TestGetProgramStatus_Tier1InProgress(t *testing.T) {
	manifest := &PROGRAMManifest{
		ProgramSlug: "test-program",
		Title:       "Test Program",
		State:       ProgramStateTierExecuting,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "complete"},
			{Slug: "impl-b", Tier: 1, Status: "in-progress"},
			{Slug: "impl-c", Tier: 2, Status: "pending"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
			{Number: 2, Impls: []string{"impl-c"}},
		},
		Completion: ProgramCompletion{
			TiersTotal: 2,
			ImplsTotal: 3,
		},
	}

	res := GetProgramStatus(manifest, ".")
	if res.IsFatal() {
		t.Fatalf("GetProgramStatus failed: %+v", res.Errors)
	}
	status := res.GetData()

	if status.CurrentTier != 1 {
		t.Errorf("expected current_tier=1, got %d", status.CurrentTier)
	}

	if status.Completion.ImplsComplete != 1 {
		t.Errorf("expected impls_complete=1, got %d", status.Completion.ImplsComplete)
	}

	tier1 := status.TierStatuses[0]
	if tier1.Complete {
		t.Errorf("tier 1 should not be complete (impl-b still in progress)")
	}
}

// TestGetProgramStatus_Tier1Complete verifies status when tier 1 is complete but tier 2 is pending.
func TestGetProgramStatus_Tier1Complete(t *testing.T) {
	manifest := &PROGRAMManifest{
		ProgramSlug: "test-program",
		Title:       "Test Program",
		State:       ProgramStateTierVerified,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "complete"},
			{Slug: "impl-b", Tier: 1, Status: "complete"},
			{Slug: "impl-c", Tier: 2, Status: "pending"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
			{Number: 2, Impls: []string{"impl-c"}},
		},
		Completion: ProgramCompletion{
			TiersTotal: 2,
			ImplsTotal: 3,
		},
	}

	res := GetProgramStatus(manifest, ".")
	if res.IsFatal() {
		t.Fatalf("GetProgramStatus failed: %+v", res.Errors)
	}
	status := res.GetData()

	if status.CurrentTier != 2 {
		t.Errorf("expected current_tier=2, got %d", status.CurrentTier)
	}

	if status.Completion.TiersComplete != 1 {
		t.Errorf("expected tiers_complete=1, got %d", status.Completion.TiersComplete)
	}

	if status.Completion.ImplsComplete != 2 {
		t.Errorf("expected impls_complete=2, got %d", status.Completion.ImplsComplete)
	}

	tier1 := status.TierStatuses[0]
	if !tier1.Complete {
		t.Errorf("tier 1 should be complete")
	}

	tier2 := status.TierStatuses[1]
	if tier2.Complete {
		t.Errorf("tier 2 should not be complete yet")
	}
}

// TestGetProgramStatus_AllComplete verifies status when the entire program is done.
func TestGetProgramStatus_AllComplete(t *testing.T) {
	manifest := &PROGRAMManifest{
		ProgramSlug: "test-program",
		Title:       "Test Program",
		State:       ProgramStateComplete,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "complete"},
			{Slug: "impl-b", Tier: 1, Status: "complete"},
			{Slug: "impl-c", Tier: 2, Status: "complete"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
			{Number: 2, Impls: []string{"impl-c"}},
		},
		Completion: ProgramCompletion{
			TiersTotal: 2,
			ImplsTotal: 3,
		},
	}

	res := GetProgramStatus(manifest, ".")
	if res.IsFatal() {
		t.Fatalf("GetProgramStatus failed: %+v", res.Errors)
	}
	status := res.GetData()

	if status.CurrentTier != 2 {
		t.Errorf("expected current_tier=2 (all complete), got %d", status.CurrentTier)
	}

	if status.Completion.TiersComplete != 2 {
		t.Errorf("expected tiers_complete=2, got %d", status.Completion.TiersComplete)
	}

	if status.Completion.ImplsComplete != 3 {
		t.Errorf("expected impls_complete=3, got %d", status.Completion.ImplsComplete)
	}

	for i, tier := range status.TierStatuses {
		if !tier.Complete {
			t.Errorf("tier %d should be complete", i+1)
		}
	}
}

// TestGetProgramStatus_ContractFreezing verifies correct freeze detection.
func TestGetProgramStatus_ContractFreezing(t *testing.T) {
	manifest := &PROGRAMManifest{
		ProgramSlug: "test-program",
		Title:       "Test Program",
		State:       ProgramStateTierVerified,
		ProgramContracts: []ProgramContract{
			{
				Name:     "SharedInterface",
				Location: "pkg/shared/interface.go",
				FreezeAt: "impl-a", // Freezes when impl-a is complete (tier 1)
			},
			{
				Name:     "DeferredContract",
				Location: "pkg/api/contract.go",
				FreezeAt: "impl-c", // Freezes when impl-c is complete (tier 2)
			},
		},
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "complete"},
			{Slug: "impl-b", Tier: 1, Status: "complete"},
			{Slug: "impl-c", Tier: 2, Status: "pending"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
			{Number: 2, Impls: []string{"impl-c"}},
		},
		Completion: ProgramCompletion{
			TiersTotal: 2,
			ImplsTotal: 3,
		},
	}

	res := GetProgramStatus(manifest, ".")
	if res.IsFatal() {
		t.Fatalf("GetProgramStatus failed: %+v", res.Errors)
	}
	status := res.GetData()

	if len(status.ContractStatuses) != 2 {
		t.Fatalf("expected 2 contract statuses, got %d", len(status.ContractStatuses))
	}

	// First contract should be frozen (tier 1 complete)
	contract1 := status.ContractStatuses[0]
	if contract1.Name != "SharedInterface" {
		t.Errorf("expected contract name=SharedInterface, got %s", contract1.Name)
	}
	if !contract1.Frozen {
		t.Errorf("SharedInterface should be frozen (tier 1 complete)")
	}
	if contract1.FrozenAtTier != 1 {
		t.Errorf("expected frozen_at_tier=1, got %d", contract1.FrozenAtTier)
	}

	// Second contract should NOT be frozen (tier 2 incomplete)
	contract2 := status.ContractStatuses[1]
	if contract2.Name != "DeferredContract" {
		t.Errorf("expected contract name=DeferredContract, got %s", contract2.Name)
	}
	if contract2.Frozen {
		t.Errorf("DeferredContract should not be frozen yet (tier 2 incomplete)")
	}
}

// TestGetProgramStatus_CompletionCounts verifies that computed counts override manifest values.
func TestGetProgramStatus_CompletionCounts(t *testing.T) {
	manifest := &PROGRAMManifest{
		ProgramSlug: "test-program",
		Title:       "Test Program",
		State:       ProgramStateTierExecuting,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "complete"},
			{Slug: "impl-b", Tier: 1, Status: "complete"},
			{Slug: "impl-c", Tier: 2, Status: "in-progress"},
			{Slug: "impl-d", Tier: 2, Status: "pending"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
			{Number: 2, Impls: []string{"impl-c", "impl-d"}},
		},
		Completion: ProgramCompletion{
			TiersComplete: 0, // Stale value from manifest
			ImplsComplete: 0, // Stale value from manifest
			TiersTotal:    2,
			ImplsTotal:    4,
		},
	}

	res := GetProgramStatus(manifest, ".")
	if res.IsFatal() {
		t.Fatalf("GetProgramStatus failed: %+v", res.Errors)
	}
	status := res.GetData()

	// Should compute correct counts from actual statuses
	if status.Completion.TiersComplete != 1 {
		t.Errorf("expected tiers_complete=1 (computed), got %d", status.Completion.TiersComplete)
	}

	if status.Completion.ImplsComplete != 2 {
		t.Errorf("expected impls_complete=2 (computed), got %d", status.Completion.ImplsComplete)
	}

	// Should preserve totals from manifest
	if status.Completion.TiersTotal != 2 {
		t.Errorf("expected tiers_total=2, got %d", status.Completion.TiersTotal)
	}

	if status.Completion.ImplsTotal != 4 {
		t.Errorf("expected impls_total=4, got %d", status.Completion.ImplsTotal)
	}
}

// TestGetProgramStatus_NilManifest verifies error handling for nil manifest.
func TestGetProgramStatus_NilManifest(t *testing.T) {
	res := GetProgramStatus(nil, ".")
	if !res.IsFatal() {
		t.Fatal("expected fatal result for nil manifest")
	}
}

// TestGetProgramStatus_MissingIMPLInTier verifies graceful handling when tier references
// an IMPL that doesn't exist in the manifest.
func TestGetProgramStatus_MissingIMPLInTier(t *testing.T) {
	manifest := &PROGRAMManifest{
		ProgramSlug: "test-program",
		Title:       "Test Program",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "pending"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-missing"}}, // Missing IMPL
		},
		Completion: ProgramCompletion{
			TiersTotal: 1,
			ImplsTotal: 1,
		},
	}

	res := GetProgramStatus(manifest, ".")
	if res.IsFatal() {
		t.Fatalf("GetProgramStatus should handle missing IMPL gracefully, got error: %+v", res.Errors)
	}
	status := res.GetData()

	tier1 := status.TierStatuses[0]
	if len(tier1.ImplStatuses) != 2 {
		t.Errorf("expected 2 impl statuses (including missing), got %d", len(tier1.ImplStatuses))
	}

	// Missing IMPL should default to "pending"
	foundMissing := false
	for _, implStatus := range tier1.ImplStatuses {
		if implStatus.Slug == "impl-missing" {
			foundMissing = true
			if implStatus.Status != "pending" {
				t.Errorf("expected missing IMPL to default to pending, got %s", implStatus.Status)
			}
		}
	}
	if !foundMissing {
		t.Errorf("missing IMPL should appear in tier statuses")
	}
}

// TestGetProgramStatus_ReadFromDisk verifies that IMPL statuses are enriched from disk.
func TestGetProgramStatus_ReadFromDisk(t *testing.T) {
	// Create a temporary directory structure with IMPL docs
	tmpDir := t.TempDir()
	implDocsDir := filepath.Join(tmpDir, "docs", "IMPL", "complete")
	if err := os.MkdirAll(implDocsDir, 0755); err != nil {
		t.Fatalf("failed to create temp IMPL docs dir: %v", err)
	}

	// Write a complete IMPL doc
	implDoc := map[string]interface{}{
		"title": "Test IMPL A",
		"state": "COMPLETE",
	}
	data, _ := yaml.Marshal(implDoc)
	implPath := filepath.Join(implDocsDir, "IMPL-impl-a.yaml")
	if err := os.WriteFile(implPath, data, 0644); err != nil {
		t.Fatalf("failed to write IMPL doc: %v", err)
	}

	manifest := &PROGRAMManifest{
		ProgramSlug: "test-program",
		Title:       "Test Program",
		State:       ProgramStateTierExecuting,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "pending"}, // Stale status in manifest
			{Slug: "impl-b", Tier: 1, Status: "pending"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
		},
		Completion: ProgramCompletion{
			TiersTotal: 1,
			ImplsTotal: 2,
		},
	}

	res := GetProgramStatus(manifest, tmpDir)
	if res.IsFatal() {
		t.Fatalf("GetProgramStatus failed: %+v", res.Errors)
	}
	status := res.GetData()

	// impl-a should be updated to "complete" from disk
	tier1 := status.TierStatuses[0]
	var implAStatus string
	for _, implStatus := range tier1.ImplStatuses {
		if implStatus.Slug == "impl-a" {
			implAStatus = implStatus.Status
			break
		}
	}

	if implAStatus != "complete" {
		t.Errorf("expected impl-a status to be enriched to 'complete' from disk, got %s", implAStatus)
	}

	if status.Completion.ImplsComplete != 1 {
		t.Errorf("expected impls_complete=1 (from disk), got %d", status.Completion.ImplsComplete)
	}
}

// TestGetProgramStatus_ContractNoFreezeAt verifies handling of contracts without FreezeAt.
func TestGetProgramStatus_ContractNoFreezeAt(t *testing.T) {
	manifest := &PROGRAMManifest{
		ProgramSlug: "test-program",
		Title:       "Test Program",
		State:       ProgramStatePlanning,
		ProgramContracts: []ProgramContract{
			{
				Name:     "NeverFreeze",
				Location: "pkg/core/interface.go",
				// No FreezeAt specified
			},
		},
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "complete"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a"}},
		},
		Completion: ProgramCompletion{
			TiersTotal: 1,
			ImplsTotal: 1,
		},
	}

	res := GetProgramStatus(manifest, ".")
	if res.IsFatal() {
		t.Fatalf("GetProgramStatus failed: %+v", res.Errors)
	}
	status := res.GetData()

	if len(status.ContractStatuses) != 1 {
		t.Fatalf("expected 1 contract status, got %d", len(status.ContractStatuses))
	}

	contract := status.ContractStatuses[0]
	if contract.Frozen {
		t.Errorf("contract without FreezeAt should never be frozen")
	}
}
