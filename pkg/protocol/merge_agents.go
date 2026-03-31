package protocol

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// isAgentAlreadyMerged returns true if the agent's branch (slug or legacy)
// is recorded in the merge log AND is an ancestor of HEAD in repoDir.
// This dual-check prevents false positives when the log has a stale entry
// but the branch was deleted before the actual merge completed.
func isAgentAlreadyMerged(repoDir string, mergeLog *MergeLog, agentID, slugBranch, legacyBranch string) bool {
	return mergeLog.IsMerged(agentID) &&
		(git.IsAncestor(repoDir, slugBranch, "HEAD") ||
			git.IsAncestor(repoDir, legacyBranch, "HEAD"))
}

// PreMergeValidation validates ownership table consistency before any merge operations.
// It verifies:
//  1. Every file_ownership entry for this wave references an agent that exists in the wave.
//  2. No duplicate files appear in ownership entries for this wave (I1 recheck).
//
// Returns a slice of result.SAWError if inconsistencies are found; nil slice means valid.
func PreMergeValidation(manifest *IMPLManifest, waveNum int) []result.SAWError {
	// Find the target wave and build a set of valid agent IDs
	targetWave := manifest.FindWave(waveNum)
	if targetWave == nil {
		return []result.SAWError{{
			Code:     "WAVE_NOT_FOUND",
			Message:  fmt.Sprintf("wave %d not found in manifest", waveNum),
			Severity: "error",
		}}
	}

	validAgents := make(map[string]bool, len(targetWave.Agents))
	for _, agent := range targetWave.Agents {
		validAgents[agent.ID] = true
	}

	var errs []result.SAWError
	seenFiles := make(map[string]string) // file path -> first agent that owns it

	for _, fo := range manifest.FileOwnership {
		if fo.Wave != waveNum {
			continue
		}

		// Check 1: agent in ownership table must exist in the wave
		if !validAgents[fo.Agent] {
			errs = append(errs, result.SAWError{
				Code:     "UNKNOWN_AGENT_IN_OWNERSHIP",
				Message:  fmt.Sprintf("file_ownership entry for wave %d references unknown agent %q (file: %s)", waveNum, fo.Agent, fo.File),
				Severity: "error",
				Field:    "file_ownership",
			})
		}

		// Check 2: no duplicate file ownership (I1 recheck)
		if firstOwner, exists := seenFiles[fo.File]; exists {
			errs = append(errs, result.SAWError{
				Code:     "DUPLICATE_FILE_OWNERSHIP",
				Message:  fmt.Sprintf("file %q is owned by both agent %q and agent %q in wave %d (I1 violation)", fo.File, firstOwner, fo.Agent, waveNum),
				Severity: "error",
				Field:    "file_ownership",
			})
		} else {
			seenFiles[fo.File] = fo.Agent
		}
	}

	return errs
}

// WorktreesAbsent returns true if none of the expected worktree directories
// for the given wave exist on disk. Uses WorktreeDir() to compute paths.
// repoDir must be the absolute repo root.
//
// This detects the "solo wave" path where no worktrees were created
// (e.g., a solo developer working directly on main). VerifyCommits would
// otherwise fail because it looks for branches that don't exist.
func WorktreesAbsent(manifest *IMPLManifest, waveNum int, repoDir string) bool {
	// Find the target wave
	targetWave := manifest.FindWave(waveNum)
	if targetWave == nil {
		return false
	}
	for _, agent := range targetWave.Agents {
		path := WorktreeDir(repoDir, manifest.FeatureSlug, waveNum, agent.ID)
		if _, err := os.Stat(path); err == nil {
			return false // at least one worktree exists
		}
	}
	return true
}

// AllBranchesAbsent returns true when every agent branch for the wave
// is absent from the repository — both slug-scoped and legacy names.
// Used by finalize-wave to detect the idempotent re-run path.
//
// NOTE: MergeAgents handles idempotency for the merge step (E9).
// For the case where all branches have been deleted post-cleanup,
// the finalize_wave.go caller must skip VerifyCommits and MergeAgents
// when it detects that all expected branches are absent from the repo
// (see AllBranchesAbsent check in finalize_wave.go).
func AllBranchesAbsent(manifest *IMPLManifest, waveNum int, repoDir string) bool {
	targetWave := manifest.FindWave(waveNum)
	if targetWave == nil {
		return true // no wave = no branches
	}
	for _, agent := range targetWave.Agents {
		slugBranch := BranchName(manifest.FeatureSlug, waveNum, agent.ID)
		legacyBranch := LegacyBranchName(waveNum, agent.ID)
		if git.BranchExists(repoDir, slugBranch) || git.BranchExists(repoDir, legacyBranch) {
			return false
		}
	}
	return true
}

// MergeStatus represents the outcome of merging a single agent branch.
type MergeStatus struct {
	Agent         string `json:"agent"`
	Branch        string `json:"branch"`
	Success       bool   `json:"success"`
	StatusUpdated bool   `json:"status_updated,omitempty"`
	Error         string `json:"error,omitempty"`
}

// MergeAgentsData represents the outcome of merging all agents in a wave.
// Wrapped in result.Result[MergeAgentsData] for consistent error handling.
type MergeAgentsData struct {
	Wave      int           `json:"wave"`
	Merges    []MergeStatus `json:"merges"`
	NextState ProtocolState `json:"next_state,omitempty"` // State after successful merge
	// Success field maintained for backward compatibility with legacy code
	// TODO: Remove once all consumers migrated to result.Result[T].IsSuccess()
	Success bool `json:"success"`
}

// MergeAgents merges all agent branches from a specified wave into their respective repositories.
// It automatically detects multi-repo waves by reading the file ownership table and completion reports.
//
// The function stops on the first merge conflict and returns a partial result.
// All merges are recorded in the result, including both successful and failed attempts.
//
// Parameters via MergeAgentsOpts:
//   - ManifestPath: path to the IMPL manifest file
//   - WaveNum: wave number to merge
//   - RepoDir: default repository directory (used when agent repo is not explicitly specified)
//   - MergeTarget: target branch for merge; empty string means merge to current HEAD (backward compatible)
//   - Logger: structured logger for operation logging
//
// Returns:
//   - result.Result[MergeAgentsData] with wave number, merge statuses
//   - error if manifest cannot be loaded or wave is not found (not returned for merge conflicts)
func MergeAgents(opts MergeAgentsOpts) (result.Result[MergeAgentsData], error) {
	manifestPath := opts.ManifestPath
	waveNum := opts.WaveNum
	repoDir := opts.RepoDir
	mergeTarget := opts.MergeTarget
	logger := opts.Logger
	// Load manifest to check if this is a multi-repo wave
	manifest, err := Load(manifestPath)
	if err != nil {
		return result.Result[MergeAgentsData]{}, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find the specified wave
	targetWave := manifest.FindWave(waveNum)
	if targetWave == nil {
		return result.Result[MergeAgentsData]{}, fmt.Errorf("wave %d not found in manifest", waveNum)
	}

	// Resolve repoDir to absolute path; repo names in fo.Repo are resolved as
	// siblings of this directory (same pattern as worktree.go line 116).
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return result.Result[MergeAgentsData]{}, fmt.Errorf("failed to resolve repo dir: %w", err)
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

	// Check if all agents use the same repo
	repoSet := make(map[string]bool)
	for _, repo := range agentRepos {
		repoSet[repo] = true
	}

	// If single-repo wave, use optimized single-repo logic
	if len(repoSet) == 1 {
		return mergeAgentsSingleRepo(manifestPath, waveNum, absRepoDir, manifest, targetWave, mergeTarget, logger)
	}

	// Multi-repo wave: merge each repo group separately
	return mergeAgentsMultiRepo(manifestPath, waveNum, manifest, targetWave, agentRepos, mergeTarget, logger)
}

// buildFileOwnerMap constructs a map of relative file path → agent ID for all
// file ownership entries in the given wave. Used by MergeNoFFWithOwnership to
// auto-resolve conflicts using I1 guarantees.
func buildFileOwnerMap(manifest *IMPLManifest, waveNum int) map[string]string {
	owners := make(map[string]string)
	for _, fo := range manifest.FileOwnership {
		if fo.Wave == waveNum {
			owners[fo.File] = fo.Agent
		}
	}
	// Also populate from agent.Files fields (some IMPL docs use both)
	for _, wave := range manifest.Waves {
		if wave.Number != waveNum {
			continue
		}
		for _, agent := range wave.Agents {
			for _, f := range agent.Files {
				if _, exists := owners[f]; !exists {
					owners[f] = agent.ID
				}
			}
		}
	}
	return owners
}

// mergeAgentsSingleRepo handles merging when all agents belong to the same repository.
// When mergeTarget is non-empty, checks out the target branch before the merge loop.
func mergeAgentsSingleRepo(manifestPath string, waveNum int, repoDir string, manifest *IMPLManifest, targetWave *Wave, mergeTarget string, logger *slog.Logger) (result.Result[MergeAgentsData], error) {
	log := loggerFrom(logger)
	// H4: PreMergeValidation — verify ownership table consistency before any git operations.
	if validationErrs := PreMergeValidation(manifest, waveNum); len(validationErrs) > 0 {
		return result.NewFailure[MergeAgentsData]([]result.SAWError{{
			Code:     "PRE_MERGE_VALIDATION_FAILED",
			Message:  validationErrs[0].Message,
			Severity: "fatal",
			Field:    "file_ownership",
		}}), nil
	}

	// Checkout merge target branch before merge loop (E28)
	if mergeTarget != "" {
		if _, err := git.Run(repoDir, "checkout", mergeTarget); err != nil {
			return result.Result[MergeAgentsData]{}, fmt.Errorf("failed to checkout merge target %s: %w", mergeTarget, err)
		}
	}

	// Load merge-log for idempotency (E9)
	mergeLog, err := LoadMergeLog(manifestPath, waveNum)
	if err != nil {
		return result.Result[MergeAgentsData]{}, fmt.Errorf("failed to load merge-log: %w", err)
	}

	// Build file ownership map for conflict auto-resolution
	fileOwners := buildFileOwnerMap(manifest, waveNum)

	// Initialize result data
	data := MergeAgentsData{
		Wave:    waveNum,
		Merges:  make([]MergeStatus, 0, len(targetWave.Agents)),
		Success: true, // Set to false on merge failure for backward compat
	}

	// Merge each agent branch
	for _, agent := range targetWave.Agents {
		branch := BranchName(manifest.FeatureSlug, waveNum, agent.ID)
		legacyBranch := LegacyBranchName(waveNum, agent.ID)
		// Check if agent already merged (idempotency): merge log AND git history agree.
		// Trusting only the log caused data loss when the log had a stale entry and the
		// branch was deleted during cleanup before the actual merge happened.
		// Check both slug-scoped and legacy branch names for backward compatibility.
		if isAgentAlreadyMerged(repoDir, mergeLog, agent.ID, branch, legacyBranch) {
			status := MergeStatus{
				Agent:   agent.ID,
				Branch:  branch,
				Success: true,
				Error:   "already merged (skipped)",
			}
			data.Merges = append(data.Merges, status)
			continue
		}

		// Build commit message prefix
		prefix := fmt.Sprintf("Merge %s: ", branch)

		// Truncate task to ensure total message fits in reasonable length
		// Limit task portion to 50 chars
		taskSummary := agent.Task
		if len(taskSummary) > 50 {
			taskSummary = taskSummary[:50]
		}
		message := prefix + taskSummary

		// Perform merge — auto-resolve conflicts using ownership table (I1)
		// Try slug-scoped branch first, fall back to legacy branch name
		status := MergeStatus{
			Agent:  agent.ID,
			Branch: branch,
		}

		err := git.MergeNoFFWithOwnership(repoDir, branch, message, agent.ID, fileOwners)
		if err != nil {
			// Try legacy branch name as fallback for backward compatibility
			err = git.MergeNoFFWithOwnership(repoDir, legacyBranch, message, agent.ID, fileOwners)
			if err == nil {
				status.Branch = legacyBranch
			}
		}
		if err != nil {
			// Merge failed - record error and stop
			status.Success = false
			status.Error = err.Error()
			data.Merges = append(data.Merges, status)
			data.Success = false // Set for backward compat
			// Return partial result with failures
			return result.NewPartial(data, []result.SAWError{{
				Code:     "MERGE_CONFLICT",
				Message:  fmt.Sprintf("merge failed for agent %s: %s", agent.ID, err.Error()),
				Severity: "error",
				Field:    "agent",
				Context:  map[string]string{"agent": agent.ID, "branch": status.Branch},
			}}), nil
		}

		// Get merge commit SHA
		mergeSHA, err := git.RevParse(repoDir, "HEAD")
		if err != nil {
			// Non-fatal: log warning but continue
			mergeSHA = "unknown"
		}

		// POST-MERGE VERIFICATION: Ensure agent's commit is actually in HEAD's history
		// This catches cases where merge operation succeeded but commit didn't land
		// (e.g., due to conflict handling bugs, partial merges, or git anomalies)
		if report, ok := manifest.CompletionReports[agent.ID]; ok && report.Commit != "" {
			if !git.IsAncestor(repoDir, report.Commit, "HEAD") {
				status.Success = false
				status.Error = fmt.Sprintf("merge operation succeeded but agent commit %s not found in HEAD history (verification failed)", report.Commit)
				data.Merges = append(data.Merges, status)
				data.Success = false
				return result.NewPartial(data, []result.SAWError{{
					Code:     "MERGE_VERIFICATION_FAILED",
					Message:  fmt.Sprintf("agent %s: merge succeeded but commit %s not in HEAD history", agent.ID, report.Commit),
					Severity: "error",
					Field:    "agent",
					Context:  map[string]string{"agent": agent.ID, "commit": report.Commit, "merge_sha": mergeSHA},
				}}), nil
			}
		}

		// Record merge in log (E9)
		mergeLog.AddMergeEntry(agent.ID, mergeSHA)
		if saveRes := SaveMergeLog(manifestPath, waveNum, mergeLog); saveRes.IsFatal() {
			// Non-fatal: log warning but continue (best-effort tracking)
			log.Warn("protocol: failed to save merge-log", "err", saveRes.Errors[0].Message)
		}

		// Merge succeeded — auto-update completion status (best-effort)
		status.Success = true
		if res := UpdateStatus(manifestPath, waveNum, agent.ID, StatusComplete, UpdateStatusOpts{}); res.IsSuccess() {
			status.StatusUpdated = true
		}
		data.Merges = append(data.Merges, status)
	}

	// All merges succeeded - update state to next wave or COMPLETE
	nextState := determineNextState(manifest, waveNum)
	if err := updateManifestState(manifestPath, nextState); err == nil {
		data.NextState = nextState
	}

	return result.NewSuccess(data), nil
}

// mergeAgentsMultiRepo handles merging when agents span multiple repositories.
// When mergeTarget is non-empty, checks out the target branch in each repo before merging.
func mergeAgentsMultiRepo(manifestPath string, waveNum int, manifest *IMPLManifest, targetWave *Wave, agentRepos map[string]string, mergeTarget string, logger *slog.Logger) (result.Result[MergeAgentsData], error) {
	log := loggerFrom(logger)
	// H4: PreMergeValidation — verify ownership table consistency before any git operations.
	if validationErrs := PreMergeValidation(manifest, waveNum); len(validationErrs) > 0 {
		return result.NewFailure[MergeAgentsData]([]result.SAWError{{
			Code:     "PRE_MERGE_VALIDATION_FAILED",
			Message:  validationErrs[0].Message,
			Severity: "fatal",
			Field:    "file_ownership",
		}}), nil
	}

	// Load merge-log for idempotency (E9)
	mergeLog, err := LoadMergeLog(manifestPath, waveNum)
	if err != nil {
		return result.Result[MergeAgentsData]{}, fmt.Errorf("failed to load merge-log: %w", err)
	}

	// Build file ownership map for conflict auto-resolution
	fileOwners := buildFileOwnerMap(manifest, waveNum)

	// Group agents by repository
	repoGroups := make(map[string][]Agent)
	for _, agent := range targetWave.Agents {
		repo := agentRepos[agent.ID]
		repoGroups[repo] = append(repoGroups[repo], agent)
	}

	// Initialize combined result data
	data := MergeAgentsData{
		Wave:    waveNum,
		Merges:  make([]MergeStatus, 0, len(targetWave.Agents)),
		Success: true, // Set to false on merge failure for backward compat
	}

	// Merge each repository group
	for repoDir, agents := range repoGroups {
		// Resolve relative paths
		absRepoDir := repoDir
		if !filepath.IsAbs(repoDir) {
			// Assume relative to manifest dir
			manifestDir := filepath.Dir(manifestPath)
			absRepoDir = filepath.Join(manifestDir, repoDir)
		}

		// Checkout merge target branch before merge loop (E28)
		if mergeTarget != "" {
			if _, err := git.Run(absRepoDir, "checkout", mergeTarget); err != nil {
				return result.Result[MergeAgentsData]{}, fmt.Errorf("failed to checkout merge target %s in %s: %w", mergeTarget, repoDir, err)
			}
		}

		// Merge agents in this repo
		for _, agent := range agents {
			branch := BranchName(manifest.FeatureSlug, waveNum, agent.ID)
			legacyBranch := LegacyBranchName(waveNum, agent.ID)
			// Check if agent already merged (idempotency): merge log AND git history agree.
			// Check both slug-scoped and legacy branch names for backward compatibility.
			if isAgentAlreadyMerged(absRepoDir, mergeLog, agent.ID, branch, legacyBranch) {
				status := MergeStatus{
					Agent:   agent.ID,
					Branch:  branch,
					Success: true,
					Error:   "already merged (skipped)",
				}
				data.Merges = append(data.Merges, status)
				continue
			}

			// Build commit message
			prefix := fmt.Sprintf("Merge %s: ", branch)
			taskSummary := agent.Task
			if len(taskSummary) > 50 {
				taskSummary = taskSummary[:50]
			}
			message := prefix + taskSummary

			// Perform merge — try slug-scoped branch first, fall back to legacy
			status := MergeStatus{
				Agent:  agent.ID,
				Branch: branch,
			}

			err := git.MergeNoFFWithOwnership(absRepoDir, branch, message, agent.ID, fileOwners)
			if err != nil {
				// Try legacy branch name as fallback for backward compatibility
				err = git.MergeNoFFWithOwnership(absRepoDir, legacyBranch, message, agent.ID, fileOwners)
				if err == nil {
					status.Branch = legacyBranch
				}
			}
			if err != nil {
				// Merge failed - record error and stop
				status.Success = false
				status.Error = fmt.Sprintf("%s (repo: %s)", err.Error(), repoDir)
				data.Merges = append(data.Merges, status)
				data.Success = false // Set for backward compat
				// Return partial result with failures
				return result.NewPartial(data, []result.SAWError{{
					Code:     "MERGE_CONFLICT",
					Message:  fmt.Sprintf("merge failed for agent %s in repo %s: %s", agent.ID, repoDir, err.Error()),
					Severity: "error",
					Field:    "agent",
					Context:  map[string]string{"agent": agent.ID, "branch": status.Branch, "repo": repoDir},
				}}), nil
			}

			// Get merge commit SHA
			mergeSHA, err := git.RevParse(absRepoDir, "HEAD")
			if err != nil {
				// Non-fatal: log warning but continue
				mergeSHA = "unknown"
			}

			// POST-MERGE VERIFICATION: Ensure agent's commit is actually in HEAD's history
			if report, ok := manifest.CompletionReports[agent.ID]; ok && report.Commit != "" {
				if !git.IsAncestor(absRepoDir, report.Commit, "HEAD") {
					status.Success = false
					status.Error = fmt.Sprintf("merge operation succeeded but agent commit %s not found in HEAD history (verification failed, repo: %s)", report.Commit, repoDir)
					data.Merges = append(data.Merges, status)
					data.Success = false
					return result.NewPartial(data, []result.SAWError{{
						Code:     "MERGE_VERIFICATION_FAILED",
						Message:  fmt.Sprintf("agent %s in repo %s: merge succeeded but commit %s not in HEAD history", agent.ID, repoDir, report.Commit),
						Severity: "error",
						Field:    "agent",
						Context:  map[string]string{"agent": agent.ID, "commit": report.Commit, "merge_sha": mergeSHA, "repo": repoDir},
					}}), nil
				}
			}

			// Record merge in log (E9)
			mergeLog.AddMergeEntry(agent.ID, mergeSHA)
			if saveRes := SaveMergeLog(manifestPath, waveNum, mergeLog); saveRes.IsFatal() {
				// Non-fatal: log warning but continue (best-effort tracking)
				log.Warn("protocol: failed to save merge-log", "err", saveRes.Errors[0].Message)
			}

			// Merge succeeded
			status.Success = true
			if res := UpdateStatus(manifestPath, waveNum, agent.ID, StatusComplete, UpdateStatusOpts{}); res.IsSuccess() {
				status.StatusUpdated = true
			}
			data.Merges = append(data.Merges, status)
		}
	}

	// All merges succeeded - update state
	nextState := determineNextState(manifest, waveNum)
	if err := updateManifestState(manifestPath, nextState); err == nil {
		data.NextState = nextState
	}

	return result.NewSuccess(data), nil
}

// determineNextState calculates the next protocol state after a successful wave merge.
func determineNextState(manifest *IMPLManifest, completedWave int) ProtocolState {
	// If this was the final wave, mark complete
	if completedWave >= len(manifest.Waves) {
		return StateComplete
	}

	// Otherwise, next wave is pending
	return StateWavePending // Orchestrator will update to WAVE{N}_PENDING format
}

// updateManifestState updates the state field in the IMPL manifest file.
// Uses line-based editing to preserve formatting (same pattern as mark-complete).
func updateManifestState(manifestPath string, newState ProtocolState) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	stateUpdated := false

	for i, line := range lines {
		if strings.HasPrefix(line, "state:") {
			lines[i] = fmt.Sprintf("state: \"%s\"", newState)
			stateUpdated = true
			break
		}
	}

	if !stateUpdated {
		return fmt.Errorf("state field not found in manifest")
	}

	// Write back
	updated := strings.Join(lines, "\n")
	if err := os.WriteFile(manifestPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}
