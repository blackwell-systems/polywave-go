package protocol

import (
	"fmt"
)

// PrepareTierResult contains the structured output of preparing a program tier.
// It includes per-step results for conflict checking, IMPL validation, and
// worktree creation.
type PrepareTierResult struct {
	Tier          int                    `json:"tier"`
	ConflictCheck *ConflictCheckResult   `json:"conflict_check"`
	Validations   []IMPLValidationResult `json:"validations"`
	Branches      []ProgramWorktreeInfo  `json:"branches"`
	Success       bool                   `json:"success"`
}

// ConflictCheckResult contains the outcome of cross-IMPL file ownership analysis.
type ConflictCheckResult struct {
	Conflicts []IMPLFileConflict `json:"conflicts"`
	Disjoint  bool               `json:"disjoint"`
}

// IMPLValidationResult contains the validation outcome for a single IMPL doc.
type IMPLValidationResult struct {
	ImplSlug string   `json:"impl_slug"`
	Valid    bool     `json:"valid"`
	Fixed    int      `json:"fixed"`
	Errors   []string `json:"errors,omitempty"`
}

// PrepareTier is an atomic batching function that combines conflict checking,
// IMPL validation, and worktree creation for a program tier. It aborts on any
// failure and returns a partial result with Success=false.
//
// Steps:
//  1. Parse program manifest.
//  2. Find tier by number.
//  3. Check for cross-IMPL file ownership conflicts.
//  4. Validate each IMPL doc (with auto-fix of gate types).
//  5. Create worktrees for the tier.
func PrepareTier(programManifestPath string, tierNumber int, repoDir string) (*PrepareTierResult, error) {
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

	result := &PrepareTierResult{
		Tier: tierNumber,
	}

	// Step 3: Conflict check.
	report, err := CheckIMPLConflicts(targetTier.Impls, repoDir)
	if err != nil {
		return nil, fmt.Errorf("conflict check failed: %w", err)
	}
	result.ConflictCheck = &ConflictCheckResult{
		Conflicts: report.Conflicts,
		Disjoint:  len(report.Conflicts) == 0,
	}
	if len(report.Conflicts) > 0 {
		result.Success = false
		return result, nil
	}

	// Step 4: IMPL validation.
	for _, slug := range targetTier.Impls {
		implPath, err := ResolveIMPLPath(repoDir, slug)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve IMPL %q: %w", slug, err)
		}

		m, err := Load(implPath)
		if err != nil {
			return nil, fmt.Errorf("cannot load IMPL %q: %w", slug, err)
		}

		fixCount := FixGateTypes(m)
		if fixCount > 0 {
			if saveErr := Save(m, implPath); saveErr != nil {
				return nil, fmt.Errorf("cannot save IMPL %q after fixes: %w", slug, saveErr)
			}
		}

		valErrors := Validate(m)
		vr := IMPLValidationResult{
			ImplSlug: slug,
			Valid:    len(valErrors) == 0,
			Fixed:    fixCount,
		}
		for _, ve := range valErrors {
			vr.Errors = append(vr.Errors, ve.Message)
		}
		result.Validations = append(result.Validations, vr)

		if !vr.Valid {
			result.Success = false
			return result, nil
		}

		// Step 4.5: E37 critic gate enforcement (auto mode for program execution).
		if !CriticGatePasses(m, true) {
			vr := IMPLValidationResult{
				ImplSlug: slug,
				Valid:    false,
				Errors:   []string{"E37 critic gate failed — ISSUES verdict with errors"},
			}
			result.Validations = append(result.Validations, vr)
			result.Success = false
			return result, nil
		}
	}

	// Step 5: Create worktrees.
	wtResult := CreateProgramWorktrees(programManifestPath, tierNumber, repoDir)
	if !wtResult.IsSuccess() {
		result.Success = false
		result.Branches = []ProgramWorktreeInfo{}
		return result, nil
	}
	wtData := wtResult.GetData()
	result.Branches = wtData.Worktrees
	result.Success = true

	return result, nil
}
