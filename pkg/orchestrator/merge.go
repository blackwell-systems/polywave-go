package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func init() {
	mergeWaveFunc = executeMergeWave
}

// executeMergeWave implements the full SAW merge procedure for waveNum.
// Called by Orchestrator.MergeWave via the mergeWaveFunc variable (set in init()).
func executeMergeWave(o *Orchestrator, waveNum int) error {
	// Step 1: Find wave in IMPL doc.
	var wave *protocol.Wave
	for i := range o.IMPLDoc().Waves {
		if o.IMPLDoc().Waves[i].Number == waveNum {
			wave = &o.IMPLDoc().Waves[i]
			break
		}
	}
	if wave == nil {
		return fmt.Errorf("executeMergeWave: wave %d not found in IMPL doc", waveNum)
	}

	// Step 2: Load manifest and check completion reports; abort if any agent is partial or blocked.
	manifest, err := protocol.Load(o.implDocPath)
	if err != nil {
		return fmt.Errorf("executeMergeWave: loading manifest: %w", err)
	}

	reports := make(map[string]*protocol.CompletionReport, len(wave.Agents))
	for _, agent := range wave.Agents {
		protoReport, ok := manifest.CompletionReports[agent.ID]
		if !ok {
			return fmt.Errorf("executeMergeWave: no completion report for agent %s", agent.ID)
		}

		report := &protoReport

		if report.Status == "partial" || report.Status == "blocked" {
			return fmt.Errorf("executeMergeWave: agent %s has status %q — merge aborted", agent.ID, report.Status)
		}
		reports[agent.ID] = report
	}

	// Step 3: Record base commit before any merges.
	baseCommit, err := git.RevParse(o.repoPath, "HEAD")
	if err != nil {
		return fmt.Errorf("executeMergeWave: resolving HEAD: %w", err)
	}

	// Build per-agent repo map for cross-repo support.
	absRepoDir, _ := filepath.Abs(o.repoPath)
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
		return fmt.Errorf("executeMergeWave: loading merge-log: %w", err)
	}

	// Filter out already-merged agents before verification.
	pendingReports := make(map[string]*protocol.CompletionReport, len(reports))
	for letter, report := range reports {
		if !mergeLog.IsMerged(letter) {
			pendingReports[letter] = report
		} else {
			fmt.Fprintf(os.Stderr, "executeMergeWave: agent %s already merged (skipping verification)\n", letter)
		}
	}

	// If all agents already merged, we're done (full idempotency).
	if len(pendingReports) == 0 {
		fmt.Fprintf(os.Stderr, "executeMergeWave: all agents already merged for wave %d\n", waveNum)
		return nil
	}

	// Step 5: Verify each pending agent has commits beyond base.
	if err := verifyAgentCommits(o.repoPath, baseCommit, pendingReports, agentRepoDir); err != nil {
		return err
	}

	// Step 6: Check for file conflicts across pending agents.
	if err := predictConflicts(pendingReports); err != nil {
		return err
	}

	// Step 7 & 8: Merge each complete agent; clean up worktree afterward.
	// Cross-repo agents are merged in their own repos.
	for _, agent := range wave.Agents {
		report, ok := reports[agent.ID]
		if !ok || report.Status != "complete" {
			continue
		}

		// Check if agent already merged (idempotency)
		if mergeLog.IsMerged(agent.ID) {
			fmt.Fprintf(os.Stderr, "executeMergeWave: agent %s already merged (skipping)\n", agent.ID)
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
			fmt.Fprintf(os.Stderr, "executeMergeWave: agent %s produced no changes (skipping merge)\n", agent.ID)
			mergeLog.AddMergeEntry(agent.ID, "no-op")
			if saveErr := protocol.SaveMergeLog(o.implDocPath, waveNum, mergeLog); saveErr != nil {
				fmt.Fprintf(os.Stderr, "executeMergeWave: warning: failed to save merge-log: %v\n", saveErr)
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
				fmt.Fprintf(os.Stderr, "executeMergeWave: warning: could not remove worktree %q: %v\n", wtPath, err)
			}
			if err := git.DeleteBranch(mergeRepo, branch); err != nil {
				fmt.Fprintf(os.Stderr, "executeMergeWave: warning: could not delete branch %q: %v\n", branch, err)
			}
			continue
		}

		mergeMsg := fmt.Sprintf("Merge %s: %s", branch, agent.ID)

		if err := git.MergeNoFF(mergeRepo, branch, mergeMsg); err != nil {
			return fmt.Errorf("executeMergeWave: merging %s in %s: %w", branch, mergeRepo, err)
		}

		// Get merge commit SHA
		mergeSHA, err := git.RevParse(mergeRepo, "HEAD")
		if err != nil {
			return fmt.Errorf("executeMergeWave: getting merge SHA for %s: %w", agent.ID, err)
		}

		// Record merge in log (E9)
		mergeLog.AddMergeEntry(agent.ID, mergeSHA)
		if saveErr := protocol.SaveMergeLog(o.implDocPath, waveNum, mergeLog); saveErr != nil {
			fmt.Fprintf(os.Stderr, "executeMergeWave: warning: failed to save merge-log: %v\n", saveErr)
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
			fmt.Fprintf(os.Stderr, "executeMergeWave: warning: could not remove worktree %q: %v\n", wtPath, err)
		}
		if err := git.DeleteBranch(mergeRepo, branch); err != nil {
			fmt.Fprintf(os.Stderr, "executeMergeWave: warning: could not delete branch %q: %v\n", branch, err)
		}
	}

	return nil
}

// predictConflicts cross-references files_changed and files_created from all
// completion reports. Returns error if any file appears in >1 agent's lists.
// Excludes files matching "docs/IMPL/" prefix.
func predictConflicts(reports map[string]*protocol.CompletionReport) error {
	// map of filename -> first agent letter that claimed it
	seen := make(map[string]string)

	for letter, report := range reports {
		all := append(report.FilesChanged, report.FilesCreated...)
		for _, f := range all {
			if strings.HasPrefix(f, "docs/IMPL/") {
				continue
			}
			if prev, exists := seen[f]; exists {
				if prev != letter {
					return fmt.Errorf("predictConflicts: file %q claimed by both agent %s and agent %s", f, prev, letter)
				}
			} else {
				seen[f] = letter
			}
		}
	}

	return nil
}

// verifyAgentCommits checks each agent with status:complete has at least 1 commit
// on its branch beyond baseCommit. agentRepoDir maps agent IDs to their repo
// paths for cross-repo waves; agents not in the map use repoPath.
func verifyAgentCommits(repoPath, baseCommit string, reports map[string]*protocol.CompletionReport, agentRepoDir map[string]string) error {
	for letter, report := range reports {
		if report.Status != "complete" {
			continue
		}

		branch := report.Branch
		if branch == "" {
			return fmt.Errorf("verifyAgentCommits: agent %s report has empty branch field", letter)
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
				continue // already merged
			}
			return fmt.Errorf("verifyAgentCommits: agent %s branch %q does not exist in %s and commit %q is not merged into HEAD", letter, branch, checkRepo, report.Commit)
		}

		// Use HEAD..branch in the agent's own repo for cross-repo agents
		// (baseCommit may be from a different repo).
		diffBase := baseCommit
		if checkRepo != repoPath {
			diffBase = "HEAD"
		}

		files, err := git.DiffNameOnly(checkRepo, diffBase, branch)
		if err != nil {
			return fmt.Errorf("verifyAgentCommits: diffing agent %s branch %q: %w", letter, branch, err)
		}
		if len(files) == 0 {
			return fmt.Errorf("verifyAgentCommits: ISOLATION FAILURE — agent %s branch %q has no commits beyond %s", letter, branch, diffBase)
		}
	}

	return nil
}
