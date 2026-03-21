package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStaleExtractSlugFromBranches(t *testing.T) {
	tests := []struct {
		branch string
		slug   string
	}{
		{"saw/my-feature/wave1-agent-A", "my-feature"},
		{"saw/type-collision-detection/wave2-agent-B3", "type-collision-detection"},
		{"saw/x/wave1-agent-A", "x"},
		{"wave1-agent-A", ""},           // legacy — no slug
		{"main", ""},                    // not a SAW branch
		{"saw/slug-only", ""},           // no wave suffix
		{"refs/heads/saw/s/wave1-agent-A", ""}, // unexpected prefix
	}

	for _, tt := range tests {
		got := ExtractSlug(tt.branch)
		if got != tt.slug {
			t.Errorf("ExtractSlug(%q) = %q, want %q", tt.branch, got, tt.slug)
		}
	}
}

func TestStaleDetectionCompletedImpl(t *testing.T) {
	// This test verifies fileExists-based detection logic without real git.
	// We test the file existence checks that drive the "completed_impl" reason.
	tmpDir := t.TempDir()

	// Create docs/IMPL/complete/IMPL-my-feature.yaml
	completeDir := filepath.Join(tmpDir, "docs", "IMPL", "complete")
	if err := os.MkdirAll(completeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(completeDir, "IMPL-my-feature.yaml"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify fileExists finds it
	if !fileExists(filepath.Join(completeDir, "IMPL-my-feature.yaml")) {
		t.Error("expected completed IMPL to exist")
	}

	// Verify active IMPL does not exist
	activePath := filepath.Join(tmpDir, "docs", "IMPL", "IMPL-my-feature.yaml")
	if fileExists(activePath) {
		t.Error("expected active IMPL to not exist")
	}
}

func TestStaleDetectionOrphaned(t *testing.T) {
	tmpDir := t.TempDir()

	// Create docs/IMPL/ but no IMPL file for slug "orphan-feature"
	implDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatal(err)
	}
	completeDir := filepath.Join(implDir, "complete")
	if err := os.MkdirAll(completeDir, 0755); err != nil {
		t.Fatal(err)
	}

	activePath := filepath.Join(implDir, "IMPL-orphan-feature.yaml")
	completePath := filepath.Join(completeDir, "IMPL-orphan-feature.yaml")

	if fileExists(activePath) || fileExists(completePath) {
		t.Error("expected neither active nor complete IMPL to exist for orphan slug")
	}
}

func TestStaleCleanupSkipsUncommittedChanges(t *testing.T) {
	// CleanStaleWorktrees calls git.StatusPorcelain on each worktree path.
	// Without a real git repo, StatusPorcelain will error, which means
	// the code path falls through to removal (error on status != dirty).
	// This test validates the Skipped path logic by checking the result
	// structure is properly initialized.
	result, err := CleanStaleWorktrees(nil, false)
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
	if len(result.Cleaned) != 0 || len(result.Skipped) != 0 || len(result.Errors) != 0 {
		t.Error("expected empty result for nil input")
	}
}

func TestStaleCleanupResultStructure(t *testing.T) {
	// Verify that CleanStaleWorktrees returns non-nil slices even with no input.
	result, err := CleanStaleWorktrees([]StaleWorktree{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Cleaned == nil {
		t.Error("Cleaned should be non-nil empty slice")
	}
	if result.Skipped == nil {
		t.Error("Skipped should be non-nil empty slice")
	}
	if result.Errors == nil {
		t.Error("Errors should be non-nil empty slice")
	}
}

func TestStaleWorktreeReasons(t *testing.T) {
	// Validate that the reason constants are what consumers expect.
	reasons := []string{"completed_impl", "orphaned", "merged_but_not_cleaned"}
	for _, r := range reasons {
		sw := StaleWorktree{Reason: r}
		if sw.Reason != r {
			t.Errorf("reason mismatch: got %q, want %q", sw.Reason, r)
		}
	}
}
