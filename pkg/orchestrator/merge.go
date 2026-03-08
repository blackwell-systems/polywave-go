package orchestrator

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

func init() {
	mergeWaveFunc = executeMergeWave
}

// executeMergeWave implements the full SAW merge procedure for waveNum.
// Called by Orchestrator.MergeWave via the mergeWaveFunc variable (set in init()).
func executeMergeWave(o *Orchestrator, waveNum int) error {
	// Step 1: Find wave in IMPL doc.
	var wave *types.Wave
	for i := range o.IMPLDoc().Waves {
		if o.IMPLDoc().Waves[i].Number == waveNum {
			wave = &o.IMPLDoc().Waves[i]
			break
		}
	}
	if wave == nil {
		return fmt.Errorf("executeMergeWave: wave %d not found in IMPL doc", waveNum)
	}

	// Step 2: Parse completion reports; abort if any agent is partial or blocked.
	reports := make(map[string]*types.CompletionReport, len(wave.Agents))
	for _, agent := range wave.Agents {
		report, err := protocol.ParseCompletionReport(o.implDocPath, agent.Letter)
		if err != nil {
			return fmt.Errorf("executeMergeWave: parsing completion report for agent %s: %w", agent.Letter, err)
		}
		if report.Status == types.StatusPartial || report.Status == types.StatusBlocked {
			return fmt.Errorf("executeMergeWave: agent %s has status %q — merge aborted", agent.Letter, report.Status)
		}
		reports[agent.Letter] = report
	}

	// Step 3: Record base commit before any merges.
	baseCommit, err := git.RevParse(o.repoPath, "HEAD")
	if err != nil {
		return fmt.Errorf("executeMergeWave: resolving HEAD: %w", err)
	}

	// Step 4: Verify each agent has commits beyond base.
	if err := verifyAgentCommits(o.repoPath, baseCommit, reports); err != nil {
		return err
	}

	// Step 5: Check for file conflicts across agents.
	if err := predictConflicts(reports); err != nil {
		return err
	}

	// Step 6 & 7: Merge each complete agent; clean up worktree afterward.
	for _, agent := range wave.Agents {
		report, ok := reports[agent.Letter]
		if !ok || report.Status != types.StatusComplete {
			continue
		}

		branch := fmt.Sprintf("wave%d-agent-%s", waveNum, agent.Letter)
		mergeMsg := fmt.Sprintf("Merge wave%d-agent-%s: %s", waveNum, agent.Letter, branch)

		if err := git.MergeNoFF(o.repoPath, branch, mergeMsg); err != nil {
			return fmt.Errorf("executeMergeWave: merging %s: %w", branch, err)
		}

		// Determine worktree path from report or convention.
		wtPath := report.Worktree
		if wtPath == "" {
			wtPath = fmt.Sprintf(".claude/worktrees/%s", branch)
		}
		// Make absolute if relative.
		if !strings.HasPrefix(wtPath, "/") {
			wtPath = o.repoPath + "/" + wtPath
		}

		if err := git.WorktreeRemove(o.repoPath, wtPath); err != nil {
			fmt.Fprintf(os.Stderr, "executeMergeWave: warning: could not remove worktree %q: %v\n", wtPath, err)
		}
		if err := git.DeleteBranch(o.repoPath, branch); err != nil {
			fmt.Fprintf(os.Stderr, "executeMergeWave: warning: could not delete branch %q: %v\n", branch, err)
		}
	}

	return nil
}

// predictConflicts cross-references files_changed and files_created from all
// completion reports. Returns error if any file appears in >1 agent's lists.
// Excludes files matching "docs/IMPL/" prefix.
func predictConflicts(reports map[string]*types.CompletionReport) error {
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
// on its branch beyond baseCommit.
func verifyAgentCommits(repoPath, baseCommit string, reports map[string]*types.CompletionReport) error {
	for letter, report := range reports {
		if report.Status != types.StatusComplete {
			continue
		}

		branch := report.Branch
		if branch == "" {
			// Derive branch from agent letter if not set in report.
			// We don't know the wave number here, but branch should be set.
			return fmt.Errorf("verifyAgentCommits: agent %s report has empty branch field", letter)
		}

		files, err := git.DiffNameOnly(repoPath, baseCommit, branch)
		if err != nil {
			return fmt.Errorf("verifyAgentCommits: diffing agent %s branch %q: %w", letter, branch, err)
		}
		if len(files) == 0 {
			return fmt.Errorf("verifyAgentCommits: ISOLATION FAILURE — agent %s branch %q has no commits beyond %s", letter, branch, baseCommit)
		}
	}

	return nil
}
