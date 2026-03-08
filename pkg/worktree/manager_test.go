package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerNew verifies that New returns a non-nil Manager.
func TestManagerNew(t *testing.T) {
	m := New("/some/repo/path")
	if m == nil {
		t.Fatal("New returned nil")
	}
	if m.repoPath != "/some/repo/path" {
		t.Errorf("repoPath = %q; want %q", m.repoPath, "/some/repo/path")
	}
	if m.active == nil {
		t.Fatal("active map is nil")
	}
	if len(m.active) != 0 {
		t.Errorf("active map should be empty; got len=%d", len(m.active))
	}
}

// TestManagerCreateRemoveRoundtrip creates a worktree entry and removes it,
// verifying that the Manager's tracking map is updated correctly.
// Because internal/git is a stub (no-op) in this worktree, we exercise the
// Manager's path/branch logic and tracking bookkeeping end-to-end.
func TestManagerCreateRemoveRoundtrip(t *testing.T) {
	repoDir := t.TempDir()

	// Initialize a bare-enough git repo so MkdirAll in Create doesn't fail.
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("setup git dir: %v", err)
	}

	m := New(repoDir)

	// Create — git.WorktreeAdd is a stub so it succeeds without touching disk.
	wtPath, err := m.Create(1, "D")
	if err != nil {
		t.Fatalf("Create returned unexpected error: %v", err)
	}

	expectedPath := filepath.Join(repoDir, ".claude", "worktrees", "wave1-agent-D")
	if wtPath != expectedPath {
		t.Errorf("Create path = %q; want %q", wtPath, expectedPath)
	}

	// Manager should track the new worktree.
	list := m.List()
	if len(list) != 1 {
		t.Fatalf("List after Create: got %d entries; want 1", len(list))
	}
	if list[0] != expectedPath {
		t.Errorf("List[0] = %q; want %q", list[0], expectedPath)
	}

	// Remove — git.WorktreeRemove and git.DeleteBranch are stubs; both succeed.
	if err := m.Remove(wtPath); err != nil {
		t.Fatalf("Remove returned unexpected error: %v", err)
	}

	// Manager should no longer track the worktree.
	list = m.List()
	if len(list) != 0 {
		t.Errorf("List after Remove: got %d entries; want 0", len(list))
	}

	// Remove of unknown path should return an error.
	err = m.Remove("/nonexistent/path")
	if err == nil {
		t.Error("Remove of untracked path should return an error, got nil")
	}
	expectedErrStr := fmt.Sprintf("manager: worktree %q is not tracked", "/nonexistent/path")
	if err.Error() != expectedErrStr {
		t.Errorf("error = %q; want %q", err.Error(), expectedErrStr)
	}
}
