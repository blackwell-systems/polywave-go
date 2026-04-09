package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// RunWaveFullOpts configures a full wave lifecycle execution.
type RunWaveFullOpts struct {
	ManifestPath string       // path to IMPL manifest file
	RepoPath     string       // absolute path to the repository
	WaveNum      int          // wave number to execute
	MergeTarget  string       // target branch for merge; empty = HEAD (default)
	Logger       *slog.Logger // optional: nil falls back to slog.Default()
}

// RunWaveFullResult captures the results of all wave lifecycle steps.
type RunWaveFullResult struct {
	Wave             int                           `json:"wave"`
	WorktreesCreated *protocol.CreateWorktreesData `json:"worktrees_created"`
	FinalizeResult   *FinalizeWaveResult           `json:"finalize_result,omitempty"`
	Success          bool                          `json:"success"`
}

// RunWaveFull orchestrates a complete wave lifecycle: worktree creation,
// agent execution (external), commit verification, merge, build verification,
// and cleanup.
//
// This function handles the pre-agent and post-agent orchestration steps.
// The caller is responsible for launching agents between worktree creation
// and commit verification.
//
// Steps 3-6 (verify commits, merge, verify build, cleanup) are delegated to
// engine.FinalizeWave(), eliminating a third copy of the finalization sequence.
//
// Returns a Result[RunWaveFullResult] with detailed status for each step.
// Success is true only if FinalizeWave reports success (both test and lint
// commands pass during build verification). Partial results are returned
// when finalize fails but worktree creation succeeded.
func RunWaveFull(ctx context.Context, opts RunWaveFullOpts) result.Result[RunWaveFullResult] {
	res := RunWaveFullResult{Wave: opts.WaveNum}

	// Step 1: Create worktrees for all agents in the wave
	wtRes := protocol.CreateWorktrees(ctx, opts.ManifestPath, opts.WaveNum, opts.RepoPath, opts.Logger)
	if !wtRes.IsSuccess() {
		return result.NewFailure[RunWaveFullResult]([]result.SAWError{
			result.NewFatal(result.CodeWaveFailed, fmt.Sprintf("create worktrees: %v", wtRes.Errors)),
		})
	}
	wt := wtRes.GetData()
	res.WorktreesCreated = &wt

	// Step 2: Agent execution happens externally (orchestrator launches agents)
	// This function handles pre/post agent work only.
	// The caller is responsible for launching agents between CreateWorktrees and VerifyCommits.

	// Steps 3-6: Delegate to engine.FinalizeWave() which handles:
	//   3. VerifyCommits (I5)
	//   3.5. ScanStubs (E20)
	//   3.75. RunGates (E21)
	//   3.9. ValidateIntegration (E25)
	//   4. MergeAgents
	//   4.5. FixGoMod
	//   5. VerifyBuild
	//   6. Cleanup
	finalizeOpts := FinalizeWaveOpts{
		IMPLPath:    opts.ManifestPath,
		RepoPath:    opts.RepoPath,
		WaveNum:     opts.WaveNum,
		MergeTarget: opts.MergeTarget,
		Logger:      opts.Logger,
	}
	finalizeRes := FinalizeWave(ctx, finalizeOpts)
	if finalizeRes.Data != nil {
		fd := finalizeRes.GetData()
		res.FinalizeResult = &fd
	}
	if !finalizeRes.IsSuccess() {
		// Return partial result: worktrees created but finalize failed
		errs := finalizeRes.Errors
		if len(errs) == 0 {
			errs = []result.SAWError{result.NewFatal(result.CodeWaveFailed, "finalize wave failed")}
		}
		return result.NewPartial(res, errs)
	}
	finalizeData := finalizeRes.GetData()
	res.Success = finalizeData.Success
	return result.NewSuccess(res)
}
