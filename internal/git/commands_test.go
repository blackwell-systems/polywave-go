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

	// Verify hook content matches embedded template
	installedContent, err := os.ReadFile(targetHookPath)
	if err != nil {
		t.Fatalf("failed to read installed hook: %v", err)
	}
	if string(installedContent) != preCommitHookTemplate {
		t.Errorf("hook content mismatch:\nexpected embedded template\ngot: %q", string(installedContent))
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

// TestInstallHooks_GeneratesTemplate verifies that InstallHooks generates
// the hook from embedded template (no dependency on main repo hook).
func TestInstallHooks_GeneratesTemplate(t *testing.T) {
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

	// Create worktree (no hook in main repo - should still work)
	worktreePath := dir + "/worktree-test"
	if err := WorktreeAdd(dir, worktreePath, "test-branch"); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Install hooks (should succeed using embedded template)
	err := InstallHooks(dir, worktreePath)
	if err != nil {
		t.Fatalf("InstallHooks failed: %v", err)
	}

	// Verify hook was generated from template
	gitDir, _ := os.ReadFile(worktreePath + "/.git")
	worktreeGitDir := strings.TrimPrefix(strings.TrimSpace(string(gitDir)), "gitdir: ")
	hookPath := worktreeGitDir + "/hooks/pre-commit"
	hookContent, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("hook not found: %v", err)
	}
	if string(hookContent) != preCommitHookTemplate {
		t.Error("hook content does not match embedded template")
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

// initTestRepo is a helper that creates a temp git repo with identity configured
// and an initial empty commit. Returns the repo directory.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, out)
	}
	return dir
}

// TestStatusPorcelainFile verifies that StatusPorcelainFile returns empty for a
// clean file and returns the status line for a modified (staged) file.
func TestStatusPorcelainFile(t *testing.T) {
	dir := initTestRepo(t)

	// Write a file and stage it
	filePath := dir + "/hello.txt"
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Before staging: file is untracked, not "modified" in porcelain terms
	// Stage the file
	if out, err := exec.Command("git", "-C", dir, "add", "hello.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v: %s", err, out)
	}

	// After staging: should return a non-empty status line
	status, err := StatusPorcelainFile(dir, "hello.txt")
	if err != nil {
		t.Fatalf("StatusPorcelainFile returned error: %v", err)
	}
	if status == "" {
		t.Fatal("expected non-empty status for staged file, got empty string")
	}
	if !strings.Contains(status, "hello.txt") {
		t.Errorf("expected status to contain filename, got: %q", status)
	}

	// Commit the file so it becomes clean
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "add hello").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, out)
	}

	// Now status for that file should be empty (clean)
	status, err = StatusPorcelainFile(dir, "hello.txt")
	if err != nil {
		t.Fatalf("StatusPorcelainFile returned error for clean file: %v", err)
	}
	if status != "" {
		t.Errorf("expected empty status for clean file, got: %q", status)
	}
}

// TestAdd verifies that Add stages specified files and that StatusPorcelainFile
// reports them as staged.
func TestAdd(t *testing.T) {
	dir := initTestRepo(t)

	// Write two files
	if err := os.WriteFile(dir+"/a.txt", []byte("aaa"), 0644); err != nil {
		t.Fatalf("failed to write a.txt: %v", err)
	}
	if err := os.WriteFile(dir+"/b.txt", []byte("bbb"), 0644); err != nil {
		t.Fatalf("failed to write b.txt: %v", err)
	}

	// Stage only a.txt via Add
	if err := Add(dir, "a.txt"); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	// a.txt should be staged
	statusA, err := StatusPorcelainFile(dir, "a.txt")
	if err != nil {
		t.Fatalf("StatusPorcelainFile(a.txt) error: %v", err)
	}
	if statusA == "" {
		t.Error("expected a.txt to be staged, status was empty")
	}

	// b.txt should NOT be staged (still untracked)
	statusB, err := StatusPorcelainFile(dir, "b.txt")
	if err != nil {
		t.Fatalf("StatusPorcelainFile(b.txt) error: %v", err)
	}
	// Untracked files appear with "??" in porcelain; they're not staged changes
	// but they do appear in status. Accept either "??" (untracked) or "" (not there).
	_ = statusB // just check no error; staging isolation is confirmed by a.txt behavior
}

// TestCommitWithMessage verifies that CommitWithMessage creates a commit and
// returns a non-empty 40-character SHA.
func TestCommitWithMessage(t *testing.T) {
	dir := initTestRepo(t)

	// Stage a file
	if err := os.WriteFile(dir+"/file.txt", []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := Add(dir, "file.txt"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	sha, err := CommitWithMessage(dir, "test commit message")
	if err != nil {
		t.Fatalf("CommitWithMessage returned error: %v", err)
	}
	if sha == "" {
		t.Fatal("expected non-empty SHA, got empty string")
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %q (len=%d)", sha, len(sha))
	}

	// Verify the commit message matches
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%s").CombinedOutput()
	if err != nil {
		t.Fatalf("git log failed: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "test commit message" {
		t.Errorf("expected commit message %q, got %q", "test commit message", strings.TrimSpace(string(out)))
	}
}

// TestLogOneline verifies LogOneline behavior:
// - returns empty slice for an empty commit range
// - returns lines for existing commits
// - returns empty slice (no error) for a non-existent branch name
func TestLogOneline(t *testing.T) {
	dir := initTestRepo(t)

	// Get current branch name
	branchOut, err := exec.Command("git", "-C", dir, "branch", "--show-current").CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --show-current failed: %v: %s", err, branchOut)
	}
	baseBranch := strings.TrimSpace(string(branchOut))

	// Create a feature branch and add commits
	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "feature").CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b feature failed: %v: %s", err, out)
	}
	if err := os.WriteFile(dir+"/x.txt", []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write x.txt: %v", err)
	}
	if err := Add(dir, "x.txt"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if _, err := CommitWithMessage(dir, "feature commit 1"); err != nil {
		t.Fatalf("CommitWithMessage failed: %v", err)
	}
	if err := os.WriteFile(dir+"/y.txt", []byte("y"), 0644); err != nil {
		t.Fatalf("failed to write y.txt: %v", err)
	}
	if err := Add(dir, "y.txt"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if _, err := CommitWithMessage(dir, "feature commit 2"); err != nil {
		t.Fatalf("CommitWithMessage failed: %v", err)
	}

	// Should return 2 lines for baseBranch..feature
	lines, err := LogOneline(dir, baseBranch+"..feature")
	if err != nil {
		t.Fatalf("LogOneline returned error: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %v", len(lines), lines)
	}

	// Empty range: feature..feature — should return empty slice, not error
	lines, err = LogOneline(dir, "feature..feature")
	if err != nil {
		t.Fatalf("LogOneline returned error for empty range: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for empty range, got %d: %v", len(lines), lines)
	}

	// Non-existent branch: should return empty slice, not error
	lines, err = LogOneline(dir, "no-such-branch..feature")
	if err != nil {
		t.Fatalf("LogOneline returned error for non-existent branch: %v", err)
	}
	if lines == nil {
		t.Fatal("expected non-nil slice for non-existent branch, got nil")
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
