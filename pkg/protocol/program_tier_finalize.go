package protocol

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// FinalizeTierResult is the result of finalizing a program tier.
// It contains per-IMPL merge outcomes and the tier gate result.
// Success is true only if all IMPL merges succeeded and the tier gate passed.
type FinalizeTierResult struct {
	TierNumber       int                           `json:"tier_number"`
	ImplMergeResults map[string]*MergeAgentsResult `json:"impl_merge_results"` // impl_slug -> result
	TierGateResult   *TierGateResult               `json:"tier_gate_result"`
	Success          bool                          `json:"success"`
	Errors           []string                      `json:"errors,omitempty"`
}

// FinalizeTier finalizes a program tier by merging all IMPL branches to main in
// sequence, then running the tier gate. It is analogous to finalize-wave but
// operates at the program tier level rather than at the wave level.
//
// Steps:
//  1. Parse the PROGRAM manifest at programManifestPath.
//  2. Find the tier by tierNumber.
//  3. For each impl slug in tier.Impls (in order):
//     a. Compute branch name via ProgramBranchName.
//     b. If the branch does not exist, skip (idempotent).
//     c. Merge the IMPL branch into HEAD using git.MergeNoFF.
//     d. Record the merge result in ImplMergeResults.
//  4. After all merges succeed, run RunTierGate.
//  5. Set Success = true only if all merges succeeded AND tier gate passed.
//
// Stops on the first merge failure and does not run the tier gate.
func FinalizeTier(programManifestPath string, tierNumber int, repoDir string) (*FinalizeTierResult, error) {
	manifest, err := ParseProgramManifest(programManifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse program manifest: %w", err)
	}

	// Find the tier by number.
	var targetTier *ProgramTier
	for i := range manifest.Tiers {
		if manifest.Tiers[i].Number == tierNumber {
			targetTier = &manifest.Tiers[i]
			break
		}
	}
	if targetTier == nil {
		return nil, fmt.Errorf("tier %d not found in program manifest", tierNumber)
	}

	result := &FinalizeTierResult{
		TierNumber:       tierNumber,
		ImplMergeResults: make(map[string]*MergeAgentsResult),
		Success:          false,
	}

	// Merge each IMPL branch in order.
	for _, implSlug := range targetTier.Impls {
		branch := ProgramBranchName(manifest.ProgramSlug, tierNumber, implSlug)

		// Idempotent: skip if branch doesn't exist (already merged and cleaned up).
		if !git.BranchExists(repoDir, branch) {
			fmt.Printf("branch %q not found, skipping (already merged or not yet created)\n", branch)
			result.ImplMergeResults[implSlug] = &MergeAgentsResult{
				Wave:    tierNumber,
				Merges:  []MergeStatus{{Agent: implSlug, Branch: branch, Success: true, Error: "branch absent (skipped)"}},
				Success: true,
			}
			continue
		}

		message := fmt.Sprintf("Merge program tier %d impl %s: %s", tierNumber, implSlug, branch)
		mergeErr := git.MergeNoFF(repoDir, branch, message)

		mergeResult := &MergeAgentsResult{
			Wave: tierNumber,
		}

		if mergeErr != nil {
			mergeResult.Merges = []MergeStatus{{
				Agent:   implSlug,
				Branch:  branch,
				Success: false,
				Error:   mergeErr.Error(),
			}}
			mergeResult.Success = false
			result.ImplMergeResults[implSlug] = mergeResult
			result.Errors = append(result.Errors, fmt.Sprintf("merge failed for impl %s: %v", implSlug, mergeErr))
			result.Success = false
			return result, nil
		}

		mergeResult.Merges = []MergeStatus{{
			Agent:   implSlug,
			Branch:  branch,
			Success: true,
		}}
		mergeResult.Success = true
		result.ImplMergeResults[implSlug] = mergeResult
	}

	// All merges succeeded; run the tier gate.
	gateResult, err := RunTierGate(manifest, tierNumber, repoDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("tier gate error: %v", err))
		result.Success = false
		return result, nil
	}
	result.TierGateResult = gateResult
	result.Success = gateResult.Passed

	if !gateResult.Passed {
		result.Errors = append(result.Errors, fmt.Sprintf("tier gate failed for tier %d", tierNumber))
	}

	return result, nil
}
