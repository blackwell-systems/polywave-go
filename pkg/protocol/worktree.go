package protocol

import (
	"fmt"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
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
// It parses the IMPL doc from manifestPath, finds the wave by waveNum, and
// creates a worktree for each agent in that wave.
//
// Each worktree is created at {repoDir}/.claude/worktrees/wave{N}-agent-{Letter}
// on a new branch named wave{N}-agent-{Letter}.
//
// If any worktree creation fails, returns an error immediately without
// attempting to create remaining worktrees.
//
// Returns an error if:
// - The IMPL doc cannot be parsed
// - The specified wave number is not found in the document
// - Any git worktree add operation fails
func CreateWorktrees(manifestPath string, waveNum int, repoDir string) (*CreateWorktreesResult, error) {
	// Parse IMPL doc (supports hybrid markdown/YAML format)
	doc, err := ParseIMPLDoc(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IMPL doc: %w", err)
	}

	// Find the specified wave
	var targetWave *types.Wave
	for i := range doc.Waves {
		if doc.Waves[i].Number == waveNum {
			targetWave = &doc.Waves[i]
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
		worktreePath := filepath.Join(repoDir, ".claude", "worktrees", fmt.Sprintf("wave%d-agent-%s", waveNum, agent.Letter))
		branchName := fmt.Sprintf("wave%d-agent-%s", waveNum, agent.Letter)

		// Create the worktree
		if err := git.WorktreeAdd(repoDir, worktreePath, branchName); err != nil {
			return nil, fmt.Errorf("failed to create worktree for agent %s: %w", agent.Letter, err)
		}

		// Collect worktree info
		worktrees = append(worktrees, WorktreeInfo{
			Agent:  agent.Letter,
			Path:   worktreePath,
			Branch: branchName,
		})
	}

	return &CreateWorktreesResult{
		Worktrees: worktrees,
	}, nil
}
