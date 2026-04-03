package protocol

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// FinalizeTierData is the data payload of finalizing a program tier.
// It contains per-IMPL merge outcomes and the tier gate result.
// Use result.Result[FinalizeTierData] to check success and access errors.
type FinalizeTierData struct {
	TierNumber       int                           `json:"tier_number"`
	ImplMergeResults map[string]*MergeAgentsData `json:"impl_merge_results"` // impl_slug -> result
	TierGateResult   *TierGateData              `json:"tier_gate_result,omitempty"`
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
//  5. Return result.NewSuccess if all merges succeeded AND tier gate passed;
//     result.NewFailure otherwise.
//
// Stops on the first merge failure and does not run the tier gate.
func FinalizeTier(programManifestPath string, tierNumber int, repoDir string) (result.Result[FinalizeTierData], error) {
	manifest, err := ParseProgramManifest(programManifestPath)
	if err != nil {
		return result.Result[FinalizeTierData]{}, fmt.Errorf("failed to parse program manifest: %w", err)
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
		return result.Result[FinalizeTierData]{}, fmt.Errorf("tier %d not found in program manifest", tierNumber)
	}

	data := FinalizeTierData{
		TierNumber:       tierNumber,
		ImplMergeResults: make(map[string]*MergeAgentsData),
	}

	// Merge each IMPL branch in order.
	for _, implSlug := range targetTier.Impls {
		branch := ProgramBranchName(manifest.ProgramSlug, tierNumber, implSlug)

		// Idempotent: skip if branch doesn't exist (already merged and cleaned up).
		if !git.BranchExists(repoDir, branch) {
			fmt.Printf("branch %q not found, skipping (already merged or not yet created)\n", branch)
			data.ImplMergeResults[implSlug] = &MergeAgentsData{
				Wave:   tierNumber,
				Merges: []MergeStatus{{Agent: implSlug, Branch: branch, Success: true, Error: "branch absent (skipped)"}},
			}
			continue
		}

		message := fmt.Sprintf("Merge program tier %d impl %s: %s", tierNumber, implSlug, branch)
		mergeErr := git.MergeNoFF(repoDir, branch, message)

		mergeResult := &MergeAgentsData{
			Wave: tierNumber,
		}

		if mergeErr != nil {
			mergeResult.Merges = []MergeStatus{{
				Agent:   implSlug,
				Branch:  branch,
				Success: false,
				Error:   mergeErr.Error(),
			}}
			data.ImplMergeResults[implSlug] = mergeResult
			data.Errors = append(data.Errors, fmt.Sprintf("merge failed for impl %s: %v", implSlug, mergeErr))
			return result.NewFailure[FinalizeTierData]([]result.SAWError{{
				Code:     result.CodeMergeConflict,
				Message:  fmt.Sprintf("merge failed for impl %s: %v", implSlug, mergeErr),
				Severity: "fatal",
			}}), nil
		}

		mergeResult.Merges = []MergeStatus{{
			Agent:   implSlug,
			Branch:  branch,
			Success: true,
		}}
		data.ImplMergeResults[implSlug] = mergeResult
	}

	// All merges succeeded; run the tier gate.
	gateRes := RunTierGate(context.Background(), manifest, tierNumber, repoDir)
	if !gateRes.IsSuccess() {
		errMsg := fmt.Sprintf("tier gate error for tier %d", tierNumber)
		if len(gateRes.Errors) > 0 {
			errMsg = gateRes.Errors[0].Message
		}
		data.Errors = append(data.Errors, errMsg)
		return result.NewFailure[FinalizeTierData]([]result.SAWError{{
			Code:     result.CodeTierGateFailed,
			Message:  errMsg,
			Severity: "fatal",
		}}), nil
	}
	gateData := gateRes.GetData()
	data.TierGateResult = gateData

	if !gateData.Passed {
		data.Errors = append(data.Errors, fmt.Sprintf("tier gate failed for tier %d", tierNumber))
		return result.NewFailure[FinalizeTierData]([]result.SAWError{{
			Code:     result.CodeTierGateFailed,
			Message:  fmt.Sprintf("tier gate failed for tier %d", tierNumber),
			Severity: "fatal",
		}}), nil
	}

	return result.NewSuccess(data), nil
}
