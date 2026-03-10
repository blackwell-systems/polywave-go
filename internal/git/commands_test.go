package git

import (
	"os/exec"
	"strings"
	"testing"
)

// TestRunInvalidDir verifies that Run returns an error when given a non-existent directory.
func TestRunInvalidDir(t *testing.T) {
	_, err := Run("/nonexistent/path/that/does/not/exist", "status")
	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestRevParseHEAD initializes a temporary git repo, makes an initial commit,
// and verifies that RevParse("HEAD") returns a valid 40-character SHA.
func TestRevParseHEAD(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	// Configure identity for the temp repo (needed for commit)
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %v: %s", err, out)
	}

	// Create a file and make the initial commit
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, out)
	}

	sha, err := RevParse(dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse returned error: %v", err)
	}

	if len(sha) != 40 {
		t.Fatalf("expected 40-char SHA, got %q (len=%d)", sha, len(sha))
	}
}

// TestCommitCount_ValidRefs initializes a temporary git repo, makes commits on a branch,
// and verifies that CommitCount returns the correct number of commits.
func TestCommitCount_ValidRefs(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	// Configure identity for the temp repo
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %v: %s", err, out)
	}

	// Create initial commit on default branch
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, out)
	}

	// Get the current branch name (could be main or master depending on git config)
	out, err := exec.Command("git", "-C", dir, "branch", "--show-current").CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --show-current failed: %v: %s", err, out)
	}
	baseBranch := strings.TrimSpace(string(out))

	// Create a branch
	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "feature").CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b failed: %v: %s", err, out)
	}

	// Make 3 commits on the feature branch
	for i := 1; i <= 3; i++ {
		if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "commit").CombinedOutput(); err != nil {
			t.Fatalf("git commit %d failed: %v: %s", i, err, out)
		}
	}

	// Count commits from base branch to feature
	count, err := CommitCount(dir, baseBranch, "feature")
	if err != nil {
		t.Fatalf("CommitCount returned error: %v", err)
	}

	if count != 3 {
		t.Fatalf("expected 3 commits, got %d", count)
	}
}

// TestCommitCount_InvalidRefs verifies that CommitCount returns an error for invalid refs.
func TestCommitCount_InvalidRefs(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	// Configure identity for the temp repo
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %v: %s", err, out)
	}

	// Create initial commit
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, out)
	}

	// Try to count commits with an invalid ref
	_, err := CommitCount(dir, "main", "nonexistent-ref")
	if err == nil {
		t.Fatal("expected error for nonexistent ref, got nil")
	}
}

// TestCommitCount_NoBranch verifies that CommitCount returns an error for a nonexistent branch.
func TestCommitCount_NoBranch(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	// Configure identity for the temp repo
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %v: %s", err, out)
	}

	// Create initial commit
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, out)
	}

	// Try to count commits to a nonexistent branch
	_, err := CommitCount(dir, "main", "no-such-branch")
	if err == nil {
		t.Fatal("expected error for nonexistent branch, got nil")
	}
}
