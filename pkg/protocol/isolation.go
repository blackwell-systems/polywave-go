package protocol

import (
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// IsolationResult reports whether an agent is running in the correct worktree.
type IsolationResult struct {
	OK     bool     `json:"ok"`
	Branch string   `json:"branch"`
	Errors []string `json:"errors,omitempty"`
}

// VerifyIsolation checks that the agent is on the expected branch, that the
// worktree is registered with git, and that repoDir actually points to the
// worktree (not the main repo). Agents call this in Field 0 before doing
// any work (E12: isolation verification).
//
// repoDir is the worktree directory (where the agent is running).
// expectedBranch is the branch name assigned by saw create-worktrees
// (e.g. "wave1-agent-A").
//
// CRITICAL: repoDir must be the absolute path to the worktree. If the agent
// passes "." or a relative path, this function resolves it to an absolute path
// and verifies it contains ".claude/worktrees/" to ensure it's not the main repo.
func VerifyIsolation(repoDir, expectedBranch string) (*IsolationResult, error) {
	result := &IsolationResult{OK: true}

	// Get absolute path of repoDir (resolves "." and relative paths)
	absPath, err := git.Run(repoDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("VerifyIsolation: could not determine repository path: %w", err)
	}
	absPath = strings.TrimSpace(absPath)

	// Check if this is actually a worktree (not the main repo)
	// Worktrees created by sawtools are always in .claude/worktrees/
	if !strings.Contains(absPath, ".claude/worktrees/") {
		result.OK = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("not in a worktree: %q does not contain '.claude/worktrees/' — agent may be running in main repo", absPath))
	}

	// Check current branch
	out, err := git.Run(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("VerifyIsolation: could not determine current branch: %w", err)
	}
	currentBranch := strings.TrimSpace(out)
	result.Branch = currentBranch

	if currentBranch != expectedBranch {
		result.OK = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("branch mismatch: expected %q, got %q", expectedBranch, currentBranch))
	}

	// Verify this worktree is registered (not running on main by accident)
	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		return nil, fmt.Errorf("VerifyIsolation: could not list worktrees: %w", err)
	}
	if len(worktrees) == 0 {
		result.OK = false
		result.Errors = append(result.Errors,
			"no registered worktrees found — agent may be running on main branch")
	}

	return result, nil
}
