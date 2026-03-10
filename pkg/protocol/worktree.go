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
// It parses the IMPL doc from manifestPath, finds the wave by waveNum, and
// creates a worktree for each agent in that wave.
//
// For cross-repo waves, agents' files are looked up in the file ownership table
// to determine which repo each agent belongs to. If a Repo column is present,
// worktrees are created in sibling directories (e.g., if repoDir is
// /path/to/scout-and-wave and an agent has Repo=scout-and-wave-go, the worktree
// is created at /path/to/scout-and-wave-go/.claude/worktrees/...).
//
// Each worktree is created at {agentRepoDir}/.claude/worktrees/wave{N}-agent-{Letter}
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
	// Load IMPL doc (pure YAML format)
	doc, err := Load(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find the specified wave
	var targetWave *Wave
	for i := range doc.Waves {
		if doc.Waves[i].Number == waveNum {
			targetWave = &doc.Waves[i]
			break
		}
	}

	if targetWave == nil {
		return nil, fmt.Errorf("wave %d not found in manifest", waveNum)
	}

	// Determine parent directory for cross-repo resolution
	repoParent := filepath.Dir(repoDir)

	// Create worktrees for each agent
	var worktrees []WorktreeInfo
	for _, agent := range targetWave.Agents {
		// Determine agent's repo by looking up their files in FileOwnership
		agentRepo := determineAgentRepo(doc.FileOwnership, agent.ID)

		// Resolve repo directory (cross-repo or same-repo)
		var agentRepoDir string
		if agentRepo == "" {
			// No repo specified - use repoDir (single-repo case)
			agentRepoDir = repoDir
		} else {
			// Cross-repo: resolve as sibling directory
			agentRepoDir = filepath.Join(repoParent, agentRepo)
		}

		// Construct worktree path and branch name
		worktreePath := filepath.Join(agentRepoDir, ".claude", "worktrees", fmt.Sprintf("wave%d-agent-%s", waveNum, agent.ID))
		branchName := fmt.Sprintf("wave%d-agent-%s", waveNum, agent.ID)

		// Create the worktree
		if err := git.WorktreeAdd(agentRepoDir, worktreePath, branchName); err != nil {
			return nil, fmt.Errorf("failed to create worktree for agent %s in repo %s: %w", agent.ID, agentRepoDir, err)
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

// determineAgentRepo looks up the agent's owned files in the file ownership table
// and returns the Repo field from the first match. Returns empty string if no repo
// is specified (single-repo case).
func determineAgentRepo(fileOwnership []FileOwnership, agentID string) string {
	for _, fo := range fileOwnership {
		if fo.Agent == agentID && fo.Repo != "" {
			return fo.Repo
		}
	}
	return ""
}
