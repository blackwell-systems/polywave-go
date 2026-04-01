package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// minimalProgramManifestYAML returns a minimal PROGRAM manifest YAML for tests.
func minimalProgramManifestYAML(implSlug string) string {
	return `title: test-program
program_slug: test-prog
state: TIER_EXECUTING
impls:
  - slug: ` + implSlug + `
    title: Test Impl
    tier: 1
    status: reviewed
tiers:
  - number: 1
    impls:
      - ` + implSlug + `
completion: {}
`
}

// initTempGitRepo initializes a temporary git repo with an initial commit.
// Returns the repo directory path.
func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	// Create an initial commit so HEAD exists
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	return dir
}

// TestFinalizeTierEngine_MissingManifest verifies that a nonexistent manifest path
// returns a non-nil error.
func TestFinalizeTierEngine_MissingManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := FinalizeTierEngine(context.Background(), FinalizeTierOpts{
		ManifestPath: filepath.Join(dir, "nonexistent.yaml"),
		TierNumber:   1,
		RepoDir:      dir,
	})
	if err == nil {
		t.Fatal("expected non-nil error for missing manifest, got nil")
	}
}

// TestFinalizeTierEngine_TierNotFound verifies that specifying a tier number
// that does not exist in the manifest returns a non-nil error.
func TestFinalizeTierEngine_TierNotFound(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "PROGRAM.yaml")
	if err := os.WriteFile(manifestPath, []byte(minimalProgramManifestYAML("test-impl")), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := FinalizeTierEngine(context.Background(), FinalizeTierOpts{
		ManifestPath: manifestPath,
		TierNumber:   99,
		RepoDir:      dir,
	})
	if err == nil {
		t.Fatal("expected non-nil error for tier not found, got nil")
	}
}

// TestMergeIMPLBranchWorktreeAware_NoWorktrees initializes a temp git repo and
// calls mergeIMPLBranchWorktreeAware with a branch that does not exist.
// Expects a non-nil error since MergeNoFF will fail on the missing branch.
func TestMergeIMPLBranchWorktreeAware_NoWorktrees(t *testing.T) {
	repoDir := initTempGitRepo(t)

	err := mergeIMPLBranchWorktreeAware(repoDir, "nonexistent-branch", "test merge message")
	if err == nil {
		t.Fatal("expected non-nil error when merging nonexistent branch, got nil")
	}
}

// TestFinalizeTierEngine_AllImplsAlreadyArchived verifies that when the IMPL doc
// file does not exist on disk (already archived), the slug appears in ImplsSkipped.
func TestFinalizeTierEngine_AllImplsAlreadyArchived(t *testing.T) {
	dir := t.TempDir()

	// Create minimal docs structure so ResolveIMPLPath can search
	docsIMPLDir := filepath.Join(dir, "docs", "IMPL")
	if err := os.MkdirAll(docsIMPLDir, 0755); err != nil {
		t.Fatal(err)
	}
	docsIMPLCompleteDir := filepath.Join(dir, "docs", "IMPL", "complete")
	if err := os.MkdirAll(docsIMPLCompleteDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write manifest but do NOT create the IMPL doc file
	manifestPath := filepath.Join(dir, "PROGRAM.yaml")
	implSlug := "test-impl"
	if err := os.WriteFile(manifestPath, []byte(minimalProgramManifestYAML(implSlug)), 0644); err != nil {
		t.Fatal(err)
	}

	result, _ := FinalizeTierEngine(context.Background(), FinalizeTierOpts{
		ManifestPath: manifestPath,
		TierNumber:   1,
		RepoDir:      dir,
	})

	// The IMPL slug should appear in ImplsSkipped (not ImplsClosed)
	found := false
	for _, s := range result.ImplsSkipped {
		if s == implSlug {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in ImplsSkipped, got ImplsSkipped=%v ImplsClosed=%v",
			implSlug, result.ImplsSkipped, result.ImplsClosed)
	}
}
