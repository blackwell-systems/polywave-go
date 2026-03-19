package resume

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// makeTestRepo initialises a bare git repo at dir and returns its path.
// It creates an initial commit so the repo is valid.
func makeTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	// Create initial commit.
	f := filepath.Join(dir, "README")
	if err := os.WriteFile(f, []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")

	return dir
}

// addWorktree creates a git worktree at wtPath with the given branch name.
func addWorktree(t *testing.T, repoDir, wtPath, branch string) {
	t.Helper()
	cmd := exec.Command("git", "-C", repoDir, "worktree", "add", "-b", branch, wtPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add %s: %v\n%s", branch, err, out)
	}
}

// makeManifest returns a minimal IMPLManifest with the given feature slug.
func makeManifest(slug string) *protocol.IMPLManifest {
	return &protocol.IMPLManifest{
		FeatureSlug: slug,
	}
}

// TestClassifyWorktrees_DirtyWorktree verifies that a worktree with an
// uncommitted file is reported as HasChanges=true.
func TestClassifyWorktrees_DirtyWorktree(t *testing.T) {
	repo := makeTestRepo(t)
	wtDir := t.TempDir()
	wtPath := filepath.Join(wtDir, "wt-dirty")
	branch := "saw/my-slug/wave1-agent-A"

	addWorktree(t, repo, wtPath, branch)

	// Create an untracked file in the worktree to make it dirty.
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := makeManifest("my-slug")
	result, err := ClassifyWorktrees([]string{wtPath}, manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if !result[0].HasChanges {
		t.Error("expected HasChanges=true for dirty worktree")
	}
	if result[0].AgentID != "A" {
		t.Errorf("expected AgentID=A, got %q", result[0].AgentID)
	}
	if result[0].WaveNum != 1 {
		t.Errorf("expected WaveNum=1, got %d", result[0].WaveNum)
	}
	if result[0].Branch != branch {
		t.Errorf("expected Branch=%q, got %q", branch, result[0].Branch)
	}
}

// TestClassifyWorktrees_CleanWorktree verifies that a worktree with no
// uncommitted changes is reported as HasChanges=false.
func TestClassifyWorktrees_CleanWorktree(t *testing.T) {
	repo := makeTestRepo(t)
	wtDir := t.TempDir()
	wtPath := filepath.Join(wtDir, "wt-clean")
	branch := "saw/my-slug/wave2-agent-B"

	addWorktree(t, repo, wtPath, branch)

	// No changes — worktree is clean.
	manifest := makeManifest("my-slug")
	result, err := ClassifyWorktrees([]string{wtPath}, manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].HasChanges {
		t.Error("expected HasChanges=false for clean worktree")
	}
	if result[0].AgentID != "B" {
		t.Errorf("expected AgentID=B, got %q", result[0].AgentID)
	}
	if result[0].WaveNum != 2 {
		t.Errorf("expected WaveNum=2, got %d", result[0].WaveNum)
	}
}

// TestClassifyWorktrees_NonexistentPath verifies that a path that does not
// exist is silently skipped and returns an empty slice with no error.
func TestClassifyWorktrees_NonexistentPath(t *testing.T) {
	manifest := makeManifest("my-slug")
	result, err := ClassifyWorktrees([]string{"/no/such/path/ever"}, manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result for nonexistent path, got %d entries", len(result))
	}
}

// TestClassifyWorktrees_MixedDirtyClean verifies that with two worktrees —
// one dirty and one clean — only the correct HasChanges flags are set.
func TestClassifyWorktrees_MixedDirtyClean(t *testing.T) {
	repo := makeTestRepo(t)
	wtDir := t.TempDir()

	dirtyPath := filepath.Join(wtDir, "wt-dirty")
	cleanPath := filepath.Join(wtDir, "wt-clean")

	addWorktree(t, repo, dirtyPath, "saw/my-slug/wave1-agent-A")
	addWorktree(t, repo, cleanPath, "saw/my-slug/wave1-agent-B")

	// Make the first worktree dirty.
	if err := os.WriteFile(filepath.Join(dirtyPath, "change.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := makeManifest("my-slug")
	result, err := ClassifyWorktrees([]string{dirtyPath, cleanPath}, manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}

	// Find each entry by path.
	byPath := map[string]DirtyWorktree{}
	for _, r := range result {
		byPath[r.Path] = r
	}

	if d, ok := byPath[dirtyPath]; !ok || !d.HasChanges {
		t.Errorf("dirty worktree: expected HasChanges=true; entry: %+v", byPath[dirtyPath])
	}
	if c, ok := byPath[cleanPath]; !ok || c.HasChanges {
		t.Errorf("clean worktree: expected HasChanges=false; entry: %+v", byPath[cleanPath])
	}
}

// TestClassifyWorktrees_SlugFiltering verifies that worktrees belonging to a
// different slug are filtered out and do not appear in the results.
func TestClassifyWorktrees_SlugFiltering(t *testing.T) {
	repo := makeTestRepo(t)
	wtDir := t.TempDir()

	matchPath := filepath.Join(wtDir, "wt-match")
	otherPath := filepath.Join(wtDir, "wt-other")

	addWorktree(t, repo, matchPath, "saw/my-slug/wave1-agent-A")
	addWorktree(t, repo, otherPath, "saw/other-slug/wave1-agent-B")

	manifest := makeManifest("my-slug")
	result, err := ClassifyWorktrees([]string{matchPath, otherPath}, manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 result (only matching slug), got %d", len(result))
	}
	if result[0].Path != matchPath {
		t.Errorf("expected matchPath=%s, got %s", matchPath, result[0].Path)
	}
}
