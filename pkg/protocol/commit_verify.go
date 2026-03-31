package protocol

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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

// VerifyCommitsData represents the outcome of verifying that all agents
// in a wave have committed their work.
type VerifyCommitsData struct {
	BaseCommit string         `json:"base_commit"`
	Agents     []CommitStatus `json:"agents"`
}

// VerifyCommits checks that all agents in the specified wave have committed
// their work to their respective branches. Automatically detects multi-repo
// waves by reading the file ownership table and completion reports.
//
// The base commit is determined from HEAD of each repository. Each agent's
// branch is expected to follow the pattern "wave{N}-agent-{ID}". If a branch
// does not exist or has no commits relative to the base, it is recorded with
// HasCommits=false.
//
// The ctx parameter supports cancellation: the function checks ctx.Err()
// between per-agent git operations and returns early with a FATAL result if
// the context is cancelled or its deadline is exceeded.
//
// Returns result.Result[VerifyCommitsData]:
//   - SUCCESS (Code="SUCCESS"): All agents have commits
//   - PARTIAL (Code="PARTIAL"): Some agents missing commits (warnings in Errors)
//   - FATAL (Code="FATAL"): System-level failure (cannot load manifest, wave not found, etc.)
func VerifyCommits(ctx context.Context, manifestPath string, waveNum int, repoDir string) result.Result[VerifyCommitsData] {
	// Load the manifest
	manifest, err := Load(context.TODO(), manifestPath)
	if err != nil {
		return result.NewFailure[VerifyCommitsData]([]result.SAWError{
			{
				Code:     result.CodeIMPLParseFailed,
				Message:  fmt.Sprintf("failed to load manifest: %v", err),
				Severity: "fatal",
			},
		})
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
		return result.NewFailure[VerifyCommitsData]([]result.SAWError{
			{
				Code:     result.CodeWaveNotReady,
				Message:  fmt.Sprintf("wave %d not found in manifest", waveNum),
				Severity: "fatal",
			},
		})
	}

	// Resolve repoDir to absolute path; repo names in fo.Repo are resolved as
	// siblings of this directory (same pattern as worktree.go line 116).
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return result.NewFailure[VerifyCommitsData]([]result.SAWError{
			{
				Code:     result.CodeWorktreeCreateFailed,
				Message:  fmt.Sprintf("failed to resolve repo dir: %v", err),
				Severity: "fatal",
			},
		})
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
			return result.NewFailure[VerifyCommitsData]([]result.SAWError{
				{
					Code:     result.CodeCommitMissing,
					Message:  fmt.Sprintf("failed to get base commit: %v", err),
					Severity: "fatal",
				},
			})
		}
	}

	// Check context before starting per-agent iteration (may be slow for large waves).
	if err := ctx.Err(); err != nil {
		return result.NewFailure[VerifyCommitsData]([]result.SAWError{
			{
				Code:     result.CodeCommitMissing,
				Message:  fmt.Sprintf("context cancelled before verifying agents: %v", err),
				Severity: "fatal",
			},
		})
	}

	// Build the data
	data := VerifyCommitsData{
		BaseCommit: baseCommit,
		Agents:     make([]CommitStatus, 0, len(targetWave.Agents)),
	}

	// Track validation status
	allValid := true
	var warnings []result.SAWError

	// Check each agent's branch in its respective repository
	for _, agent := range targetWave.Agents {
		// Support cancellation between agents. Each iteration may invoke multiple
		// git subprocesses; checking here avoids starting new git work after cancel.
		if err := ctx.Err(); err != nil {
			return result.NewFailure[VerifyCommitsData]([]result.SAWError{
				{
					Code:     result.CodeCommitMissing,
					Message:  fmt.Sprintf("context cancelled while verifying agent %s: %v", agent.ID, err),
					Severity: "fatal",
				},
			})
		}
		// Determine which repo this agent worked in (already an absolute path)
		agentRepoDir := agentRepos[agent.ID]

		// Resolve the active branch (slug-scoped or legacy) via git.BranchExists.
		activeBranch, _ := resolveAgentBranch(manifest.FeatureSlug, agent.ID, waveNum, agentRepoDir)

		status := CommitStatus{
			Agent:  agent.ID,
			Branch: activeBranch,
		}

		// Count commits on the branch relative to recorded base commit.
		// Use the wave's base commit (recorded at worktree creation) rather than
		// current HEAD, so verification works even if branches were already merged.
		output, err := git.Run(agentRepoDir, "rev-list", "--count", baseCommit+".."+activeBranch)

		if err != nil {
			// The base commit may not exist in this repo (cross-repo wave: base commit
			// was recorded from a different repo). Fall back to HEAD..branch in the
			// agent's own repo, which counts commits not yet merged to the local HEAD.
			output, err = git.Run(agentRepoDir, "rev-list", "--count", "HEAD.."+activeBranch)
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

		// Update overall validity and collect warnings
		if !status.HasCommits {
			allValid = false
			warnings = append(warnings, result.SAWError{
				Code:     result.CodeCompletionVerificationWarning,
				Message:  fmt.Sprintf("agent %s has no commits on branch %s", status.Agent, status.Branch),
				Severity: "warning",
				Field:    "agent",
				Context: map[string]string{
					"agent":  status.Agent,
					"branch": status.Branch,
				},
			})
		}

		data.Agents = append(data.Agents, status)
	}

	// Return appropriate result based on validation status
	if allValid {
		return result.NewSuccess(data)
	}
	return result.NewPartial(data, warnings)
}
