package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func init() {
	mergeWaveFunc = executeMergeWave
}

// executeMergeWave implements the full Polywave merge procedure for waveNum.
// Called by Orchestrator.MergeWave via the mergeWaveFunc variable (set in init()).
func executeMergeWave(ctx context.Context, o *Orchestrator, waveNum int) result.Result[MergeData] {
	// Step 1: Find wave in IMPL doc.
	wave := o.IMPLDoc().FindWave(waveNum)
	if wave == nil {
		return result.NewFailure[MergeData]([]result.PolywaveError{
			result.NewFatal(result.CodeWaveNotReady,
				fmt.Sprintf("executeMergeWave: wave %d not found in IMPL doc", waveNum)),
		})
	}

	// Step 2: Load manifest and check completion reports; abort if any agent is partial or blocked.
	manifest, err := protocol.Load(ctx, o.implDocPath)
	if err != nil {
		return result.NewFailure[MergeData]([]result.PolywaveError{
			result.NewFatal(result.CodeIMPLParseFailed,
				fmt.Sprintf("executeMergeWave: loading manifest: %s", err.Error())),
		})
	}

	reports := make(map[string]*protocol.CompletionReport, len(wave.Agents))
	for _, agent := range wave.Agents {
		protoReport, ok := manifest.CompletionReports[agent.ID]
		if !ok {
			return result.NewFailure[MergeData]([]result.PolywaveError{
				result.NewFatal(result.CodeCompletionReportMissing,
					fmt.Sprintf("executeMergeWave: no completion report for agent %s", agent.ID)),
			})
		}

		report := &protoReport

		if report.Status == protocol.StatusPartial || report.Status == protocol.StatusBlocked {
			return result.NewFailure[MergeData]([]result.PolywaveError{
				result.NewFatal(result.CodeInvalidMergeState, fmt.Sprintf(
					"executeMergeWave: agent %s has status %q — merge aborted", agent.ID, report.Status)),
			})
		}
		reports[agent.ID] = report
	}

	// Step 3: Record base commit before any merges.
	baseCommit, err := git.RevParse(o.repoPath, "HEAD")
	if err != nil {
		return result.NewFailure[MergeData]([]result.PolywaveError{
			result.NewFatal(result.CodeCommitMissing,
				fmt.Sprintf("executeMergeWave: resolving HEAD: %s", err.Error())),
		})
	}

	// Build per-agent repo map for cross-repo support.
	absRepoDir, err := filepath.Abs(o.repoPath)
	if err != nil {
		return result.NewFailure[MergeData]([]result.PolywaveError{
			result.NewFatal("MERGE_PATH_ERROR", fmt.Sprintf("executeMergeWave: resolving repo path: %v", err)),
		})
	}
	repoParent := filepath.Dir(absRepoDir)
	agentRepoDir := make(map[string]string) // agent ID -> absolute repo path
	for _, fo := range manifest.FileOwnership {
		if fo.Wave == waveNum && fo.Repo != "" && fo.Repo != filepath.Base(absRepoDir) {
			agentRepoDir[fo.Agent] = filepath.Join(repoParent, fo.Repo)
		}
	}

	// Step 4: Load merge-log for idempotency (E9) — must happen BEFORE
	// verifyAgentCommits so we can skip agents whose branches were already
	// merged and deleted.
	mergeLog, err := protocol.LoadMergeLog(o.implDocPath, waveNum)
	if err != nil {
		return result.NewFailure[MergeData]([]result.PolywaveError{
			result.NewFatal(result.CodeIMPLParseFailed,
				fmt.Sprintf("executeMergeWave: loading merge-log: %s", err.Error())),
		})
	}

	// Filter out already-merged agents before verification.
	pendingReports := make(map[string]*protocol.CompletionReport, len(reports))
	for letter, report := range reports {
		if !mergeLog.IsMerged(letter) {
			pendingReports[letter] = report
		} else {
			o.log().Debug("executeMergeWave: agent already merged (skipping verification)", "agent", letter)
		}
	}

	// If all agents already merged, we're done (full idempotency).
	if len(pendingReports) == 0 {
		o.log().Debug("executeMergeWave: all agents already merged", "wave", waveNum)
		return result.NewSuccess(MergeData{WaveNum: waveNum})
	}

	// Step 5: Verify each pending agent has commits beyond base.
	if res := verifyAgentCommits(o.repoPath, baseCommit, pendingReports, agentRepoDir); res.IsFatal() {
		return result.NewFailure[MergeData](res.Errors)
	}

	// Step 6: Check for file conflicts across pending agents.
	if res := predictConflicts(pendingReports); res.IsFatal() {
		return result.NewFailure[MergeData](res.Errors)
	}

	// Step 7 & 8: Merge each complete agent; clean up worktree afterward.
	// Cross-repo agents are merged in their own repos.
	for _, agent := range wave.Agents {
		report, ok := reports[agent.ID]
		if !ok || report.Status != protocol.StatusComplete {
			continue
		}

		// Check if agent already merged (idempotency)
		if mergeLog.IsMerged(agent.ID) {
			o.log().Debug("executeMergeWave: agent already merged (skipping)", "agent", agent.ID)
			continue
		}

		branch := protocol.BranchName(manifest.FeatureSlug, waveNum, agent.ID)

		// Resolve the repo this agent works in (cross-repo support).
		mergeRepo := o.repoPath
		if r, ok := agentRepoDir[agent.ID]; ok {
			mergeRepo = r
		}

		// Skip merge for no-op agents (no file changes — nothing to merge).
		if len(report.FilesChanged) == 0 && len(report.FilesCreated) == 0 {
			o.log().Debug("executeMergeWave: agent produced no changes (skipping merge)", "agent", agent.ID)
			mergeLog.AddMergeEntry(agent.ID, "no-op")
			if saveRes := protocol.SaveMergeLog(o.implDocPath, waveNum, mergeLog); saveRes.IsFatal() {
				o.log().Warn("executeMergeWave: failed to save merge-log", "err", saveRes.Errors[0].Message)
			}
			// Still clean up the worktree.
			wtPath := report.Worktree
			if wtPath == "" {
				wtPath = protocol.ResolveWorktreePath(mergeRepo, branch)
			}
			if !strings.HasPrefix(wtPath, "/") {
				wtPath = mergeRepo + "/" + wtPath
			}
			if err := git.WorktreeRemove(mergeRepo, wtPath); err != nil {
				o.log().Warn("executeMergeWave: could not remove worktree", "path", wtPath, "err", err)
			}
			if err := git.DeleteBranch(mergeRepo, branch); err != nil {
				o.log().Warn("executeMergeWave: could not delete branch", "branch", branch, "err", err)
			}
			continue
		}

		mergeMsg := fmt.Sprintf("Merge %s: %s", branch, agent.ID)

		if err := git.MergeNoFF(mergeRepo, branch, mergeMsg); err != nil {
			return result.NewFailure[MergeData]([]result.PolywaveError{
				result.NewFatal(result.CodeMergeConflict, fmt.Sprintf(
					"executeMergeWave: merging %s in %s: %s", branch, mergeRepo, err.Error())),
			})
		}

		// Get merge commit SHA
		mergeSHA, err := git.RevParse(mergeRepo, "HEAD")
		if err != nil {
			return result.NewFailure[MergeData]([]result.PolywaveError{
				result.NewFatal(result.CodeCommitMissing, fmt.Sprintf(
					"executeMergeWave: getting merge SHA for %s: %s", agent.ID, err.Error())),
			})
		}

		// Record merge in log (E9)
		mergeLog.AddMergeEntry(agent.ID, mergeSHA)
		if saveRes := protocol.SaveMergeLog(o.implDocPath, waveNum, mergeLog); saveRes.IsFatal() {
			o.log().Warn("executeMergeWave: failed to save merge-log", "err", saveRes.Errors[0].Message)
		}

		// Determine worktree path from report or convention.
		wtPath := report.Worktree
		if wtPath == "" {
			wtPath = protocol.ResolveWorktreePath(mergeRepo, branch)
		}
		if !strings.HasPrefix(wtPath, "/") {
			wtPath = mergeRepo + "/" + wtPath
		}

		if err := git.WorktreeRemove(mergeRepo, wtPath); err != nil {
			o.log().Warn("executeMergeWave: could not remove worktree", "path", wtPath, "err", err)
		}
		if err := git.DeleteBranch(mergeRepo, branch); err != nil {
			o.log().Warn("executeMergeWave: could not delete branch", "branch", branch, "err", err)
		}

	}

	return result.NewSuccess(MergeData{WaveNum: waveNum})
}

// predictConflicts cross-references files_changed and files_created from all
// completion reports. Returns failure if any file appears in >1 agent's lists.
// Excludes files matching "docs/IMPL/" prefix.
func predictConflicts(reports map[string]*protocol.CompletionReport) result.Result[ConflictData] {
	// map of filename -> first agent letter that claimed it
	seen := make(map[string]string)
	filesChecked := 0

	for letter, report := range reports {
		all := append(report.FilesChanged, report.FilesCreated...)
		for _, f := range all {
			if strings.HasPrefix(f, "docs/IMPL/") {
				continue
			}
			filesChecked++
			if prev, exists := seen[f]; exists {
				if prev != letter {
					return result.NewFailure[ConflictData]([]result.PolywaveError{
						result.NewFatal(result.CodeMergeConflict, fmt.Sprintf(
							"predictConflicts: file %q claimed by both agent %s and agent %s", f, prev, letter)),
					})
				}
			} else {
				seen[f] = letter
			}
		}
	}

	return result.NewSuccess(ConflictData{FilesChecked: filesChecked})
}

// verifyAgentCommits checks each agent with status:complete has at least 1 commit
// on its branch beyond baseCommit. agentRepoDir maps agent IDs to their repo
// paths for cross-repo waves; agents not in the map use repoPath.
func verifyAgentCommits(repoPath, baseCommit string, reports map[string]*protocol.CompletionReport, agentRepoDir map[string]string) result.Result[VerifyCommitsData] {
	agentsVerified := 0
	for letter, report := range reports {
		if report.Status != protocol.StatusComplete {
			continue
		}

		branch := report.Branch
		if branch == "" {
			return result.NewFailure[VerifyCommitsData]([]result.PolywaveError{
				result.NewFatal(result.CodeCommitMissing, fmt.Sprintf(
					"verifyAgentCommits: agent %s report has empty branch field", letter)),
			})
		}

		// Skip diff check for no-op agents (auto-committed with no changes).
		if len(report.FilesChanged) == 0 && len(report.FilesCreated) == 0 {
			continue
		}

		// Resolve the repo this agent's branch lives in.
		checkRepo := repoPath
		if r, ok := agentRepoDir[letter]; ok {
			checkRepo = r
		}

		// If branch no longer exists, check if the agent's commit was already
		// merged into HEAD (idempotent retry after cleanup deleted the branch).
		if !git.BranchExists(checkRepo, branch) {
			if report.Commit != "" && git.IsAncestor(checkRepo, report.Commit, "HEAD") {
				agentsVerified++
				continue // already merged
			}
			return result.NewFailure[VerifyCommitsData]([]result.PolywaveError{
				result.NewFatal(result.CodeCommitMissing, fmt.Sprintf(
					"verifyAgentCommits: agent %s branch %q does not exist in %s and commit %q is not merged into HEAD",
					letter, branch, checkRepo, report.Commit)),
			})
		}

		// Use HEAD..branch in the agent's own repo for cross-repo agents
		// (baseCommit may be from a different repo).
		diffBase := baseCommit
		if checkRepo != repoPath {
			diffBase = "HEAD"
		}

		files, err := git.DiffNameOnly(checkRepo, diffBase, branch)
		if err != nil {
			return result.NewFailure[VerifyCommitsData]([]result.PolywaveError{
				result.NewFatal(result.CodeCommitMissing, fmt.Sprintf(
					"verifyAgentCommits: diffing agent %s branch %q: %s", letter, branch, err.Error())),
			})
		}
		if len(files) == 0 {
			return result.NewFailure[VerifyCommitsData]([]result.PolywaveError{
				result.NewFatal(result.CodeIsolationVerifyFailed, fmt.Sprintf(
					"verifyAgentCommits: ISOLATION FAILURE — agent %s branch %q has no commits beyond %s",
					letter, branch, diffBase)),
			})
		}
		agentsVerified++
	}

	return result.NewSuccess(VerifyCommitsData{AgentsVerified: agentsVerified})
}
