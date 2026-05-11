package protocol

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// PrepareTierOpts contains the options for PrepareTier, replacing positional
// arguments and adding a SkipCritic flag.
type PrepareTierOpts struct {
	Ctx                 context.Context // Context for Load/Save calls; nil defaults to context.Background()
	ProgramManifestPath string
	TierNumber          int
	RepoDir             string
	SkipCritic          bool // When true, auto-write synthetic PASS for IMPLs requiring E37 with no critic report

	// RunPrepareWave controls whether the caller should run prepare-wave for
	// each IMPL. When true, the CLI adapter iterates IMPLs and calls
	// engine.PrepareWave with CommitState: true between calls.
	// This field is informational for the protocol layer; actual prepare-wave
	// calls happen in the CLI adapter to avoid circular imports.
	RunPrepareWave bool
	WaveNum        int    // Wave number to pass to prepare-wave (defaults to 1)
	MergeTarget    string // Baseline branch for prepare-wave calls (empty = HEAD)
}

// PrepareTierResult contains the structured output of preparing a program tier.
// It includes per-step results for conflict checking, IMPL validation, and
// worktree creation.
type PrepareTierResult struct {
	Tier          int                    `json:"tier"`
	ConflictCheck *ConflictCheckResult   `json:"conflict_check"`
	Validations   []IMPLValidationResult `json:"validations"`
	Branches      []ProgramWorktreeInfo  `json:"branches"`
	Success       bool                   `json:"success"`

	// PrepareWaveResults holds per-IMPL prepare-wave results.
	// Non-nil only when PrepareTierOpts.RunPrepareWave is true.
	PrepareWaveResults []IMPLPrepareWaveResult `json:"prepare_wave_results,omitempty"`
}

// ConflictCheckResult contains the outcome of cross-IMPL file ownership analysis.
type ConflictCheckResult struct {
	Conflicts []IMPLFileConflict `json:"conflicts"`
	Disjoint  bool               `json:"disjoint"`
}

// IMPLPrepareWaveResult captures the result of a prepare-wave call for a
// single IMPL within a tier. Used when PrepareTierOpts.RunPrepareWave is true.
// AgentBriefs and Worktrees use protocol-level types to avoid a circular import
// with pkg/engine.
type IMPLPrepareWaveResult struct {
	ImplSlug  string         `json:"impl_slug"`
	Success   bool           `json:"success"`
	Worktrees []WorktreeInfo `json:"worktrees"`
	Error     string         `json:"error,omitempty"`
}

// IMPLValidationResult contains the validation outcome for a single IMPL doc.
type IMPLValidationResult struct {
	ImplSlug string   `json:"impl_slug"`
	Valid    bool     `json:"valid"`
	Fixed    int      `json:"fixed"`
	Errors   []string `json:"errors,omitempty"`
}

// PrepareTier is an atomic batching function that combines conflict checking,
// IMPL validation, and worktree creation for a program tier. It collects all
// failures instead of aborting on the first one, and returns a result with
// Success=false if any failure occurs.
//
// Steps:
//  1. Parse program manifest.
//  2. Find tier by number.
//  3. Check for cross-IMPL file ownership conflicts.
//  4. Validate each IMPL doc (with auto-fix of gate types).
//  5. Create worktrees for the tier.
func PrepareTier(opts PrepareTierOpts) result.Result[*PrepareTierResult] {
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	manifest, err := ParseProgramManifest(opts.ProgramManifestPath)
	if err != nil {
		return result.NewFailure[*PrepareTierResult]([]result.PolywaveError{{
			Code:     result.CodeParseError,
			Message:  fmt.Sprintf("failed to parse program manifest: %v", err),
			Severity: "fatal",
		}})
	}

	// Find the tier by number.
	var targetTier *ProgramTier
	for i := range manifest.Tiers {
		if manifest.Tiers[i].Number == opts.TierNumber {
			targetTier = &manifest.Tiers[i]
			break
		}
	}
	if targetTier == nil {
		return result.NewFailure[*PrepareTierResult]([]result.PolywaveError{{
			Code:     result.CodeParseError,
			Message:  fmt.Sprintf("tier %d not found in program manifest", opts.TierNumber),
			Severity: "fatal",
		}})
	}

	res := &PrepareTierResult{
		Tier: opts.TierNumber,
	}

	// Step 3: Conflict check.
	report, err := CheckIMPLConflicts(targetTier.Impls, opts.RepoDir)
	if err != nil {
		return result.NewFailure[*PrepareTierResult]([]result.PolywaveError{{
			Code:     result.CodeParseError,
			Message:  fmt.Sprintf("conflict check failed: %v", err),
			Severity: "fatal",
		}})
	}
	res.ConflictCheck = &ConflictCheckResult{
		Conflicts: report.Conflicts,
		Disjoint:  len(report.Conflicts) == 0,
	}
	if len(report.Conflicts) > 0 {
		res.Success = false
		return result.NewSuccess(res)
	}

	// Step 4: IMPL validation — collect all failures instead of returning early.
	hasFailure := false
	for _, slug := range targetTier.Impls {
		implPath, err := ResolveIMPLPath(opts.RepoDir, slug)
		if err != nil {
			return result.NewFailure[*PrepareTierResult]([]result.PolywaveError{{
				Code:     result.CodeParseError,
				Message:  fmt.Sprintf("cannot resolve IMPL %q: %v", slug, err),
				Severity: "fatal",
			}})
		}

		m, err := Load(ctx, implPath)
		if err != nil {
			return result.NewFailure[*PrepareTierResult]([]result.PolywaveError{{
				Code:     result.CodeParseError,
				Message:  fmt.Sprintf("cannot load IMPL %q: %v", slug, err),
				Severity: "fatal",
			}})
		}

		fixCount := FixGateTypes(m)
		if fixCount > 0 {
			if saveRes := Save(ctx, m, implPath); saveRes.IsFatal() {
				saveMsg := ""
				if len(saveRes.Errors) > 0 {
					saveMsg = saveRes.Errors[0].Message
				}
				return result.NewFailure[*PrepareTierResult]([]result.PolywaveError{{
					Code:     result.CodeParseError,
					Message:  fmt.Sprintf("cannot save IMPL %q after fixes: %s", slug, saveMsg),
					Severity: "fatal",
				}})
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
		res.Validations = append(res.Validations, vr)

		if !vr.Valid {
			hasFailure = true
			continue
		}

		// Step 4.5: E37 critic gate enforcement (auto mode for program execution).
		// Only enforce when the threshold is met: 3+ agents in wave 1 OR 2+ repos.
		if E37Required(m) && !CriticGatePasses(m, true) {
			if opts.SkipCritic {
				skipRes := SkipCriticForIMPL(ctx, implPath, m)
				if skipRes.IsFatal() {
					skipMsg := ""
					if len(skipRes.Errors) > 0 {
						skipMsg = skipRes.Errors[0].Message
					}
					e37vr := IMPLValidationResult{
						ImplSlug: slug,
						Valid:    false,
						Errors:   []string{fmt.Sprintf("E37 critic gate: failed to write synthetic PASS: %v", skipMsg)},
					}
					res.Validations = append(res.Validations, e37vr)
					hasFailure = true
				}
				// If skipped successfully, continue without failure.
			} else {
				e37vr := IMPLValidationResult{
					ImplSlug: slug,
					Valid:    false,
					Errors:   []string{"E37 critic gate required but not satisfied — run `polywave-tools run-critic` or `polywave-tools run-critic --skip` before prepare-tier"},
				}
				res.Validations = append(res.Validations, e37vr)
				hasFailure = true
			}
		}
	}

	if hasFailure {
		res.Success = false
		return result.NewSuccess(res)
	}

	// Step 5: Create worktrees.
	wtResult := CreateProgramWorktrees(opts.ProgramManifestPath, opts.TierNumber, opts.RepoDir, nil)
	if !wtResult.IsSuccess() {
		res.Success = false
		res.Branches = []ProgramWorktreeInfo{}
		return result.NewSuccess(res)
	}
	wtData := wtResult.GetData()
	res.Branches = wtData.Worktrees
	res.Success = true

	return result.NewSuccess(res)
}
