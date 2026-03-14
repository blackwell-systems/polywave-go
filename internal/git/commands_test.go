package git

import (
	"os"
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

// TestInstallHooks_Success verifies that InstallHooks correctly copies a hook
// from the main repo to a worktree, making it executable.
func TestInstallHooks_Success(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	// Configure identity
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %v: %s", err, out)
	}

	// Create initial commit (required for worktree)
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, out)
	}

	// Create pre-commit hook in main repo
	hookContent := "#!/bin/sh\necho 'SAW pre-commit hook'\n"
	hookPath := dir + "/.git/hooks/pre-commit"
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		t.Fatalf("failed to create source hook: %v", err)
	}

	// Create worktree
	worktreePath := dir + "/worktree-test"
	if err := WorktreeAdd(dir, worktreePath, "test-branch"); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Install hooks
	if err := InstallHooks(dir, worktreePath); err != nil {
		t.Fatalf("InstallHooks failed: %v", err)
	}

	// Read .git file to find the actual hook location
	gitFileContent, err := os.ReadFile(worktreePath + "/.git")
	if err != nil {
		t.Fatalf("failed to read .git file: %v", err)
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(string(gitFileContent), "gitdir: "))
	targetHookPath := gitDir + "/hooks/pre-commit"

	// Verify hook exists
	if _, err := os.Stat(targetHookPath); err != nil {
		t.Fatalf("hook not found at %s: %v", targetHookPath, err)
	}

	// Verify hook content matches source
	installedContent, err := os.ReadFile(targetHookPath)
	if err != nil {
		t.Fatalf("failed to read installed hook: %v", err)
	}
	if string(installedContent) != hookContent {
		t.Errorf("hook content mismatch:\nexpected: %q\ngot: %q", hookContent, string(installedContent))
	}

	// Verify hook is executable
	info, err := os.Stat(targetHookPath)
	if err != nil {
		t.Fatalf("failed to stat hook: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("hook is not executable: mode=%v", info.Mode())
	}
}

// TestInstallHooks_MissingSourceHook verifies that InstallHooks returns an error
// when the source hook doesn't exist in the main repo.
func TestInstallHooks_MissingSourceHook(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	// Configure identity
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

	// Create worktree (no hook in main repo)
	worktreePath := dir + "/worktree-test"
	if err := WorktreeAdd(dir, worktreePath, "test-branch"); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Try to install hooks (should fail)
	err := InstallHooks(dir, worktreePath)
	if err == nil {
		t.Fatal("expected error for missing source hook, got nil")
	}

	if !strings.Contains(err.Error(), "pre-commit hook not found") {
		t.Errorf("expected error to contain 'pre-commit hook not found', got: %v", err)
	}
}

// TestInstallHooks_InvalidWorktreePath verifies that InstallHooks returns an error
// when given a nonexistent worktree path.
func TestInstallHooks_InvalidWorktreePath(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	// Create pre-commit hook
	hookContent := "#!/bin/sh\necho 'test'\n"
	hookPath := dir + "/.git/hooks/pre-commit"
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		t.Fatalf("failed to create source hook: %v", err)
	}

	// Try to install to nonexistent worktree
	err := InstallHooks(dir, "/nonexistent/worktree/path")
	if err == nil {
		t.Fatal("expected error for invalid worktree path, got nil")
	}
}

// TestInstallHooks_CreatesHooksDirectory verifies that InstallHooks creates
// the hooks directory if it doesn't exist.
func TestInstallHooks_CreatesHooksDirectory(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	// Configure identity
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

	// Create pre-commit hook
	hookContent := "#!/bin/sh\necho 'test'\n"
	hookPath := dir + "/.git/hooks/pre-commit"
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		t.Fatalf("failed to create source hook: %v", err)
	}

	// Create worktree
	worktreePath := dir + "/worktree-test"
	if err := WorktreeAdd(dir, worktreePath, "test-branch"); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Remove hooks directory if it exists (simulate fresh worktree)
	gitFileContent, err := os.ReadFile(worktreePath + "/.git")
	if err != nil {
		t.Fatalf("failed to read .git file: %v", err)
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(string(gitFileContent), "gitdir: "))
	hooksDir := gitDir + "/hooks"
	os.RemoveAll(hooksDir)

	// Install hooks (should create directory)
	if err := InstallHooks(dir, worktreePath); err != nil {
		t.Fatalf("InstallHooks failed: %v", err)
	}

	// Verify hooks directory was created
	if _, err := os.Stat(hooksDir); err != nil {
		t.Fatalf("hooks directory not created at %s: %v", hooksDir, err)
	}

	// Verify hook was installed
	targetHookPath := hooksDir + "/pre-commit"
	if _, err := os.Stat(targetHookPath); err != nil {
		t.Fatalf("hook not found at %s: %v", targetHookPath, err)
	}
}
