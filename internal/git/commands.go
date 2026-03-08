// Package git wraps the git CLI for operations required by scout-and-wave-go:
// worktree creation and removal, branch merging with conflict detection, diff
// inspection, and repository root resolution. All commands are thin
// wrappers that return structured errors on non-zero exit codes.
package git

import (
	"fmt"
	"os/exec"
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

// DeleteBranch deletes the named branch from the repository at repoPath.
func DeleteBranch(repoPath, branch string) error {
	_, err := Run(repoPath, "branch", "-d", branch)
	if err != nil {
		return fmt.Errorf("git branch -d failed: %w", err)
	}
	return nil
}

// RevParse resolves ref to a commit SHA in the repository at repoPath.
func RevParse(repoPath, ref string) (string, error) {
	out, err := Run(repoPath, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}
	return strings.TrimSpace(out), nil
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
