package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"gopkg.in/yaml.v3"
)

func TestProgramProgress_UpdateStatus(t *testing.T) {
	manifest := &protocol.PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-prog",
		Impls: []protocol.ProgramIMPL{
			{Slug: "feat-a", Title: "Feature A", Tier: 1, Status: "pending"},
			{Slug: "feat-b", Title: "Feature B", Tier: 1, Status: "pending"},
			{Slug: "feat-c", Title: "Feature C", Tier: 2, Status: "pending"},
		},
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"feat-a", "feat-b"}},
			{Number: 2, Impls: []string{"feat-c"}},
		},
		Completion: protocol.ProgramCompletion{
			TiersTotal: 2,
			ImplsTotal: 3,
		},
	}
	manifestPath := createTestManifest(t, manifest)

	// Update feat-a to complete
	if err := UpdateProgramIMPLStatus(manifestPath, "feat-a", "complete"); err != nil {
		t.Fatalf("UpdateProgramIMPLStatus failed: %v", err)
	}

	// Re-read and verify
	updated, err := protocol.ParseProgramManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to re-read manifest: %v", err)
	}

	// Check status was updated
	for _, impl := range updated.Impls {
		if impl.Slug == "feat-a" && impl.Status != "complete" {
			t.Errorf("expected feat-a status=complete, got %q", impl.Status)
		}
	}

	// Check counters: 1 IMPL complete, 0 tiers complete (tier 1 has feat-b still pending)
	if updated.Completion.ImplsComplete != 1 {
		t.Errorf("expected ImplsComplete=1, got %d", updated.Completion.ImplsComplete)
	}
	if updated.Completion.TiersComplete != 0 {
		t.Errorf("expected TiersComplete=0, got %d", updated.Completion.TiersComplete)
	}

	// Now complete feat-b to finish tier 1
	if err := UpdateProgramIMPLStatus(manifestPath, "feat-b", "complete"); err != nil {
		t.Fatalf("UpdateProgramIMPLStatus failed: %v", err)
	}

	updated, err = protocol.ParseProgramManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to re-read manifest: %v", err)
	}

	if updated.Completion.ImplsComplete != 2 {
		t.Errorf("expected ImplsComplete=2, got %d", updated.Completion.ImplsComplete)
	}
	if updated.Completion.TiersComplete != 1 {
		t.Errorf("expected TiersComplete=1, got %d", updated.Completion.TiersComplete)
	}
}

func TestProgramProgress_UpdateStatus_NotFound(t *testing.T) {
	manifest := &protocol.PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-prog",
		Impls: []protocol.ProgramIMPL{
			{Slug: "feat-a", Title: "Feature A", Tier: 1, Status: "pending"},
		},
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"feat-a"}},
		},
		Completion: protocol.ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
	}
	manifestPath := createTestManifest(t, manifest)

	err := UpdateProgramIMPLStatus(manifestPath, "nonexistent", "complete")
	if err == nil {
		t.Fatal("expected error for nonexistent slug, got nil")
	}
}

func TestProgramProgress_SyncFromDisk(t *testing.T) {
	// Create a repo structure with IMPL docs on disk
	repoDir := t.TempDir()
	implDir := filepath.Join(repoDir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatalf("failed to create IMPL dir: %v", err)
	}

	// Write an IMPL doc that shows COMPLETE state
	implDoc := map[string]interface{}{
		"slug":  "feat-a",
		"state": "COMPLETE",
	}
	data, _ := yaml.Marshal(implDoc)
	if err := os.WriteFile(filepath.Join(implDir, "IMPL-feat-a.yaml"), data, 0644); err != nil {
		t.Fatalf("failed to write IMPL doc: %v", err)
	}

	// Write another IMPL doc that shows in-progress state
	implDoc2 := map[string]interface{}{
		"slug":  "feat-b",
		"state": "WAVE_EXECUTING",
	}
	data2, _ := yaml.Marshal(implDoc2)
	if err := os.WriteFile(filepath.Join(implDir, "IMPL-feat-b.yaml"), data2, 0644); err != nil {
		t.Fatalf("failed to write IMPL doc: %v", err)
	}

	// Create manifest with both IMPLs as pending
	manifest := &protocol.PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-prog",
		Impls: []protocol.ProgramIMPL{
			{Slug: "feat-a", Title: "Feature A", Tier: 1, Status: "pending"},
			{Slug: "feat-b", Title: "Feature B", Tier: 1, Status: "pending"},
		},
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"feat-a", "feat-b"}},
		},
		Completion: protocol.ProgramCompletion{
			TiersTotal: 1,
			ImplsTotal: 2,
		},
	}
	manifestPath := createTestManifest(t, manifest)

	// Sync from disk
	if err := SyncProgramStatusFromDisk(manifestPath, repoDir); err != nil {
		t.Fatalf("SyncProgramStatusFromDisk failed: %v", err)
	}

	// Re-read and verify
	updated, err := protocol.ParseProgramManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to re-read manifest: %v", err)
	}

	statusMap := make(map[string]string)
	for _, impl := range updated.Impls {
		statusMap[impl.Slug] = impl.Status
	}

	if statusMap["feat-a"] != "complete" {
		t.Errorf("expected feat-a status=complete, got %q", statusMap["feat-a"])
	}
	if statusMap["feat-b"] != "in-progress" {
		t.Errorf("expected feat-b status=in-progress, got %q", statusMap["feat-b"])
	}

	// Counters: 1 complete, 0 tiers complete (feat-b not complete)
	if updated.Completion.ImplsComplete != 1 {
		t.Errorf("expected ImplsComplete=1, got %d", updated.Completion.ImplsComplete)
	}
	if updated.Completion.TiersComplete != 0 {
		t.Errorf("expected TiersComplete=0, got %d", updated.Completion.TiersComplete)
	}
}
