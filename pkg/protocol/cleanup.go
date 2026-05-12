package protocol

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// loggerFrom returns the provided logger if non-nil, otherwise returns slog.Default().
func loggerFrom(l *slog.Logger) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l
}

// CleanupAllStale removes all stale worktrees across all slugs.
func CleanupAllStale(ctx context.Context, repoDir string, force bool) result.Result[*StaleCleanupData] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[*StaleCleanupData]([]result.PolywaveError{
			{
				Code:     result.CodeWorktreeCreateFailed,
				Message:  fmt.Sprintf("cleanup all stale: %v", err),
				Severity: "fatal",
			},
		})
	}
	stale, err := DetectStaleWorktrees(repoDir)
	if err != nil {
		return result.NewFailure[*StaleCleanupData]([]result.PolywaveError{
			{
				Code:     result.CodeWorktreeCreateFailed,
				Message:  fmt.Sprintf("detect stale worktrees: %v", err),
				Severity: "fatal",
			},
		})
	}
	if len(stale) == 0 {
		return result.NewSuccess(&StaleCleanupData{
			Cleaned: []StaleWorktree{},
			Skipped: []StaleWorktree{},
			Errors:  []StaleCleanupFailure{},
		})
	}
	if err := ctx.Err(); err != nil {
		return result.NewFailure[*StaleCleanupData]([]result.PolywaveError{
			{
				Code:     result.CodeWorktreeCreateFailed,
				Message:  fmt.Sprintf("cleanup all stale: %v", err),
				Severity: "fatal",
			},
		})
	}
	res := CleanStaleWorktrees(stale, force)
	if !res.IsSuccess() {
		// Return partial data if available (some cleaned, some failed)
		if res.Data != nil {
			return result.NewPartial(*res.Data, res.Errors)
		}
		return result.NewFailure[*StaleCleanupData](res.Errors)
	}
	data := res.GetData()
	return result.NewSuccess(data)
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
// The provided ctx is checked for cancellation before each agent's cleanup. If the
// context is cancelled, Cleanup returns immediately with the partial data collected
// so far plus a context error.
//
// Returns result.Result[CleanupData] with status for each agent, or an error if the
// wave is not found.
func Cleanup(ctx context.Context, manifestPath string, waveNum int, repoDir string, logger *slog.Logger) result.Result[CleanupData] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[CleanupData]([]result.PolywaveError{
			{
				Code:     result.CodeWorktreeCreateFailed,
				Message:  fmt.Sprintf("cleanup cancelled: %v", err),
				Severity: "fatal",
			},
		})
	}
	log := loggerFrom(logger)
	// Load manifest
	manifest, err := Load(ctx, manifestPath)
	if err != nil {
		return result.NewFailure[CleanupData]([]result.PolywaveError{
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
		return result.NewFailure[CleanupData]([]result.PolywaveError{
			{
				Code:     result.CodeWaveNotReady,
				Message:  fmt.Sprintf("wave %d not found in manifest", waveNum),
				Severity: "fatal",
			},
		})
	}

	data := CleanupData{
		Wave:   waveNum,
		Agents: make([]CleanupStatus, 0, len(targetWave.Agents)),
	}

	// Resolve repoDir to absolute path for cross-repo resolution.
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return result.NewFailure[CleanupData]([]result.PolywaveError{
			{
				Code:     result.CodeWorktreeCreateFailed,
				Message:  fmt.Sprintf("failed to resolve repo dir: %v", err),
				Severity: "fatal",
			},
		})
	}
	repoParent := filepath.Dir(absRepoDir)

	// Track unique repos for pruning at the end.
	prunedRepos := map[string]bool{absRepoDir: true}

	// Clean up each agent in the wave
	for _, agent := range targetWave.Agents {
		// Check for cancellation before each agent's cleanup.
		if err := ctx.Err(); err != nil {
			return result.NewPartial(data, []result.PolywaveError{
				{
					Code:     result.CodeWorktreeCreateFailed,
					Message:  fmt.Sprintf("cleanup cancelled after %d agents: %v", len(data.Agents), err),
					Severity: "warning",
				},
			})
		}

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

		// Resolve the active branch (slug-scoped or legacy) via git.BranchExists.
		activeBranch, _ := resolveAgentBranch(manifest.FeatureSlug, agent.ID, waveNum, agentRepoDir)

		// Resolve worktree path for the slug-scoped format.
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

		// Attempt to delete the resolved branch. If not found, treat as already deleted.
		delErr := git.DeleteBranch(agentRepoDir, activeBranch)
		if delErr != nil {
			errMsg := delErr.Error()
			if strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "branch") && strings.Contains(errMsg, "not found") {
				status.BranchDeleted = true
			}
		} else {
			status.BranchDeleted = true
		}

		data.Agents = append(data.Agents, status)
	}

	// Best-effort: prune stale worktree entries from all involved repos.
	for repo := range prunedRepos {
		if err := git.WorktreePrune(repo); err != nil {
			log.Warn("protocol: git worktree prune failed", "repo", repo, "err", err)
		}
	}

	// Remove empty slug parent directory if all agent worktrees were cleaned.
	// Worktree paths are: .claude/worktrees/polywave/{slug}/wave{N}-agent-{ID}
	// After removing all agent dirs, the slug dir is left empty.
	for repo := range prunedRepos {
		slugDir := filepath.Join(repo, ".claude", "worktrees", "polywave", manifest.FeatureSlug)
		entries, err := os.ReadDir(slugDir)
		if err == nil && len(entries) == 0 {
			_ = os.Remove(slugDir)
		}
	}

	return result.NewSuccess(data)
}
