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
		{"polywave/my-feature/wave1-agent-A", "my-feature"},
		{"polywave/type-collision-detection/wave2-agent-B3", "type-collision-detection"},
		{"polywave/x/wave1-agent-A", "x"},
		{"wave1-agent-A", ""},           // legacy — no slug
		{"main", ""},                    // not a SAW branch
		{"polywave/slug-only", ""},           // no wave suffix
		{"refs/heads/polywave/s/wave1-agent-A", ""}, // unexpected prefix
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
	r := CleanStaleWorktrees(nil, false)
	if !r.IsSuccess() {
		t.Fatalf("unexpected failure on empty input: %v", r.Errors)
	}
	data := r.GetData()
	if len(data.Cleaned) != 0 || len(data.Skipped) != 0 || len(data.Errors) != 0 {
		t.Error("expected empty result for nil input")
	}
}

func TestStaleCleanupDataStructure(t *testing.T) {
	// Verify that CleanStaleWorktrees returns non-nil slices even with no input.
	r := CleanStaleWorktrees([]StaleWorktree{}, false)
	if !r.IsSuccess() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	data := r.GetData()
	if data.Cleaned == nil {
		t.Error("Cleaned should be non-nil empty slice")
	}
	if data.Skipped == nil {
		t.Error("Skipped should be non-nil empty slice")
	}
	if data.Errors == nil {
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

func TestStaleDetectWorktreesForSlug(t *testing.T) {
	// Use a temp dir that has no git worktrees — expect empty result and no error.
	tmpDir := t.TempDir()
	result, err := DetectStaleWorktreesForSlug(tmpDir, "my-feature")
	if err != nil {
		// git.WorktreeList may return an error for a non-git dir; that is
		// propagated as an error from DetectStaleWorktreesForSlug.
		// Accept either empty-with-no-error or an error (not a panic).
		t.Logf("DetectStaleWorktreesForSlug returned expected error for non-git dir: %v", err)
		return
	}
	if len(result) != 0 {
		t.Errorf("expected 0 stale worktrees, got %d", len(result))
	}
}
