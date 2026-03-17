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

// WorktreeAdd creates a new worktree at path on a new branch named branch,
// branching from HEAD of the repository at repoPath.
func WorktreeAdd(repoPath, path, branch string) error {
	_, err := Run(repoPath, "worktree", "add", "-b", branch, path, "HEAD")
	if err != nil {
		return fmt.Errorf("git worktree add failed: %w", err)
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
	if listErr != nil || len(conflicted) == 0 || fileOwners == nil {
		// Not a conflict, or no ownership map — abort and surface original error
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

# Get current branch name
branch=$(git rev-parse --abbrev-ref HEAD)

# Block commits to main/master branches
if [[ "$branch" == "main" || "$branch" == "master" ]]; then
	echo "❌ SAW isolation violation: Cannot commit to $branch from Wave agent worktree"
	echo ""
	echo "Wave agents must commit to their dedicated branches (wave{N}-agent-{ID})."
	echo "If you are the orchestrator performing a merge operation, set:"
	echo "  export SAW_ALLOW_MAIN_COMMIT=1"
	echo ""
	exit 1
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
