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
	WorktreesCreated *protocol.CreateWorktreesData  `json:"worktrees_created"`
	CommitsVerified  *protocol.VerifyCommitsData    `json:"commits_verified"`
	Merged           *protocol.MergeAgentsData      `json:"merged"`
	BuildVerified    *protocol.VerifyBuildData      `json:"build_verified"`
	Cleaned          *protocol.CleanupData          `json:"cleaned"`
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
	wtRes := protocol.CreateWorktrees(opts.ManifestPath, opts.WaveNum, opts.RepoPath)
	if !wtRes.IsSuccess() {
		return result, fmt.Errorf("create worktrees: %v", wtRes.Errors)
	}
	wt := wtRes.GetData()
	result.WorktreesCreated = &wt

	// Step 2: Agent execution happens externally (orchestrator launches agents)
	// This function handles pre/post agent work only.
	// The caller is responsible for launching agents between CreateWorktrees and VerifyCommits.

	// Step 3: Verify commits from all agents
	vcRes := protocol.VerifyCommits(opts.ManifestPath, opts.WaveNum, opts.RepoPath)
	vc := vcRes.GetData()
	result.CommitsVerified = &vc

	if !vcRes.IsSuccess() {
		return result, fmt.Errorf("verify commits: %v", vcRes.Errors)
	}

	// Check if all agents have commits
	allValid := true
	for _, agent := range vc.Agents {
		if !agent.HasCommits {
			allValid = false
			break
		}
	}
	if !allValid {
		return result, fmt.Errorf("commit verification failed: not all agents have commits")
	}

	// Step 4: Merge agent branches into main
	maRes, err := protocol.MergeAgents(opts.ManifestPath, opts.WaveNum, opts.RepoPath, opts.MergeTarget)
	if err != nil {
		return result, fmt.Errorf("merge agents: %w", err)
	}
	if !maRes.IsSuccess() {
		return result, fmt.Errorf("merge agents: %v", maRes.Errors)
	}
	ma := maRes.GetData()
	result.Merged = &ma
	if !ma.Success {
		return result, fmt.Errorf("merge failed")
	}

	// Step 5: Verify build (run test and lint commands)
	bvRes := protocol.VerifyBuild(opts.ManifestPath, opts.RepoPath)
	if !bvRes.IsSuccess() {
		return result, fmt.Errorf("verify build: %v", bvRes.Errors)
	}
	bv := bvRes.GetData()
	result.BuildVerified = &bv

	// Step 6: Cleanup worktrees and branches
	cl, err := protocol.Cleanup(opts.ManifestPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("cleanup: %w", err)
	}
	clData := cl.GetData()
	result.Cleaned = &clData

	// Success is true only if both test and lint passed
	result.Success = bv.TestPassed && bv.LintPassed
	return result, nil
}
