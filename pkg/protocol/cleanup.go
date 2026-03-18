package protocol

import (
	"fmt"
	"log"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// CleanupStatus represents the cleanup result for a single agent.
// It tracks whether the worktree and branch were successfully removed.
type CleanupStatus struct {
	Agent           string `json:"agent"`
	WorktreeRemoved bool   `json:"worktree_removed"`
	BranchDeleted   bool   `json:"branch_deleted"`
}

// CleanupResult represents the overall cleanup result for a wave.
// It contains the cleanup status for each agent in the wave.
type CleanupResult struct {
	Wave   int             `json:"wave"`
	Agents []CleanupStatus `json:"agents"`
}

// Cleanup removes worktrees and branches for all agents in the specified wave.
// It loads the manifest from manifestPath, finds the wave by waveNum, and attempts
// to remove each agent's worktree and branch. Cleanup is best-effort and idempotent:
// if a worktree or branch is already gone, it's treated as success. Individual agent
// cleanup failures do not stop the overall cleanup process.
//
// Returns CleanupResult with status for each agent, or an error if the wave is not found.
func Cleanup(manifestPath string, waveNum int, repoDir string) (*CleanupResult, error) {
	// Load manifest
	manifest, err := Load(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
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
		return nil, fmt.Errorf("wave %d not found in manifest", waveNum)
	}

	result := &CleanupResult{
		Wave:   waveNum,
		Agents: make([]CleanupStatus, 0, len(targetWave.Agents)),
	}

	// Clean up each agent in the wave
	for _, agent := range targetWave.Agents {
		status := CleanupStatus{
			Agent:           agent.ID,
			WorktreeRemoved: false,
			BranchDeleted:   false,
		}

		// Try new slug-scoped format first, then legacy format as fallback.
		branchName := BranchName(manifest.FeatureSlug, waveNum, agent.ID)
		legacyBranch := LegacyBranchName(waveNum, agent.ID)

		// Resolve worktree paths for both formats
		worktreePath := ResolveWorktreePathWithSlug(repoDir, manifest.FeatureSlug, waveNum, agent.ID)

		// Attempt to remove worktree (tries slug-scoped path which may resolve to legacy)
		err := git.WorktreeRemove(repoDir, worktreePath)
		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "is not a working tree") ||
				strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "no such file") {
				status.WorktreeRemoved = true
			}
		} else {
			status.WorktreeRemoved = true
		}

		// Attempt to delete branch: try new format first, then legacy
		err = git.DeleteBranch(repoDir, branchName)
		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "branch") && strings.Contains(errMsg, "not found") {
				// New-format branch not found — try legacy
				err2 := git.DeleteBranch(repoDir, legacyBranch)
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

		result.Agents = append(result.Agents, status)
	}

	// Best-effort: prune stale worktree entries from git metadata.
	// This removes references to worktrees whose directories were already deleted
	// but whose entries persist in .git/worktrees/, which can confuse LSP and other tools.
	if err := git.WorktreePrune(repoDir); err != nil {
		log.Printf("warning: git worktree prune failed (non-fatal): %v", err)
	}

	return result, nil
}
