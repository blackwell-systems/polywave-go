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

// VerifyIsolation checks that the agent is on the expected branch and that the
// worktree is registered with git. Agents call this in Field 0 before doing
// any work (E12: isolation verification).
//
// repoDir is the worktree directory (where the agent is running).
// expectedBranch is the branch name assigned by saw create-worktrees
// (e.g. "wave1-agent-A").
func VerifyIsolation(repoDir, expectedBranch string) (*IsolationResult, error) {
	result := &IsolationResult{OK: true}

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
