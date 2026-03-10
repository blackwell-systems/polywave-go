package protocol

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// CommitStatus represents the commit status of a single agent in a wave.
// It captures whether the agent has committed any work to its branch.
type CommitStatus struct {
	Agent       string `json:"agent"`
	Branch      string `json:"branch"`
	CommitCount int    `json:"commit_count"`
	HasCommits  bool   `json:"has_commits"`
}

// VerifyCommitsResult represents the outcome of verifying that all agents
// in a wave have committed their work.
type VerifyCommitsResult struct {
	BaseCommit string         `json:"base_commit"`
	Agents     []CommitStatus `json:"agents"`
	AllValid   bool           `json:"all_valid"`
}

// VerifyCommits checks that all agents in the specified wave have committed
// their work to their respective branches. It returns a detailed status for
// each agent and an overall validity flag.
//
// The base commit is determined from HEAD of the repository. Each agent's
// branch is expected to follow the pattern "wave{N}-agent-{ID}". If a branch
// does not exist or has no commits relative to the base, it is recorded with
// HasCommits=false but does not cause an error - the AllValid flag will be false.
//
// Returns an error only for system-level failures (e.g., cannot determine base commit,
// cannot load manifest, wave not found). Missing or empty branches are recorded
// in the result but do not cause errors.
func VerifyCommits(manifestPath string, waveNum int, repoDir string) (*VerifyCommitsResult, error) {
	// Get the base commit from HEAD
	baseCommit, err := git.RevParse(repoDir, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get base commit: %w", err)
	}

	// Load the manifest
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

	// Build the result
	result := &VerifyCommitsResult{
		BaseCommit: baseCommit,
		Agents:     make([]CommitStatus, 0, len(targetWave.Agents)),
		AllValid:   true,
	}

	// Check each agent's branch
	for _, agent := range targetWave.Agents {
		branchName := fmt.Sprintf("wave%d-agent-%s", waveNum, agent.ID)

		status := CommitStatus{
			Agent:  agent.ID,
			Branch: branchName,
		}

		// Count commits on the branch relative to base
		// Use rev-list to count commits between base and branch
		revListArg := baseCommit + ".." + branchName
		output, err := git.Run(repoDir, "rev-list", "--count", revListArg)

		if err != nil {
			// Branch doesn't exist or rev-list failed - treat as 0 commits
			status.CommitCount = 0
			status.HasCommits = false
		} else {
			// Parse the commit count from output
			countStr := strings.TrimSpace(output)
			count, parseErr := strconv.Atoi(countStr)
			if parseErr != nil {
				// Could not parse count - treat as 0 commits
				status.CommitCount = 0
				status.HasCommits = false
			} else {
				status.CommitCount = count
				status.HasCommits = count > 0
			}
		}

		// Update overall validity
		if !status.HasCommits {
			result.AllValid = false
		}

		result.Agents = append(result.Agents, status)
	}

	return result, nil
}
