package worktree

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestManagerNew verifies that New returns a non-nil Manager with slug.
func TestManagerNew(t *testing.T) {
	m := New("/some/repo/path", "my-feature")
	if m == nil {
		t.Fatal("New returned nil")
	}
	if m.repoPath != "/some/repo/path" {
		t.Errorf("repoPath = %q; want %q", m.repoPath, "/some/repo/path")
	}
	if m.slug != "my-feature" {
		t.Errorf("slug = %q; want %q", m.slug, "my-feature")
	}
	if m.active == nil {
		t.Fatal("active map is nil")
	}
	if len(m.active) != 0 {
		t.Errorf("active map should be empty; got len=%d", len(m.active))
	}
}

// setupGitRepo initialises a temporary git repo suitable for worktree commands.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	cmds := [][]string{
		{"git", "-C", repoDir, "init", "-b", "main"},
		{"git", "-C", repoDir, "config", "user.email", "test@test.com"},
		{"git", "-C", repoDir, "config", "user.name", "Test"},
		{"git", "-C", repoDir, "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("setup: %v: %s", err, out)
		}
	}
	return repoDir
}

// TestManagerCreateRemoveRoundtrip creates a worktree and removes it in a
// real git repo, verifying path construction and tracking map correctness.
func TestManagerCreateRemoveRoundtrip(t *testing.T) {
	repoDir := setupGitRepo(t)
	m := New(repoDir, "test-feature")

	wtPath, err := m.Create(1, "D")
	if err != nil {
		t.Fatalf("Create returned unexpected error: %v", err)
	}

	expectedPath := filepath.Join(repoDir, ".claude", "worktrees", "saw", "test-feature", "wave1-agent-D")
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
	r := m.Remove(wtPath)
	if r.IsFatal() {
		t.Fatalf("Remove returned fatal result: %v", r.Errors)
	}

	// Verify RemoveData has the correct path.
	data := r.GetData()
	if data.RemovedPath != wtPath {
		t.Errorf("RemoveData.RemovedPath = %q; want %q", data.RemovedPath, wtPath)
	}
	if !data.WasTracked {
		t.Error("RemoveData.WasTracked should be true")
	}

	// Manager should no longer track the worktree.
	list = m.List()
	if len(list) != 0 {
		t.Errorf("List after Remove: got %d entries; want 0", len(list))
	}
}

// TestRemoveUntrackedPath verifies Remove returns Fatal for untracked paths.
func TestRemoveUntrackedPath(t *testing.T) {
	m := New("/some/repo", "feat")
	r := m.Remove("/nonexistent/path")
	if !r.IsFatal() {
		t.Fatal("Remove of untracked path should return Fatal result")
	}
	if len(r.Errors) == 0 {
		t.Fatal("Remove of untracked path should have errors")
	}
	if r.Errors[0].Code != "WORKTREE_REMOVE_FAILED" {
		t.Errorf("error code = %q; want %q", r.Errors[0].Code, "WORKTREE_REMOVE_FAILED")
	}
}

// TestCleanupAllSuccess verifies CleanupAll returns Success and correct count
// when all worktrees are removed successfully.
func TestCleanupAllSuccess(t *testing.T) {
	repoDir := setupGitRepo(t)
	m := New(repoDir, "cleanup-test")

	// Create two worktrees.
	wt1, err := m.Create(1, "A")
	if err != nil {
		t.Fatalf("Create wt1: %v", err)
	}
	wt2, err := m.Create(1, "B")
	if err != nil {
		t.Fatalf("Create wt2: %v", err)
	}

	r := m.CleanupAll()
	if r.IsFatal() {
		t.Fatalf("CleanupAll returned fatal: %v", r.Errors)
	}

	data := r.GetData()
	if data.RemovedCount != 2 {
		t.Errorf("RemovedCount = %d; want 2", data.RemovedCount)
	}
	if len(data.RemovedPaths) != 2 {
		t.Errorf("RemovedPaths len = %d; want 2", len(data.RemovedPaths))
	}

	// Paths should include both created worktrees.
	found := map[string]bool{wt1: false, wt2: false}
	for _, p := range data.RemovedPaths {
		found[p] = true
	}
	for path, ok := range found {
		if !ok {
			t.Errorf("RemovedPaths missing %q", path)
		}
	}

	// Manager should now track nothing.
	if len(m.List()) != 0 {
		t.Errorf("List after CleanupAll: got %d entries; want 0", len(m.List()))
	}
}

// TestCleanupAllEmpty verifies CleanupAll on an empty manager returns Success
// with zero count.
func TestCleanupAllEmpty(t *testing.T) {
	m := New("/some/repo", "empty-test")
	r := m.CleanupAll()
	if r.IsFatal() {
		t.Fatalf("CleanupAll on empty manager returned fatal: %v", r.Errors)
	}
	data := r.GetData()
	if data.RemovedCount != 0 {
		t.Errorf("RemovedCount = %d; want 0", data.RemovedCount)
	}
}

// TestCleanupAllPartial verifies CleanupAll returns Partial when some
// worktrees succeed and some fail. We simulate a failure by manually injecting
// a path into the active map that has no real worktree backing it.
func TestCleanupAllPartial(t *testing.T) {
	repoDir := setupGitRepo(t)
	m := New(repoDir, "partial-test")

	// Create one real worktree that will succeed.
	wt1, err := m.Create(1, "A")
	if err != nil {
		t.Fatalf("Create wt1: %v", err)
	}

	// Inject a fake tracked path that will fail removal (no real worktree).
	fakePath := "/tmp/nonexistent-worktree-for-test"
	m.active[fakePath] = "saw/partial-test/wave1-agent-fake"

	r := m.CleanupAll()

	// Should be Partial: one succeeded, one failed.
	if !r.IsPartial() {
		t.Errorf("CleanupAll with mixed results should be Partial; got code=%q errors=%v", r.Code, r.Errors)
	}

	data := r.GetData()
	if data.RemovedCount != 1 {
		t.Errorf("RemovedCount = %d; want 1", data.RemovedCount)
	}
	if len(data.RemovedPaths) != 1 || data.RemovedPaths[0] != wt1 {
		t.Errorf("RemovedPaths = %v; want [%q]", data.RemovedPaths, wt1)
	}
	if len(r.Errors) == 0 {
		t.Error("Partial result should have warnings for failed removal")
	}
}
