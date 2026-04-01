package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// FinalizeTierOpts configures the thick finalize-tier orchestrator.
type FinalizeTierOpts struct {
	ManifestPath string       // absolute path to PROGRAM manifest (required)
	TierNumber   int          // tier to finalize (required)
	RepoDir      string       // absolute path to repo root (required)
	Logger       *slog.Logger // optional; nil falls back to slog.Default()
}

// FinalizeTierResult is the structured result of FinalizeTierEngine.
type FinalizeTierResult struct {
	TierNumber     int                                  `json:"tier_number"`
	ImplsClosed    []string                             `json:"impls_closed"`
	ImplsSkipped   []string                             `json:"impls_skipped"`
	MergeResults   map[string]*protocol.MergeAgentsData `json:"merge_results"`
	TierGateResult *protocol.TierGateData               `json:"tier_gate_result,omitempty"`
	StateAdvanced  bool                                 `json:"state_advanced"`
	Errors         []string                             `json:"errors,omitempty"`
}

// FinalizeTierEngine is the thick orchestrator for finalize-tier.
// Replaces the thin protocol.FinalizeTier call in the CLI adapter.
// Steps (each idempotent):
//  1. For each IMPL in the tier: call MarkIMPLComplete if not already closed.
//  2. Merge each IMPL branch into HEAD, handling the "branch in worktree" case.
//  3. Run RunTierGate with enriched statuses.
//  4. Update IMPL statuses in the PROGRAM manifest and commit.
func FinalizeTierEngine(ctx context.Context, opts FinalizeTierOpts) (FinalizeTierResult, error) {
	logger := loggerFrom(opts.Logger)

	result := FinalizeTierResult{
		TierNumber:   opts.TierNumber,
		ImplsClosed:  []string{},
		ImplsSkipped: []string{},
		MergeResults: make(map[string]*protocol.MergeAgentsData),
	}

	// Step 1 — Close all complete IMPLs
	logger.Info("finalize-tier: step 1 — closing IMPLs", "tier", opts.TierNumber)

	manifest, err := protocol.ParseProgramManifest(opts.ManifestPath)
	if err != nil {
		return result, fmt.Errorf("finalize-tier: parse manifest: %w", err)
	}

	// Find the target tier
	var targetTier *protocol.ProgramTier
	for i := range manifest.Tiers {
		if manifest.Tiers[i].Number == opts.TierNumber {
			targetTier = &manifest.Tiers[i]
			break
		}
	}
	if targetTier == nil {
		return result, fmt.Errorf("finalize-tier: tier %d not found in manifest", opts.TierNumber)
	}

	for _, implSlug := range targetTier.Impls {
		implPath, err := protocol.ResolveIMPLPath(opts.RepoDir, implSlug)
		if err != nil {
			// ResolveIMPLPath returns an error when not found — treat as archived
			logger.Info("finalize-tier: IMPL not found on disk (already archived?)", "slug", implSlug)
			result.ImplsSkipped = append(result.ImplsSkipped, implSlug)
			continue
		}
		if _, statErr := os.Stat(implPath); os.IsNotExist(statErr) {
			logger.Info("finalize-tier: IMPL not on disk (already archived)", "slug", implSlug)
			result.ImplsSkipped = append(result.ImplsSkipped, implSlug)
			continue
		}

		res := MarkIMPLComplete(ctx, MarkIMPLCompleteOpts{
			IMPLPath: implPath,
			RepoPath: opts.RepoDir,
			Logger:   opts.Logger,
		})
		if res.IsFatal() {
			logger.Warn("finalize-tier: failed to close IMPL", "slug", implSlug, "error", res.Errors[0].Message)
			result.Errors = append(result.Errors, fmt.Sprintf("close IMPL %s: %s", implSlug, res.Errors[0].Message))
			// non-fatal, continue
		} else {
			logger.Info("finalize-tier: closed IMPL", "slug", implSlug)
			result.ImplsClosed = append(result.ImplsClosed, implSlug)
		}
	}

	// Step 2 — Merge IMPL branches (worktree-aware)
	logger.Info("finalize-tier: step 2 — merging IMPL branches", "tier", opts.TierNumber)

	for _, implSlug := range targetTier.Impls {
		branch := protocol.ProgramBranchName(manifest.ProgramSlug, opts.TierNumber, implSlug)

		if !git.BranchExists(opts.RepoDir, branch) {
			logger.Info("finalize-tier: branch not found (already merged or never created)", "branch", branch)
			result.MergeResults[implSlug] = &protocol.MergeAgentsData{
				Wave:   opts.TierNumber,
				Merges: []protocol.MergeStatus{{Agent: implSlug, Branch: branch, Success: true, Error: "skipped: branch not found"}},
			}
			continue
		}

		if git.IsAncestor(opts.RepoDir, branch, "HEAD") {
			logger.Info("finalize-tier: branch already merged into HEAD", "branch", branch)
			result.MergeResults[implSlug] = &protocol.MergeAgentsData{
				Wave:   opts.TierNumber,
				Merges: []protocol.MergeStatus{{Agent: implSlug, Branch: branch, Success: true, Error: "skipped: already merged"}},
			}
			continue
		}

		message := fmt.Sprintf("Merge program tier %d impl %s: %s", opts.TierNumber, implSlug, branch)
		if mergeErr := mergeIMPLBranchWorktreeAware(opts.RepoDir, branch, message); mergeErr != nil {
			logger.Warn("finalize-tier: merge failed", "branch", branch, "error", mergeErr)
			result.Errors = append(result.Errors, fmt.Sprintf("merge %s: %v", branch, mergeErr))
			return result, nil // stop-on-first behavior
		}

		logger.Info("finalize-tier: merged IMPL branch", "branch", branch)
		result.MergeResults[implSlug] = &protocol.MergeAgentsData{
			Wave:   opts.TierNumber,
			Merges: []protocol.MergeStatus{{Agent: implSlug, Branch: branch, Success: true}},
		}
	}

	// Step 3 — Run tier gate
	logger.Info("finalize-tier: step 3 — running tier gate", "tier", opts.TierNumber)

	// Re-parse manifest so enriched statuses are picked up
	manifest, err = protocol.ParseProgramManifest(opts.ManifestPath)
	if err != nil {
		logger.Warn("finalize-tier: failed to re-parse manifest for tier gate", "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("re-parse manifest: %v", err))
		return result, nil
	}

	gateRes := protocol.RunTierGate(manifest, opts.TierNumber, opts.RepoDir)
	gateData := gateRes.GetData()
	result.TierGateResult = gateData

	if gateData == nil || !gateData.Passed {
		errMsg := "tier gate did not pass"
		if len(gateRes.Errors) > 0 {
			errMsg = gateRes.Errors[0].Message
		}
		logger.Warn("finalize-tier: tier gate failed", "error", errMsg)
		result.Errors = append(result.Errors, fmt.Sprintf("tier gate: %s", errMsg))
		return result, nil
	}

	// Step 4 — Update IMPL statuses in PROGRAM manifest and commit
	logger.Info("finalize-tier: step 4 — updating manifest and committing", "tier", opts.TierNumber)

	for i := range manifest.Impls {
		for _, slug := range targetTier.Impls {
			if manifest.Impls[i].Slug == slug {
				manifest.Impls[i].Status = "complete"
			}
		}
	}

	data, marshalErr := yaml.Marshal(manifest)
	if marshalErr != nil {
		logger.Warn("finalize-tier: failed to marshal manifest", "error", marshalErr)
		result.StateAdvanced = false
		return result, nil
	}
	if writeErr := os.WriteFile(opts.ManifestPath, data, 0644); writeErr != nil {
		logger.Warn("finalize-tier: failed to write manifest", "error", writeErr)
		result.StateAdvanced = false
		return result, nil
	}

	if addErr := git.Add(opts.RepoDir, opts.ManifestPath); addErr != nil {
		logger.Warn("finalize-tier: git add manifest failed", "error", addErr)
	}

	commitMsg := fmt.Sprintf("chore: finalize tier %d", opts.TierNumber)
	if _, commitErr := git.CommitWithMessage(opts.RepoDir, commitMsg); commitErr != nil {
		logger.Warn("finalize-tier: git commit failed", "error", commitErr)
		// non-fatal
	} else {
		result.StateAdvanced = true
	}

	return result, nil
}

// mergeIMPLBranchWorktreeAware merges branch into HEAD of repoDir.
// If the branch is currently checked out in a worktree, it:
//  1. Finds the worktree path via git.WorktreeList.
//  2. Resolves the commit SHA of the worktree HEAD using git.RevParse.
//  3. Merges by SHA from the main repo (git merge --no-ff <sha> -m <msg>),
//     which avoids the "already checked out" error.
//
// If the branch is not in any worktree, uses git.MergeNoFF directly.
func mergeIMPLBranchWorktreeAware(repoDir, branch, message string) error {
	worktrees, err := git.WorktreeList(repoDir)
	if err != nil {
		// If we can't list worktrees, fall back to direct merge
		return git.MergeNoFF(repoDir, branch, message)
	}

	for _, entry := range worktrees {
		if entry[1] == branch {
			// Branch is checked out in a worktree — resolve SHA
			sha, revErr := git.RevParse(entry[0], "HEAD")
			if revErr != nil {
				// Fall back to SHA from main repo
				sha, revErr = git.RevParse(repoDir, branch)
				if revErr != nil {
					return revErr
				}
			}
			// Merge by SHA to avoid "already checked out" error
			return git.MergeNoFF(repoDir, sha, message)
		}
	}

	// Branch not in any worktree — merge directly
	return git.MergeNoFF(repoDir, branch, message)
}
