package protocol

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// WorktreeInfo contains the details of a created worktree for a single agent.
type WorktreeInfo struct {
	Agent  string `json:"agent"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

// CreateWorktreesData contains the list of worktrees created for a wave.
type CreateWorktreesData struct {
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
// Each worktree is created at {agentRepoDir}/.claude/worktrees/saw/{slug}/wave{N}-agent-{Letter}
// on a new branch named saw/{slug}/wave{N}-agent-{Letter}.
//
// If any worktree creation fails, returns an error immediately without
// attempting to create remaining worktrees.
//
// Returns a Result containing CreateWorktreesData on success, or structured errors if:
// - The IMPL doc cannot be parsed
// - The specified wave number is not found in the document
// - Any git worktree add operation fails
func CreateWorktrees(manifestPath string, waveNum int, repoDir string, logger *slog.Logger) result.Result[CreateWorktreesData] {
	log := loggerFrom(logger)
	// Load IMPL doc (pure YAML format)
	doc, err := Load(manifestPath)
	if err != nil {
		return result.NewFailure[CreateWorktreesData]([]result.SAWError{
			{
				Code:     result.CodeIMPLParseFailed,
				Message:  fmt.Sprintf("failed to load manifest: %v", err),
				Severity: "fatal",
			},
		})
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
		return result.NewFailure[CreateWorktreesData]([]result.SAWError{
			{
				Code:     result.CodeWaveNotReady,
				Message:  fmt.Sprintf("wave %d not found in manifest", waveNum),
				Severity: "fatal",
			},
		})
	}

	// Record base commit for post-merge verification (prevention fix for verify-commits bug)
	baseCommit, err := git.RevParse(repoDir, "HEAD")
	if err != nil {
		return result.NewFailure[CreateWorktreesData]([]result.SAWError{
			{
				Code:     result.CodeWorktreeCreateFailed,
				Message:  fmt.Sprintf("failed to get base commit: %v", err),
				Severity: "fatal",
			},
		})
	}
	targetWave.BaseCommit = baseCommit

	// Save manifest with base commit recorded
	if err := Save(doc, manifestPath); err != nil {
		return result.NewFailure[CreateWorktreesData]([]result.SAWError{
			{
				Code:     result.CodeIMPLParseFailed,
				Message:  fmt.Sprintf("failed to save manifest with base commit: %v", err),
				Severity: "fatal",
			},
		})
	}

	// Resolve absolute path for repoDir (handles "." case)
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return result.NewFailure[CreateWorktreesData]([]result.SAWError{
			{
				Code:     result.CodeWorktreeCreateFailed,
				Message:  fmt.Sprintf("failed to resolve repo path: %v", err),
				Severity: "fatal",
			},
		})
	}

	// Determine parent directory for cross-repo resolution
	repoParent := filepath.Dir(absRepoDir)

	// Detect same-repo case: all agents have identical repo value matching current repo basename
	currentRepoName := filepath.Base(absRepoDir)
	isSameRepo := true
	var firstRepo string
	for _, agent := range targetWave.Agents {
		agentRepo := AgentRepoName(doc.FileOwnership, agent.ID)
		if agentRepo == "" {
			// Empty repo field is always same-repo
			continue
		}
		if firstRepo == "" {
			firstRepo = agentRepo
		}
		if agentRepo != firstRepo || agentRepo != currentRepoName {
			isSameRepo = false
			break
		}
	}

	// Create worktrees for each agent
	var worktrees []WorktreeInfo
	for _, agent := range targetWave.Agents {
		// Determine agent's repo by looking up their files in FileOwnership
		agentRepo := AgentRepoName(doc.FileOwnership, agent.ID)

		// Resolve repo directory (cross-repo or same-repo)
		var agentRepoDir string
		if agentRepo == "" || isSameRepo {
			// No repo specified OR same-repo with unnecessary repo field - use absRepoDir
			agentRepoDir = absRepoDir
		} else {
			// Cross-repo: resolve as sibling directory
			agentRepoDir = filepath.Join(repoParent, agentRepo)
		}

		// Construct worktree path and branch name using slug-scoped helpers
		worktreePath := WorktreeDir(agentRepoDir, doc.FeatureSlug, waveNum, agent.ID)
		branchName := BranchName(doc.FeatureSlug, waveNum, agent.ID)

		// Also compute legacy names for backward-compat stale branch cleanup
		legacyBranch := LegacyBranchName(waveNum, agent.ID)
		legacyWorktreePath := filepath.Join(agentRepoDir, ".claude", "worktrees", legacyBranch)

		// Auto-clean stale branches from previous IMPLs that reuse the same
		// wave/agent naming scheme. A branch is "stale" if it already exists and
		// its tip is an ancestor of HEAD (i.e., it was already merged).
		// Check both new-format and legacy branch names.
		for _, candidate := range []struct {
			branch       string
			worktreePath string
		}{
			{branchName, worktreePath},
			{legacyBranch, legacyWorktreePath},
		} {
			if git.BranchExists(agentRepoDir, candidate.branch) {
				if git.IsAncestor(agentRepoDir, candidate.branch, "HEAD") {
					// Branch is merged — safe to delete. Also remove its worktree if present.
					_ = git.WorktreeRemove(agentRepoDir, candidate.worktreePath)
					_ = git.DeleteBranch(agentRepoDir, candidate.branch)
					log.Debug("protocol: cleaned up stale merged branch", "branch", candidate.branch, "repo", agentRepoDir)
				} else if candidate.branch == branchName {
					// Only error on the primary (new-format) branch; legacy branches
					// that are unmerged are a soft warning.
					return result.NewFailure[CreateWorktreesData]([]result.SAWError{
						{
							Code:     result.CodeBranchExists,
							Message:  fmt.Sprintf("branch %q already exists in %s and is not merged into HEAD; delete it manually or merge first", candidate.branch, agentRepoDir),
							Severity: "fatal",
						},
					})
				}
			}
		}

		// Check if worktree already exists (defensive staleness detection)
		if git.WorktreeExists(agentRepoDir, worktreePath) {
			// Worktree exists - check if it's stale
			currentHead, err := git.RevParse(agentRepoDir, "HEAD")
			if err != nil {
				return result.NewFailure[CreateWorktreesData]([]result.SAWError{
					{
						Code:     result.CodeWorktreeCreateFailed,
						Message:  fmt.Sprintf("failed to get current HEAD: %v", err),
						Severity: "fatal",
					},
				})
			}

			worktreeBase, err := git.GetWorktreeBaseCommit(agentRepoDir, worktreePath)
			if err != nil {
				// Can't determine base commit - treat as stale for safety
				log.Warn("protocol: failed to get worktree base commit, treating as stale", "agent", agent.ID, "err", err)
				_ = git.WorktreeRemove(agentRepoDir, worktreePath)
				_ = git.DeleteBranch(agentRepoDir, branchName)
				log.Debug("protocol: removed stale worktree", "agent", agent.ID, "reason", "unable to verify base commit")
			} else if worktreeBase != currentHead {
				// Base commit doesn't match - stale worktree
				_ = git.WorktreeRemove(agentRepoDir, worktreePath)
				_ = git.DeleteBranch(agentRepoDir, branchName)
				log.Debug("protocol: removed stale worktree", "agent", agent.ID, "reason", "base commit mismatch")
			} else {
				// Base is current - check hooks
				hookValid, err := git.VerifyHookInWorktree(worktreePath)
				if err != nil {
					// Hook check I/O error - log warning but continue
					log.Warn("protocol: failed to verify hook", "agent", agent.ID, "err", err)
				} else if !hookValid {
					// Hooks missing or invalid - recreate
					_ = git.WorktreeRemove(agentRepoDir, worktreePath)
					_ = git.DeleteBranch(agentRepoDir, branchName)
					log.Debug("protocol: removed worktree with invalid hooks", "agent", agent.ID)
				} else {
					// Worktree is valid - skip creation
					log.Debug("protocol: reusing valid worktree", "agent", agent.ID)
					worktrees = append(worktrees, WorktreeInfo{
						Agent:  agent.ID,
						Path:   worktreePath,
						Branch: branchName,
					})
					continue
				}
			}
		}

		// Create the worktree
		if err := git.WorktreeAdd(agentRepoDir, worktreePath, branchName); err != nil {
			return result.NewFailure[CreateWorktreesData]([]result.SAWError{
				{
					Code:     result.CodeWorktreeCreateFailed,
					Message:  fmt.Sprintf("failed to create worktree for agent %s in repo %s: %v", agent.ID, agentRepoDir, err),
					Severity: "fatal",
				},
			})
		}

		// Install pre-commit hook (H10 isolation enforcement)
		if err := git.InstallHooks(agentRepoDir, worktreePath); err != nil {
			// Log warning but don't fail — hook verification in prepare-wave will catch this
			// and provide actionable error message
			log.Warn("protocol: failed to install hooks", "agent", agent.ID, "err", err)
		}

		// Collect worktree info
		worktrees = append(worktrees, WorktreeInfo{
			Agent:  agent.ID,
			Path:   worktreePath,
			Branch: branchName,
		})
	}

	return result.NewSuccess(CreateWorktreesData{
		Worktrees: worktrees,
	})
}
