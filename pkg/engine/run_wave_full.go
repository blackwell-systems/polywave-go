package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// RunWaveFullOpts configures a full wave lifecycle execution.
type RunWaveFullOpts struct {
	ManifestPath string // path to IMPL manifest file
	RepoPath     string // absolute path to the repository
	WaveNum      int    // wave number to execute
	MergeTarget  string // target branch for merge; empty = HEAD (default)
}

// RunWaveFullResult captures the results of all wave lifecycle steps.
type RunWaveFullResult struct {
	Wave             int                              `json:"wave"`
	WorktreesCreated *protocol.CreateWorktreesResult  `json:"worktrees_created"`
	CommitsVerified  *protocol.VerifyCommitsResult    `json:"commits_verified"`
	Merged           *protocol.MergeAgentsResult      `json:"merged"`
	BuildVerified    *protocol.VerifyBuildResult      `json:"build_verified"`
	Cleaned          *protocol.CleanupResult          `json:"cleaned"`
	Success          bool                             `json:"success"`
}

// RunWaveFull orchestrates a complete wave lifecycle: worktree creation,
// agent execution (external), commit verification, merge, build verification,
// and cleanup.
//
// This function handles the pre-agent and post-agent orchestration steps.
// The caller is responsible for launching agents between worktree creation
// and commit verification.
//
// Returns a RunWaveFullResult with detailed status for each step, and an
// error if any critical step fails. Success is true only if both test and
// lint commands pass during build verification.
func RunWaveFull(ctx context.Context, opts RunWaveFullOpts) (*RunWaveFullResult, error) {
	result := &RunWaveFullResult{Wave: opts.WaveNum}

	// Step 1: Create worktrees for all agents in the wave
	wt, err := protocol.CreateWorktrees(opts.ManifestPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("create worktrees: %w", err)
	}
	result.WorktreesCreated = wt

	// Step 2: Agent execution happens externally (orchestrator launches agents)
	// This function handles pre/post agent work only.
	// The caller is responsible for launching agents between CreateWorktrees and VerifyCommits.

	// Step 3: Verify commits from all agents
	vc, err := protocol.VerifyCommits(opts.ManifestPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("verify commits: %w", err)
	}
	result.CommitsVerified = vc
	if !vc.AllValid {
		return result, fmt.Errorf("commit verification failed: not all agents have commits")
	}

	// Step 4: Merge agent branches into main
	ma, err := protocol.MergeAgents(opts.ManifestPath, opts.WaveNum, opts.RepoPath, opts.MergeTarget)
	if err != nil {
		return result, fmt.Errorf("merge agents: %w", err)
	}
	result.Merged = ma
	if !ma.Success {
		return result, fmt.Errorf("merge failed")
	}

	// Step 5: Verify build (run test and lint commands)
	bv, err := protocol.VerifyBuild(opts.ManifestPath, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("verify build: %w", err)
	}
	result.BuildVerified = bv

	// Step 6: Cleanup worktrees and branches
	cl, err := protocol.Cleanup(opts.ManifestPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("cleanup: %w", err)
	}
	result.Cleaned = cl

	// Success is true only if both test and lint passed
	result.Success = bv.TestPassed && bv.LintPassed
	return result, nil
}
