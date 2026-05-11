package worktree

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// TestManagerNew verifies that New returns a non-nil Manager with slug.
func TestManagerNew(t *testing.T) {
	m, err := New("/some/repo/path", "my-feature")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
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

	// Empty repoPath returns error.
	_, err = New("", "my-feature")
	if err == nil {
		t.Error("New with empty repoPath should return error")
	}
	// Empty slug returns error.
	_, err = New("/some/repo", "")
	if err == nil {
		t.Error("New with empty slug should return error")
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
	m, err := New(repoDir, "test-feature")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := m.Create(1, "D")
	if r.IsFatal() {
		t.Fatalf("Create returned fatal result: %v", r.Errors)
	}

	wtPath := r.GetData().Path
	expectedPath := filepath.Join(repoDir, ".claude", "worktrees", "polywave", "test-feature", "wave1-agent-D")
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
	removeResult := m.Remove(wtPath)
	if removeResult.IsFatal() {
		t.Fatalf("Remove returned fatal result: %v", removeResult.Errors)
	}

	// Verify RemoveData has the correct path.
	data := removeResult.GetData()
	if data.RemovedPath != wtPath {
		t.Errorf("RemoveData.RemovedPath = %q; want %q", data.RemovedPath, wtPath)
	}

	// Manager should no longer track the worktree.
	list = m.List()
	if len(list) != 0 {
		t.Errorf("List after Remove: got %d entries; want 0", len(list))
	}
}

// TestRemoveUntrackedPath verifies Remove returns Fatal for untracked paths.
func TestRemoveUntrackedPath(t *testing.T) {
	m, err := New("/some/repo", "feat")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r := m.Remove("/nonexistent/path")
	if !r.IsFatal() {
		t.Fatal("Remove of untracked path should return Fatal result")
	}
	if len(r.Errors) == 0 {
		t.Fatal("Remove of untracked path should have errors")
	}
	if r.Errors[0].Code != "G008_WORKTREE_REMOVE_FAILED" {
		t.Errorf("error code = %q; want %q", r.Errors[0].Code, "G008_WORKTREE_REMOVE_FAILED")
	}
}

// TestCleanupAllSuccess verifies CleanupAll returns Success and correct count
// when all worktrees are removed successfully.
func TestCleanupAllSuccess(t *testing.T) {
	repoDir := setupGitRepo(t)
	m, err := New(repoDir, "cleanup-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create two worktrees.
	r1 := m.Create(1, "A")
	if r1.IsFatal() {
		t.Fatalf("Create wt1: %v", r1.Errors)
	}
	wt1 := r1.GetData().Path
	r2 := m.Create(1, "B")
	if r2.IsFatal() {
		t.Fatalf("Create wt2: %v", r2.Errors)
	}
	wt2 := r2.GetData().Path

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
	m, err := New("/some/repo", "empty-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
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
	m, err := New(repoDir, "partial-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create one real worktree that will succeed.
	r1 := m.Create(1, "A")
	if r1.IsFatal() {
		t.Fatalf("Create wt1: %v", r1.Errors)
	}
	wt1 := r1.GetData().Path

	// Inject a fake tracked path that will fail removal (no real worktree).
	fakePath := "/tmp/nonexistent-worktree-for-test"
	m.active[fakePath] = "polywave/partial-test/wave1-agent-fake"

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

// TestCreateAlreadyExists verifies Create returns Fatal when attempting to
// create a worktree that already exists.
func TestCreateAlreadyExists(t *testing.T) {
	repoDir := setupGitRepo(t)
	m, err := New(repoDir, "duplicate-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a worktree successfully.
	r1 := m.Create(1, "C")
	if r1.IsFatal() {
		t.Fatalf("First Create failed: %v", r1.Errors)
	}

	// Attempt to create the same wave/agent worktree again.
	r2 := m.Create(1, "C")
	if !r2.IsFatal() {
		t.Fatal("Create of already-existing worktree should return Fatal")
	}
	if len(r2.Errors) == 0 {
		t.Fatal("Create should have errors")
	}
	if r2.Errors[0].Code != result.CodeWorktreeCreateFailed {
		t.Errorf("error code = %q; want %q", r2.Errors[0].Code, result.CodeWorktreeCreateFailed)
	}
}

// TestCreateInvalidRepoRoot verifies Create returns Fatal when the repo path
// is not a valid git repository.
func TestCreateInvalidRepoRoot(t *testing.T) {
	invalidRepo := t.TempDir()
	m, err := New(invalidRepo, "invalid-repo-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := m.Create(1, "X")
	if !r.IsFatal() {
		t.Fatal("Create with invalid repo root should return Fatal")
	}
	if len(r.Errors) == 0 {
		t.Fatal("Create should have errors")
	}
	if r.Errors[0].Code != result.CodeWorktreeCreateFailed {
		t.Errorf("error code = %q; want %q", r.Errors[0].Code, result.CodeWorktreeCreateFailed)
	}
}

// TestSetLogger verifies SetLogger correctly configures the manager's logger.
func TestSetLogger(t *testing.T) {
	m, err := New("/some/repo", "logger-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Default logger should be slog.Default().
	if m.log() != slog.Default() {
		t.Error("log() should return slog.Default() before SetLogger is called")
	}

	// Set a custom logger.
	customLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	m.SetLogger(customLogger)

	// log() should now return the custom logger.
	if m.log() != customLogger {
		t.Error("log() should return custom logger after SetLogger")
	}
}

// TestListEmpty verifies List() returns an empty slice (not nil) on a fresh manager.
func TestListEmpty(t *testing.T) {
	m, err := New("/some/repo", "empty-list-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	list := m.List()
	if list == nil {
		t.Fatal("List() should return empty slice, not nil")
	}
	if len(list) != 0 {
		t.Errorf("List() on fresh manager: got len=%d; want 0", len(list))
	}
}

// TestCreateBranchAlreadyExists verifies Create returns CodeBranchExists when the
// branch already exists but the worktree directory has been removed externally.
func TestCreateBranchAlreadyExists(t *testing.T) {
	repoDir := setupGitRepo(t)
	m, err := New(repoDir, "branch-exists-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Create worktree once successfully.
	r1 := m.Create(1, "Z")
	if r1.IsFatal() {
		t.Fatalf("First Create failed: %v", r1.Errors)
	}
	wtPath := r1.GetData().Path

	// Remove the worktree directory via git (force) but keep the branch.
	if out, err := exec.Command("git", "-C", repoDir, "worktree", "remove", "--force", wtPath).CombinedOutput(); err != nil {
		t.Fatalf("git worktree remove: %v: %s", err, out)
	}
	// Also remove from the Manager's tracking so os.Stat check passes.
	delete(m.active, wtPath)

	// Branch still exists — Create should return CodeBranchExists.
	r2 := m.Create(1, "Z")
	if !r2.IsFatal() {
		t.Fatal("Create with existing branch should return Fatal")
	}
	if len(r2.Errors) == 0 {
		t.Fatal("Create should have errors")
	}
	if r2.Errors[0].Code != result.CodeBranchExists {
		t.Errorf("error code = %q; want %q", r2.Errors[0].Code, result.CodeBranchExists)
	}
}

// TestCleanupAllErrorPropagation verifies that CleanupAll propagates the original
// error code from Remove rather than emitting a generic cleanup code.
func TestCleanupAllErrorPropagation(t *testing.T) {
	m, err := New("/some/repo", "err-prop-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Inject a fake tracked path that has no real worktree (Remove will fail).
	fakePath := "/tmp/nonexistent-for-error-prop-test"
	m.active[fakePath] = "polywave/err-prop-test/wave1-agent-fake"

	r := m.CleanupAll()
	// All removals failed — expect Fatal.
	if !r.IsFatal() {
		t.Fatalf("CleanupAll should be Fatal when all removals fail; got code=%q", r.Code)
	}
	if len(r.Errors) == 0 {
		t.Fatal("CleanupAll should have errors")
	}
	// Error code must be from Remove (G008), not the old generic cleanup code (G007).
	for _, e := range r.Errors {
		if e.Code != result.CodeWorktreeRemoveFailed {
			t.Errorf("error code = %q; want %q (original Remove error must be propagated)",
				e.Code, result.CodeWorktreeRemoveFailed)
		}
	}
}
