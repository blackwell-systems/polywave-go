package protocol

import (
	"fmt"
	"path/filepath"
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
	CrossRepo   bool   `json:"cross_repo,omitempty"` // true when verified via completion report (cross-repo agent)
}

// VerifyCommitsResult represents the outcome of verifying that all agents
// in a wave have committed their work.
type VerifyCommitsResult struct {
	BaseCommit string         `json:"base_commit"`
	Agents     []CommitStatus `json:"agents"`
	AllValid   bool           `json:"all_valid"`
}

// VerifyCommits checks that all agents in the specified wave have committed
// their work to their respective branches. Automatically detects multi-repo
// waves by reading the file ownership table and completion reports.
//
// The base commit is determined from HEAD of each repository. Each agent's
// branch is expected to follow the pattern "wave{N}-agent-{ID}". If a branch
// does not exist or has no commits relative to the base, it is recorded with
// HasCommits=false but does not cause an error - the AllValid flag will be false.
//
// Returns an error only for system-level failures (e.g., cannot determine base commit,
// cannot load manifest, wave not found). Missing or empty branches are recorded
// in the result but do not cause errors.
func VerifyCommits(manifestPath string, waveNum int, repoDir string) (*VerifyCommitsResult, error) {
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

	// Resolve repoDir to absolute path; repo names in fo.Repo are resolved as
	// siblings of this directory (same pattern as worktree.go line 116).
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repo dir: %w", err)
	}
	repoParent := filepath.Dir(absRepoDir)

	// Group agents by repository using file ownership table
	agentRepos := make(map[string]string) // agent ID -> absolute repo path
	for _, fo := range manifest.FileOwnership {
		if fo.Wave == waveNum {
			if fo.Repo != "" {
				// fo.Repo is a repo name (e.g. "scout-and-wave-go"), not a path.
				// Resolve it as a sibling of the provided repoDir.
				agentRepos[fo.Agent] = filepath.Join(repoParent, fo.Repo)
			} else {
				agentRepos[fo.Agent] = absRepoDir
			}
		}
	}

	// Fallback: if agent not in file ownership table, use completion report repo field
	for _, agent := range targetWave.Agents {
		if _, found := agentRepos[agent.ID]; !found {
			if report, ok := manifest.CompletionReports[agent.ID]; ok && report.Repo != "" {
				agentRepos[agent.ID] = filepath.Join(repoParent, report.Repo)
			} else {
				agentRepos[agent.ID] = absRepoDir
			}
		}
	}

	// Get base commit - use wave's recorded base commit if available (prevention fix),
	// otherwise fall back to current HEAD for backward compatibility
	baseCommit := targetWave.BaseCommit
	if baseCommit == "" {
		// Backward compatibility: wave created before base commit tracking
		var err error
		baseCommit, err = git.RevParse(absRepoDir, "HEAD")
		if err != nil {
			return nil, fmt.Errorf("failed to get base commit: %w", err)
		}
	}

	// Build the result
	result := &VerifyCommitsResult{
		BaseCommit: baseCommit,
		Agents:     make([]CommitStatus, 0, len(targetWave.Agents)),
		AllValid:   true,
	}

	// Check each agent's branch in its respective repository
	for _, agent := range targetWave.Agents {
		branchName := BranchName(manifest.FeatureSlug, waveNum, agent.ID)
		legacyBranch := LegacyBranchName(waveNum, agent.ID)

		// Determine which repo this agent worked in (already an absolute path)
		agentRepoDir := agentRepos[agent.ID]

		status := CommitStatus{
			Agent:  agent.ID,
			Branch: branchName,
		}

		// Count commits on the branch relative to recorded base commit.
		// Use the wave's base commit (recorded at worktree creation) rather than
		// current HEAD, so verification works even if branches were already merged.
		// Try slug-scoped branch first, then fall back to legacy branch name.
		activeBranch := branchName
		revListArg := baseCommit + ".." + branchName
		output, err := git.Run(agentRepoDir, "rev-list", "--count", revListArg)

		if err != nil {
			// The base commit may not exist in this repo (cross-repo wave: base commit
			// was recorded from a different repo). Fall back to HEAD..branch in the
			// agent's own repo, which counts commits not yet merged to the local HEAD.
			output, err = git.Run(agentRepoDir, "rev-list", "--count", "HEAD.."+branchName)
		}

		if err != nil {
			// Try legacy branch name for backward compatibility
			activeBranch = legacyBranch
			revListArg = baseCommit + ".." + legacyBranch
			output, err = git.Run(agentRepoDir, "rev-list", "--count", revListArg)
			if err != nil {
				output, err = git.Run(agentRepoDir, "rev-list", "--count", "HEAD.."+legacyBranch)
			}
		}

		if err == nil {
			status.Branch = activeBranch
		}

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

		// Fallback: if no branch commits found, check if completion report has a commit SHA.
		// Cross-repo agents (e.g. docs agents committing to a different repo's default branch)
		// won't have a wave branch — their completion report commit is the I5 proof.
		if !status.HasCommits {
			if report, ok := manifest.CompletionReports[agent.ID]; ok && report.Commit != "" {
				status.HasCommits = true
				status.CommitCount = 1
				status.CrossRepo = true
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
