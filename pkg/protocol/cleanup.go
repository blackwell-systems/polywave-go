package protocol

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// CleanupAllStale removes all stale worktrees across all slugs.
func CleanupAllStale(repoDir string, force bool) (*StaleCleanupData, error) {
	stale, err := DetectStaleWorktrees(repoDir)
	if err != nil {
		return nil, fmt.Errorf("detect stale worktrees: %w", err)
	}
	if len(stale) == 0 {
		return &StaleCleanupData{
			Cleaned: []StaleWorktree{},
			Skipped: []StaleWorktree{},
			Errors:  []StaleCleanupFailure{},
		}, nil
	}
	res := CleanStaleWorktrees(stale, force)
	if !res.IsSuccess() {
		errMsg := "clean stale worktrees failed"
		if len(res.Errors) > 0 {
			errMsg = res.Errors[0].Message
		}
		// Return partial data if available
		if res.Data != nil {
			return *res.Data, fmt.Errorf("%s", errMsg)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	data := res.GetData()
	return data, nil
}

// CleanupStatus represents the cleanup result for a single agent.
// It tracks whether the worktree and branch were successfully removed.
type CleanupStatus struct {
	Agent           string `json:"agent"`
	WorktreeRemoved bool   `json:"worktree_removed"`
	BranchDeleted   bool   `json:"branch_deleted"`
}

// CleanupData contains the cleanup outcome for a wave.
// It contains the cleanup status for each agent in the wave.
type CleanupData struct {
	Wave   int             `json:"wave"`
	Agents []CleanupStatus `json:"agents"`
}

// Cleanup removes worktrees and branches for all agents in the specified wave.
// It loads the manifest from manifestPath, finds the wave by waveNum, and attempts
// to remove each agent's worktree and branch. Cleanup is best-effort and idempotent:
// if a worktree or branch is already gone, it's treated as success. Individual agent
// cleanup failures do not stop the overall cleanup process.
//
// Returns result.Result[CleanupData] with status for each agent, or an error if the
// wave is not found.
func Cleanup(manifestPath string, waveNum int, repoDir string) (result.Result[CleanupData], error) {
	// Load manifest
	manifest, err := Load(manifestPath)
	if err != nil {
		return result.NewFailure[CleanupData]([]result.StructuredError{
			{
				Code:     "E001",
				Message:  fmt.Sprintf("failed to load manifest: %v", err),
				Severity: "fatal",
			},
		}), fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find the specified wave
	var targetWave *Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			targetWave = &manifest.Waves[i]
			break
		}
	}

	if targetWave == nil {
		return result.NewFailure[CleanupData]([]result.StructuredError{
			{
				Code:     "E002",
				Message:  fmt.Sprintf("wave %d not found in manifest", waveNum),
				Severity: "fatal",
			},
		}), fmt.Errorf("wave %d not found in manifest", waveNum)
	}

	data := CleanupData{
		Wave:   waveNum,
		Agents: make([]CleanupStatus, 0, len(targetWave.Agents)),
	}

	// Resolve repoDir to absolute path for cross-repo resolution.
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return result.NewFailure[CleanupData]([]result.StructuredError{
			{
				Code:     "E003",
				Message:  fmt.Sprintf("failed to resolve repo dir: %v", err),
				Severity: "fatal",
			},
		}), fmt.Errorf("failed to resolve repo dir: %w", err)
	}
	repoParent := filepath.Dir(absRepoDir)

	// Track unique repos for pruning at the end.
	prunedRepos := map[string]bool{absRepoDir: true}

	// Clean up each agent in the wave
	for _, agent := range targetWave.Agents {
		status := CleanupStatus{
			Agent:           agent.ID,
			WorktreeRemoved: false,
			BranchDeleted:   false,
		}

		// Determine agent's repo (cross-repo support).
		agentRepo := AgentRepoName(manifest.FileOwnership, agent.ID)
		agentRepoDir := absRepoDir
		if agentRepo != "" && agentRepo != filepath.Base(absRepoDir) {
			agentRepoDir = filepath.Join(repoParent, agentRepo)
		}
		prunedRepos[agentRepoDir] = true

		// Try new slug-scoped format first, then legacy format as fallback.
		branchName := BranchName(manifest.FeatureSlug, waveNum, agent.ID)
		legacyBranch := LegacyBranchName(waveNum, agent.ID)

		// Resolve worktree paths for both formats
		worktreePath := ResolveWorktreePathWithSlug(agentRepoDir, manifest.FeatureSlug, waveNum, agent.ID)

		// Attempt to remove worktree (tries slug-scoped path which may resolve to legacy)
		rmErr := git.WorktreeRemove(agentRepoDir, worktreePath)
		if rmErr != nil {
			errMsg := rmErr.Error()
			if strings.Contains(errMsg, "is not a working tree") ||
				strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "no such file") {
				status.WorktreeRemoved = true
			}
		} else {
			status.WorktreeRemoved = true
		}

		// Attempt to delete branch: try new format first, then legacy
		delErr := git.DeleteBranch(agentRepoDir, branchName)
		if delErr != nil {
			errMsg := delErr.Error()
			if strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "branch") && strings.Contains(errMsg, "not found") {
				// New-format branch not found — try legacy
				err2 := git.DeleteBranch(agentRepoDir, legacyBranch)
				if err2 != nil {
					errMsg2 := err2.Error()
					if strings.Contains(errMsg2, "not found") ||
						strings.Contains(errMsg2, "branch") && strings.Contains(errMsg2, "not found") {
						status.BranchDeleted = true
					}
				} else {
					status.BranchDeleted = true
				}
			}
		} else {
			status.BranchDeleted = true
		}

		data.Agents = append(data.Agents, status)
	}

	// Best-effort: prune stale worktree entries from all involved repos.
	for repo := range prunedRepos {
		if err := git.WorktreePrune(repo); err != nil {
			log.Printf("warning: git worktree prune failed for %s (non-fatal): %v", repo, err)
		}
	}

	// Remove empty slug parent directory if all agent worktrees were cleaned.
	// Worktree paths are: .claude/worktrees/saw/{slug}/wave{N}-agent-{ID}
	// After removing all agent dirs, the slug dir is left empty.
	for repo := range prunedRepos {
		slugDir := filepath.Join(repo, ".claude", "worktrees", "saw", manifest.FeatureSlug)
		entries, err := os.ReadDir(slugDir)
		if err == nil && len(entries) == 0 {
			_ = os.Remove(slugDir)
		}
	}

	return result.NewSuccess(data), nil
}
