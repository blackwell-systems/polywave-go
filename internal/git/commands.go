// Package git wraps the git CLI for operations required by scout-and-wave-go:
// worktree creation and removal, branch merging with conflict detection, diff
// inspection, and repository root resolution. All commands are thin
// wrappers that return structured errors on non-zero exit codes.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// DiffStats holds statistics for a git diff
type DiffStats struct {
	FilesChanged int
	LinesAdded   int
	LinesRemoved int
}

// Run executes a git command in repoPath with the given args.
// It returns combined stdout+stderr output and any error encountered.
func Run(repoPath string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return output, fmt.Errorf("%w: %s", err, strings.TrimSpace(output))
	}
	return output, nil
}

// RunOutput executes a git command in repoPath with the given args.
// It returns stdout-only output (not combined with stderr) and any error.
// Use this for machine-parseable output (e.g., rev-parse, diff --stat).
// Use Run when you want combined stdout+stderr for error messages.
func RunOutput(repoPath string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.Output()
	output := string(out)
	if err != nil {
		return output, fmt.Errorf("%w: %s", err, strings.TrimSpace(output))
	}
	return output, nil
}

// WorktreeAdd creates a new worktree at path on a new branch named branch,
// branching from HEAD of the repository at repoPath.
func WorktreeAdd(repoPath, path, branch string) error {
	_, err := Run(repoPath, "worktree", "add", "-b", branch, path, "HEAD")
	if err != nil {
		return fmt.Errorf("git worktree add failed: %w", err)
	}
	return nil
}

// WorktreePrune removes stale worktree entries from the repository at repoPath.
// This cleans up references to worktrees whose directories have been deleted
// but whose metadata still exists in .git/worktrees/.
func WorktreePrune(repoPath string) error {
	_, err := Run(repoPath, "worktree", "prune")
	if err != nil {
		return fmt.Errorf("git worktree prune failed: %w", err)
	}
	return nil
}

// WorktreeRemove removes the worktree at path from the repository at repoPath.
// --force is required because agent worktrees often contain untracked files
// that git would otherwise refuse to delete.
func WorktreeRemove(repoPath, path string) error {
	_, err := Run(repoPath, "worktree", "remove", "--force", path)
	if err != nil {
		return fmt.Errorf("git worktree remove failed: %w", err)
	}
	return nil
}

// WorktreeList returns a list of [path, branch] pairs for all non-main worktrees
// in the repository at repoPath. The main worktree (first entry) is skipped.
func WorktreeList(repoPath string) ([][2]string, error) {
	out, err := Run(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list failed: %w", err)
	}

	var result [][2]string
	var currentPath string
	var currentBranch string
	isFirst := true

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			// Empty line separates worktree entries
			if !isFirst && currentPath != "" {
				result = append(result, [2]string{currentPath, currentBranch})
			}
			isFirst = false
			currentPath = ""
			currentBranch = ""
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			if isFirst {
				// This is the first worktree entry; mark it as seen but skip
				currentPath = strings.TrimPrefix(line, "worktree ")
			} else {
				currentPath = strings.TrimPrefix(line, "worktree ")
			}
		} else if strings.HasPrefix(line, "branch ") {
			branchRef := strings.TrimPrefix(line, "branch ")
			// branchRef is typically refs/heads/branchname
			parts := strings.Split(branchRef, "/")
			if len(parts) >= 3 {
				currentBranch = strings.Join(parts[2:], "/")
			} else {
				currentBranch = branchRef
			}
		}
	}

	// Handle last entry if not followed by blank line
	if !isFirst && currentPath != "" {
		result = append(result, [2]string{currentPath, currentBranch})
	}

	return result, nil
}

// MergeNoFF performs a non-fast-forward merge of branch into the current HEAD
// of the repository at repoPath, using message as the commit message.
func MergeNoFF(repoPath, branch, message string) error {
	_, err := Run(repoPath, "merge", "--no-ff", branch, "-m", message)
	if err != nil {
		return fmt.Errorf("git merge --no-ff failed: %w", err)
	}
	return nil
}

// MergeNoFFWithOwnership performs a non-fast-forward merge and automatically
// resolves conflicts using the file ownership table.
//
// When a conflict occurs, each conflicting file is resolved deterministically:
//   - File owned by currentAgent → checkout --theirs (agent's version wins)
//   - File owned by any other agent  → checkout --ours  (develop's version wins)
//   - File with no known owner       → abort and return a ConflictError
//
// This works because I1 (disjoint file ownership) guarantees each file belongs
// to at most one agent. A conflict on a file owned by the current agent means
// the agent diverged from develop; its version is authoritative. A conflict on
// a file owned by another agent means the current agent touched something it
// shouldn't have; develop's version (already containing the owner's work) wins.
//
// fileOwners maps relative file path → agent ID (e.g. "web/src/foo.ts" → "A").
// If fileOwners is nil, no auto-resolution is attempted and conflicts are fatal.
func MergeNoFFWithOwnership(repoPath, branch, message, currentAgent string, fileOwners map[string]string) error {
	_, err := Run(repoPath, "merge", "--no-ff", branch, "-m", message)
	if err == nil {
		return nil
	}

	// Check if this is a merge conflict (exit code 1 with conflict markers)
	conflicted, listErr := ConflictedFiles(repoPath)
	if listErr != nil || len(conflicted) == 0 {
		// No conflicted files — git may have auto-resolved via recursive strategy.
		// Check if merge is still in progress or if it completed cleanly.
		if _, inProgress := Run(repoPath, "rev-parse", "MERGE_HEAD"); inProgress != nil {
			// No MERGE_HEAD means git completed the merge successfully despite non-zero exit.
			return nil
		}
		if fileOwners == nil {
			Run(repoPath, "merge", "--abort") //nolint:errcheck
			return fmt.Errorf("git merge --no-ff failed: %w", err)
		}
		// MERGE_HEAD exists but no conflicted files — commit to finish
		if _, commitErr := Run(repoPath, "commit", "--no-edit"); commitErr != nil {
			Run(repoPath, "merge", "--abort") //nolint:errcheck
			return fmt.Errorf("git merge --no-ff failed: %w", err)
		}
		return nil
	}
	if fileOwners == nil {
		Run(repoPath, "merge", "--abort") //nolint:errcheck
		return fmt.Errorf("git merge --no-ff failed: %w", err)
	}

	// Resolve each conflicting file using ownership
	unresolvable := []string{}
	for _, f := range conflicted {
		owner, known := fileOwners[f]
		if !known {
			unresolvable = append(unresolvable, f)
			continue
		}
		strategy := "--ours"
		if owner == currentAgent {
			strategy = "--theirs"
		}
		if _, checkoutErr := Run(repoPath, "checkout", strategy, "--", f); checkoutErr != nil {
			unresolvable = append(unresolvable, f)
			continue
		}
		Run(repoPath, "add", f) //nolint:errcheck
	}

	if len(unresolvable) > 0 {
		Run(repoPath, "merge", "--abort") //nolint:errcheck
		return fmt.Errorf("merge conflict on files with no known owner (cannot auto-resolve): %v", unresolvable)
	}

	// All conflicts resolved — complete the merge
	if _, commitErr := Run(repoPath, "commit", "--no-edit"); commitErr != nil {
		Run(repoPath, "merge", "--abort") //nolint:errcheck
		return fmt.Errorf("failed to commit after conflict resolution: %w", commitErr)
	}
	return nil
}

// ConflictedFiles returns the list of files with unresolved merge conflicts
// in the repository at repoPath.
func ConflictedFiles(repoPath string) ([]string, error) {
	out, err := Run(repoPath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return []string{}, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

// DeleteBranch deletes the named branch from the repository at repoPath.
// Uses -D (force delete) because this is only called during cleanup after
// successful merge, where the branch may not be fast-forward mergeable but
// is known to be safe to delete.
func DeleteBranch(repoPath, branch string) error {
	_, err := Run(repoPath, "branch", "-D", branch)
	if err != nil {
		return fmt.Errorf("git branch -D failed: %w", err)
	}
	return nil
}

// BranchExists returns true if the named branch exists in the repository.
func BranchExists(repoPath, branch string) bool {
	_, err := Run(repoPath, "rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// RevParse resolves ref to a commit SHA in the repository at repoPath.
func RevParse(repoPath, ref string) (string, error) {
	out, err := Run(repoPath, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// IsAncestor returns true if commit is an ancestor of ref (i.e. already merged).
// Uses git merge-base --is-ancestor which exits 0 if true, 1 if false.
func IsAncestor(repoPath, commit, ref string) bool {
	_, err := Run(repoPath, "merge-base", "--is-ancestor", commit, ref)
	return err == nil
}

// DiffNameOnly returns a list of file paths that differ between fromRef and toRef
// in the repository at repoPath.
func DiffNameOnly(repoPath, fromRef, toRef string) ([]string, error) {
	rangeArg := fromRef + ".." + toRef
	out, err := Run(repoPath, "diff", "--name-only", rangeArg)
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only failed: %w", err)
	}

	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return []string{}, nil
	}

	return strings.Split(trimmed, "\n"), nil
}

// CommitCount returns the number of commits between fromRef and toRef
// in the repository at repoPath. Uses git rev-list --count.
func CommitCount(repoPath, fromRef, toRef string) (int, error) {
	rangeArg := fromRef + ".." + toRef
	out, err := Run(repoPath, "rev-list", "--count", rangeArg)
	if err != nil {
		return 0, fmt.Errorf("git rev-list --count failed: %w", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}
	return count, nil
}

// preCommitHookTemplate is the SAW worktree isolation hook that blocks commits
// to main/master branches unless SAW_ALLOW_MAIN_COMMIT=1 is set.
const preCommitHookTemplate = `#!/usr/bin/env bash
# SAW pre-commit guard: Block commits to main/master in Wave agent worktrees
# This hook prevents accidental commits to protected branches during parallel execution.
# Wave agents should only commit to their dedicated wave{N}-agent-{ID} branches.

set -euo pipefail

# Allow bypass via environment variable (for SAW orchestrator merge operations)
if [[ "${SAW_ALLOW_MAIN_COMMIT:-0}" == "1" ]]; then
	exit 0
fi

# Only enforce isolation in Wave agent contexts (presence of .saw-agent-brief.md)
# Orchestrator commits to main are allowed (baseline fixes, state management, etc.)
if [[ ! -f ".saw-agent-brief.md" ]]; then
	# Not in agent worktree - allow commit to any branch (Orchestrator context)
	exit 0
fi

# Get current branch name
branch=$(git rev-parse --abbrev-ref HEAD)

# Block commits to main/master branches from agent worktrees
if [[ "$branch" == "main" || "$branch" == "master" ]]; then
	echo "❌ SAW isolation violation: Cannot commit to $branch from Wave agent worktree"
	echo ""
	echo "Wave agents must commit to their dedicated branches (wave{N}-agent-{ID})."
	echo "If you are the orchestrator performing a merge operation, set:"
	echo "  export SAW_ALLOW_MAIN_COMMIT=1"
	echo ""
	exit 1
fi

# Block go.mod replace directives with deep relative paths (worktree artifact).
# Paths like ../../../../sibling are relative to the worktree depth, not repo root.
# They break after merge. Correct paths use at most ../ (one level up from repo root).
if git diff --cached --name-only | grep -q '^go\.mod$'; then
	if git diff --cached -- go.mod | grep -E '^\+.*=>.*\.\./\.\./\.\.' > /dev/null 2>&1; then
		echo "❌ SAW go.mod guard: replace directive has deep relative path (../../../...)"
		echo ""
		echo "Replace paths in go.mod must be relative to the repo root, not the worktree."
		echo "Do NOT modify replace directives — they are already correct for the repo root."
		echo ""
		exit 1
	fi
fi

# Allow commits to wave branches
exit 0
`

// StatusPorcelain returns the porcelain (machine-readable) status of the
// working tree at repoPath. Returns empty string if clean.
func StatusPorcelain(repoPath string) (string, error) {
	out, err := Run(repoPath, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git status --porcelain failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// AddAll stages all changes (new, modified, deleted) in the repository at repoPath.
func AddAll(repoPath string) error {
	_, err := Run(repoPath, "add", "-A")
	if err != nil {
		return fmt.Errorf("git add -A failed: %w", err)
	}
	return nil
}

// Commit creates a commit with the given message in the repository at repoPath.
// Uses --no-verify to skip hooks (the orchestrator is the authority here).
func Commit(repoPath, message string) (string, error) {
	_, err := Run(repoPath, "commit", "--no-verify", "-m", message)
	if err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}
	sha, err := RevParse(repoPath, "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD after commit: %w", err)
	}
	return sha, nil
}

// ChangedFilesSinceRef returns the list of files changed between ref and HEAD
// in the repository at repoPath.
func ChangedFilesSinceRef(repoPath, ref string) ([]string, error) {
	return DiffNameOnly(repoPath, ref, "HEAD")
}

// InstallHooks generates and installs the SAW pre-commit hook in a worktree.
// It writes the hook template to the worktree's hooks directory, making it executable.
// Creates the hooks directory if it doesn't exist.
//
// For worktrees, the hook path is: .git/worktrees/<name>/hooks/pre-commit
// For regular repos, the hook path is: .git/hooks/pre-commit
//
// Returns an error if:
// - The worktree path is invalid or doesn't exist
// - File I/O operations fail
func InstallHooks(repoPath, worktreePath string) error {
	// Use embedded hook template (no dependency on main repo hook)
	hookContent := []byte(preCommitHookTemplate)

	// Read the .git file in the worktree to find the gitdir pointer
	gitFilePath := filepath.Join(worktreePath, ".git")
	gitFileContent, err := os.ReadFile(gitFilePath)
	if err != nil {
		return fmt.Errorf("failed to read worktree .git file at %s: %w", gitFilePath, err)
	}

	// Parse "gitdir: /path/to/repo/.git/worktrees/<name>"
	gitFileStr := strings.TrimSpace(string(gitFileContent))
	if !strings.HasPrefix(gitFileStr, "gitdir: ") {
		return fmt.Errorf("malformed .git file at %s: expected 'gitdir: ...' but got: %s", gitFilePath, gitFileStr)
	}
	worktreeGitDir := strings.TrimPrefix(gitFileStr, "gitdir: ")

	// Create hooks directory if it doesn't exist
	hooksDir := filepath.Join(worktreeGitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory at %s: %w", hooksDir, err)
	}

	// Write hook to target path with executable permissions
	targetHookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(targetHookPath, hookContent, 0755); err != nil {
		return fmt.Errorf("failed to write hook to %s: %w", targetHookPath, err)
	}

	return nil
}
// GetWorktreeBaseCommit returns the commit SHA that the worktree's branch
// diverged from the main repository's HEAD. This is the point at which the
// worktree was effectively "created from" — i.e. the merge-base between the
// worktree branch and the main repo's current HEAD.
//
// If the worktree was just created from current HEAD and no new commits have
// landed on main since, the result equals HEAD (worktree is fresh).
// If main has advanced since the worktree was created, the result differs
// from HEAD (worktree is stale).
func GetWorktreeBaseCommit(repoPath, worktreePath string) (string, error) {
	ref, err := SymbolicRef(worktreePath)
	if err != nil {
		return "", fmt.Errorf("failed to get worktree branch: %w", err)
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	base, err := MergeBase(repoPath, "HEAD", branch)
	if err != nil {
		return "", fmt.Errorf("failed to get merge base for branch %s: %w", branch, err)
	}
	return base, nil
}

// WorktreeExists checks if a worktree at the given path is registered in git's worktree list.
func WorktreeExists(repoPath, worktreePath string) bool {
	worktrees, err := WorktreeList(repoPath)
	if err != nil {
		return false
	}

	cleanPath := filepath.Clean(worktreePath)
	for _, wt := range worktrees {
		if filepath.Clean(wt[0]) == cleanPath {
			return true
		}
	}
	return false
}

// VerifyHookInWorktree checks if the pre-commit hook exists and is valid in a worktree.
func VerifyHookInWorktree(worktreePath string) (bool, error) {
	// Read .git file to find gitdir
	gitFile := filepath.Join(worktreePath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return false, fmt.Errorf("failed to read .git file: %w", err)
	}

	gitdir := strings.TrimSpace(strings.TrimPrefix(string(content), "gitdir: "))
	if gitdir == "" {
		return false, fmt.Errorf(".git file malformed: %s", gitFile)
	}

	hookPath := filepath.Join(gitdir, "hooks", "pre-commit")

	// Check if hook exists and is executable
	info, err := os.Stat(hookPath)
	if os.IsNotExist(err) {
		return false, nil // Missing hook is not an error
	}
	if err != nil {
		return false, fmt.Errorf("failed to stat hook: %w", err)
	}

	if info.Mode()&0111 == 0 {
		return false, nil // Not executable
	}

	// Check hook content contains SAW isolation logic
	hookContent, err := os.ReadFile(hookPath)
	if err != nil {
		return false, fmt.Errorf("failed to read hook: %w", err)
	}

	contentStr := string(hookContent)
	if !strings.Contains(contentStr, "SAW_ALLOW_MAIN_COMMIT") && !strings.Contains(contentStr, "SAW pre-commit guard") {
		return false, nil // Hook doesn't contain SAW logic
	}

	return true, nil
}

// StatusPorcelainFile returns the git porcelain (machine-readable) status for a
// specific file in the repository at repoPath. Returns empty string if the file
// is unmodified. Returns an error if the git command fails.
func StatusPorcelainFile(repoPath, filePath string) (string, error) {
	out, err := Run(repoPath, "status", "--porcelain", filePath)
	if err != nil {
		return "", fmt.Errorf("git status --porcelain %s failed: %w", filePath, err)
	}
	return strings.TrimSpace(out), nil
}

// Add stages the given file paths for commit in the repository at repoPath.
// Unlike AddAll, this stages only the specified paths (not all changes).
func Add(repoPath string, paths ...string) error {
	args := append([]string{"add"}, paths...)
	_, err := Run(repoPath, args...)
	if err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}
	return nil
}

// CommitWithMessage creates a commit with the given message in the repository
// at repoPath. Unlike Commit(), this does NOT pass --no-verify, so pre-commit
// hooks run. Use this for protocol-layer commits (state transitions) where
// hook enforcement is appropriate.
// Returns the commit SHA string on success.
func CommitWithMessage(repoPath, message string) (string, error) {
	_, err := Run(repoPath, "commit", "-m", message)
	if err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}
	sha, err := RevParse(repoPath, "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD after commit: %w", err)
	}
	return sha, nil
}

// LogOneline returns lines from git log --oneline for the given args
// (e.g., a commit range like "abc123..branchname"). Returns an empty
// slice (not an error) when no commits match. Returns an error only
// on git system failures (not when the branch simply has no commits).
func LogOneline(repoPath string, args ...string) ([]string, error) {
	allArgs := append([]string{"log", "--oneline"}, args...)
	out, err := Run(repoPath, allArgs...)
	if err != nil {
		// git log exits 128 when a ref doesn't exist; treat as empty, not error
		return []string{}, nil
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return []string{}, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

// SymbolicRef returns the full ref that HEAD points to in the repository
// at repoPath (e.g., "refs/heads/main"). Returns an error for detached HEAD
// or when the git command fails.
func SymbolicRef(repoPath string) (string, error) {
	out, err := RunOutput(repoPath, "symbolic-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git symbolic-ref failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// WorktreeListRaw executes git worktree list --porcelain in repoPath and
// returns the raw stdout bytes for callers that parse the porcelain format directly.
func WorktreeListRaw(repoPath string) ([]byte, error) {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list --porcelain failed: %w", err)
	}
	return out, nil
}

// DiffNameOnlyHEAD returns file paths that differ between HEAD and the working
// tree (unstaged changes) in the repository at repoPath. Returns nil and no
// error when there are no differences.
func DiffNameOnlyHEAD(repoPath string) ([]string, error) {
	out, err := RunOutput(repoPath, "diff", "--name-only", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only HEAD failed: %w", err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

// DiffNameOnlyStaged returns file paths that are staged for commit (index vs HEAD)
// in the repository at repoPath. Returns nil and no error when nothing is staged.
func DiffNameOnlyStaged(repoPath string) ([]string, error) {
	out, err := RunOutput(repoPath, "diff", "--name-only", "--cached")
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only --cached failed: %w", err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

// AddUpdate stages updates (modifications and deletions, not new files) for all
// tracked files under path in the repository at repoPath. Equivalent to git add -u <path>.
func AddUpdate(repoPath, path string) error {
	_, err := Run(repoPath, "add", "-u", path)
	if err != nil {
		return fmt.Errorf("git add -u failed: %w", err)
	}
	return nil
}

// MergeBase returns the best common ancestor commit SHA of ref1 and ref2
// in the repository at repoPath. Used for hunk-level conflict prediction.
func MergeBase(repoPath, ref1, ref2 string) (string, error) {
	out, err := RunOutput(repoPath, "merge-base", ref1, ref2)
	if err != nil {
		return "", fmt.Errorf("git merge-base failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// DiffUnifiedZero returns the unified diff with zero context lines between
// fromRef and toRef for the specified file path. Zero context gives minimal
// hunk ranges for accurate conflict prediction.
// Returns empty string when there are no differences.
func DiffUnifiedZero(repoPath, fromRef, toRef, file string) (string, error) {
	out, err := RunOutput(repoPath, "diff", "--unified=0", fromRef+".."+toRef, "--", file)
	if err != nil {
		return "", fmt.Errorf("git diff --unified=0 failed: %w", err)
	}
	return out, nil
}

// Version returns the raw output of git --version (e.g., "git version 2.39.2").
// Does not require a repository path. Returns an error if git is not found on PATH.
func Version() (string, error) {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return "", fmt.Errorf("git --version failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetDiffStats runs git diff --numstat between fromRef and toRef for file
// and returns parsed statistics. Returns zero stats on empty diff (no changes).
// Returns an error if the diff contains binary files (not supported) or if git fails.
func GetDiffStats(repoPath, fromRef, toRef, file string) (*DiffStats, error) {
	rangeArg := fromRef + ".." + toRef
	out, err := RunOutput(repoPath, "diff", "--numstat", rangeArg, "--", file)
	if err != nil {
		return nil, fmt.Errorf("git diff --numstat failed: %w", err)
	}

	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		// No changes — return zero stats
		return &DiffStats{FilesChanged: 0, LinesAdded: 0, LinesRemoved: 0}, nil
	}

	// Parse --numstat format: <added>\t<removed>\t<file>
	// Binary files show "-" for stats — return error
	lines := strings.Split(trimmed, "\n")
	stats := &DiffStats{}

	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue // Malformed line, skip
		}

		addedStr := fields[0]
		removedStr := fields[1]

		// Binary files show "-" for stats
		if addedStr == "-" || removedStr == "-" {
			return nil, fmt.Errorf("binary file detected in diff (not supported): %s", fields[2])
		}

		added, err := strconv.Atoi(addedStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse added lines %q: %w", addedStr, err)
		}

		removed, err := strconv.Atoi(removedStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse removed lines %q: %w", removedStr, err)
		}

		stats.FilesChanged++
		stats.LinesAdded += added
		stats.LinesRemoved += removed
	}

	return stats, nil
}
