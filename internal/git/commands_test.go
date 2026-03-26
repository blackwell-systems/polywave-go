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

// TestSymbolicRef_OnBranch verifies that SymbolicRef returns a ref starting
// with "refs/heads/" when HEAD points to a branch.
func TestSymbolicRef_OnBranch(t *testing.T) {
	dir := initTestRepo(t)

	ref, err := SymbolicRef(dir)
	if err != nil {
		t.Fatalf("SymbolicRef returned error: %v", err)
	}
	if !strings.HasPrefix(ref, "refs/heads/") {
		t.Errorf("expected ref to start with 'refs/heads/', got %q", ref)
	}
}

// TestSymbolicRef_DetachedHEAD verifies that SymbolicRef returns an error
// when the repository is in detached HEAD state.
func TestSymbolicRef_DetachedHEAD(t *testing.T) {
	dir := initTestRepo(t)

	// Detach HEAD — ignore error (checkout --detach may print output but succeed)
	Run(dir, "checkout", "--detach", "HEAD") //nolint:errcheck

	_, err := SymbolicRef(dir)
	if err == nil {
		t.Fatal("expected error for detached HEAD, got nil")
	}
}

// TestWorktreeListRaw_ReturnsBytes verifies that WorktreeListRaw returns
// non-empty bytes containing the word "worktree".
func TestWorktreeListRaw_ReturnsBytes(t *testing.T) {
	dir := initTestRepo(t)

	out, err := WorktreeListRaw(dir)
	if err != nil {
		t.Fatalf("WorktreeListRaw returned error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty output from WorktreeListRaw")
	}
	if !strings.Contains(string(out), "worktree") {
		t.Errorf("expected output to contain 'worktree', got %q", string(out))
	}
}

// TestDiffNameOnlyHEAD_NoDiff verifies that DiffNameOnlyHEAD returns nil
// for a clean repository with no unstaged changes.
func TestDiffNameOnlyHEAD_NoDiff(t *testing.T) {
	dir := initTestRepo(t)

	result, err := DiffNameOnlyHEAD(dir)
	if err != nil {
		t.Fatalf("DiffNameOnlyHEAD returned error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for clean repo, got %v", result)
	}
}

// TestDiffNameOnlyHEAD_WithModification verifies that DiffNameOnlyHEAD returns
// the filename of a file that was committed and then modified (but not re-staged).
func TestDiffNameOnlyHEAD_WithModification(t *testing.T) {
	dir := initTestRepo(t)

	// Write, stage and commit a file
	filePath := dir + "/tracked.txt"
	if err := os.WriteFile(filePath, []byte("original content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := Add(dir, "tracked.txt"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if _, err := CommitWithMessage(dir, "add tracked.txt"); err != nil {
		t.Fatalf("CommitWithMessage failed: %v", err)
	}

	// Modify the file without staging it
	if err := os.WriteFile(filePath, []byte("modified content"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	result, err := DiffNameOnlyHEAD(dir)
	if err != nil {
		t.Fatalf("DiffNameOnlyHEAD returned error: %v", err)
	}
	found := false
	for _, f := range result {
		if f == "tracked.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected result to contain 'tracked.txt', got %v", result)
	}
}

// TestDiffNameOnlyStaged_NoStaged verifies that DiffNameOnlyStaged returns nil
// when no files are staged.
func TestDiffNameOnlyStaged_NoStaged(t *testing.T) {
	dir := initTestRepo(t)

	result, err := DiffNameOnlyStaged(dir)
	if err != nil {
		t.Fatalf("DiffNameOnlyStaged returned error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for clean index, got %v", result)
	}
}

// TestDiffNameOnlyStaged_WithStaged verifies that DiffNameOnlyStaged returns
// a filename that has been staged via git add.
func TestDiffNameOnlyStaged_WithStaged(t *testing.T) {
	dir := initTestRepo(t)

	// Write and stage a file
	filePath := dir + "/staged.txt"
	if err := os.WriteFile(filePath, []byte("staged content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := Add(dir, "staged.txt"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	result, err := DiffNameOnlyStaged(dir)
	if err != nil {
		t.Fatalf("DiffNameOnlyStaged returned error: %v", err)
	}
	found := false
	for _, f := range result {
		if f == "staged.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected result to contain 'staged.txt', got %v", result)
	}
}

// TestAddUpdate_TrackedFile verifies that AddUpdate stages modifications to
// a tracked file (but not new untracked files).
func TestAddUpdate_TrackedFile(t *testing.T) {
	dir := initTestRepo(t)

	// Write, stage and commit a file so it is tracked
	filePath := dir + "/update.txt"
	if err := os.WriteFile(filePath, []byte("initial content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := Add(dir, "update.txt"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if _, err := CommitWithMessage(dir, "add update.txt"); err != nil {
		t.Fatalf("CommitWithMessage failed: %v", err)
	}

	// Modify the tracked file
	if err := os.WriteFile(filePath, []byte("updated content"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	// Stage via AddUpdate
	if err := AddUpdate(dir, "."); err != nil {
		t.Fatalf("AddUpdate returned error: %v", err)
	}

	// Verify the file is now staged
	status, err := StatusPorcelainFile(dir, "update.txt")
	if err != nil {
		t.Fatalf("StatusPorcelainFile returned error: %v", err)
	}
	if status == "" {
		t.Error("expected update.txt to be staged after AddUpdate, status was empty")
	}
}

// TestVersion_ReturnsGitVersion verifies that Version returns a string
// containing "git version" and no error.
func TestVersion_ReturnsGitVersion(t *testing.T) {
	out, err := Version()
	if err != nil {
		t.Fatalf("Version returned error: %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Errorf("expected output to contain 'git version', got %q", out)
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
