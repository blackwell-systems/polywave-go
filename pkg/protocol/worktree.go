package protocol

import (
	"fmt"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// WorktreeInfo contains the details of a created worktree for a single agent.
type WorktreeInfo struct {
	Agent  string `json:"agent"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

// CreateWorktreesResult contains the list of worktrees created for a wave.
type CreateWorktreesResult struct {
	Worktrees []WorktreeInfo `json:"worktrees"`
}

// CreateWorktrees creates git worktrees for all agents in the specified wave.
// It loads the manifest from manifestPath, finds the wave by waveNum, and
// creates a worktree for each agent in that wave.
//
// Each worktree is created at {repoDir}/.claude/worktrees/wave{N}-agent-{ID}
// on a new branch named wave{N}-agent-{ID}.
//
// If any worktree creation fails, returns an error immediately without
// attempting to create remaining worktrees.
//
// Returns an error if:
// - The manifest file cannot be loaded
// - The specified wave number is not found in the manifest
// - Any git worktree add operation fails
func CreateWorktrees(manifestPath string, waveNum int, repoDir string) (*CreateWorktreesResult, error) {
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

	// Create worktrees for each agent
	var worktrees []WorktreeInfo
	for _, agent := range targetWave.Agents {
		// Construct worktree path and branch name
		worktreePath := filepath.Join(repoDir, ".claude", "worktrees", fmt.Sprintf("wave%d-agent-%s", waveNum, agent.ID))
		branchName := fmt.Sprintf("wave%d-agent-%s", waveNum, agent.ID)

		// Create the worktree
		if err := git.WorktreeAdd(repoDir, worktreePath, branchName); err != nil {
			return nil, fmt.Errorf("failed to create worktree for agent %s: %w", agent.ID, err)
		}

		// Collect worktree info
		worktrees = append(worktrees, WorktreeInfo{
			Agent:  agent.ID,
			Path:   worktreePath,
			Branch: branchName,
		})
	}

	return &CreateWorktreesResult{
		Worktrees: worktrees,
	}, nil
}
