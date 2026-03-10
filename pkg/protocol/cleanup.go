package protocol

import (
	"fmt"
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

		// Construct worktree path: {repoDir}/.claude/worktrees/wave{N}-agent-{ID}
		worktreePath := fmt.Sprintf("%s/.claude/worktrees/wave%d-agent-%s", repoDir, waveNum, agent.ID)

		// Attempt to remove worktree
		err := git.WorktreeRemove(repoDir, worktreePath)
		if err != nil {
			// Check if the error indicates the worktree is already gone (idempotent)
			errMsg := err.Error()
			if strings.Contains(errMsg, "is not a working tree") ||
				strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "no such file") {
				// Already gone - treat as success
				status.WorktreeRemoved = true
			}
			// Otherwise, leave WorktreeRemoved as false and continue
		} else {
			status.WorktreeRemoved = true
		}

		// Construct branch name: wave{N}-agent-{ID}
		branchName := fmt.Sprintf("wave%d-agent-%s", waveNum, agent.ID)

		// Attempt to delete branch
		err = git.DeleteBranch(repoDir, branchName)
		if err != nil {
			// Check if the error indicates the branch is already gone (idempotent)
			errMsg := err.Error()
			if strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "branch") && strings.Contains(errMsg, "not found") {
				// Already gone - treat as success
				status.BranchDeleted = true
			}
			// Otherwise, leave BranchDeleted as false and continue
		} else {
			status.BranchDeleted = true
		}

		result.Agents = append(result.Agents, status)
	}

	return result, nil
}
