package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// TierAdvanceResult is returned by AdvanceTierAutomatically.
// It captures tier gate status, contract freeze status, and the decision
// on whether to advance to the next tier or enter human review.
type TierAdvanceResult struct {
	TierNumber      int                            `json:"tier_number"`
	GateResult      *protocol.TierGateData          `json:"gate_result"`
	FreezeResult    *protocol.FreezeContractsData   `json:"freeze_result,omitempty"`
	AdvancedToNext  bool                           `json:"advanced_to_next"`
	RequiresReview  bool                           `json:"requires_review"`
	NextTier        int                            `json:"next_tier,omitempty"`
	ProgramComplete bool                           `json:"program_complete"`
	Errors          []string                       `json:"errors,omitempty"`
	// ScoredIMPLOrder is the priority-ordered list of IMPL slugs for the next tier.
	// Populated when AdvancedToNext=true so callers can display launch order
	// before execution starts. The caller is responsible for honoring ConcurrencyCap.
	ScoredIMPLOrder []string `json:"scored_impl_order,omitempty"`
}

// ReplanProgramOpts configures a Planner re-engagement to revise a PROGRAM manifest.
type ReplanProgramOpts struct {
	ProgramManifestPath string // path to existing PROGRAM manifest to revise
	Reason              string // why re-planning was triggered
	FailedTier          int    // tier number that failed (0 if user-initiated)
	PlannerModel        string // optional: model override for Planner agent
}

// ReplanResult is returned by ReplanProgram.
type ReplanResult struct {
	RevisedManifestPath string   `json:"revised_manifest_path"`
	ValidationPassed    bool     `json:"validation_passed"`
	ChangesSummary      string   `json:"changes_summary,omitempty"`
	PlannerAgentID      string   `json:"planner_agent_id"`
	Errors              []string `json:"errors,omitempty"`
}

// AdvanceTierAutomatically checks tier gate, freezes contracts, and advances
// to next tier if --auto mode is active. Returns result.Result[TierAdvanceResult].
//
// Logic:
//  1. Run RunTierGate to verify tier completion.
//  2. If gate fails: RequiresReview=true, AdvancedToNext=false.
//  3. If gate passes and autoMode=false: RequiresReview=true (human gate).
//  4. If gate passes and autoMode=true: freeze contracts, then either mark
//     ProgramComplete (final tier) or set AdvancedToNext=true, NextTier=completedTier+1.
func AdvanceTierAutomatically(manifest *protocol.PROGRAMManifest, completedTier int, repoPath string, autoMode bool) result.Result[TierAdvanceResult] {
	data := TierAdvanceResult{
		TierNumber: completedTier,
	}

	// Step 1: Run tier gate
	gateRes := protocol.RunTierGate(context.Background(), manifest, completedTier, repoPath)
	if gateRes.IsFatal() {
		return result.NewFailure[TierAdvanceResult]([]result.SAWError{
			result.NewFatal(result.CodeTierGateFailed,
				fmt.Sprintf("AdvanceTierAutomatically: run tier gate: %s", gateRes.Errors[0].Message)).
				WithContext("tier", fmt.Sprintf("%d", completedTier)),
		})
	}
	gateResult := gateRes.GetData()
	data.GateResult = gateResult

	// Step 2: Gate failed — requires human review
	if !gateResult.Passed {
		data.RequiresReview = true
		data.AdvancedToNext = false
		return result.NewSuccess(data)
	}

	// Step 3: Gate passed but auto mode is off — defer to human
	if !autoMode {
		data.RequiresReview = true
		data.AdvancedToNext = false
		return result.NewSuccess(data)
	}

	// Step 4: Auto mode — freeze contracts
	freezeRes := protocol.FreezeContracts(manifest, completedTier, repoPath)
	if freezeRes.IsFatal() {
		return result.NewFailure[TierAdvanceResult]([]result.SAWError{
			result.NewFatal(result.CodeFreezeError,
				fmt.Sprintf("AdvanceTierAutomatically: freeze contracts: %s", freezeRes.Errors[0].Message)).
				WithContext("tier", fmt.Sprintf("%d", completedTier)),
		})
	}
	freezeResult := freezeRes.GetData()
	data.FreezeResult = freezeResult

	if !freezeResult.Success {
		data.RequiresReview = true
		data.AdvancedToNext = false
		data.Errors = append(data.Errors, freezeResult.Errors...)
		return result.NewSuccess(data)
	}

	// Determine if this was the final tier
	finalTier := isFinalTier(manifest, completedTier)
	if finalTier {
		data.ProgramComplete = true
		data.AdvancedToNext = false
	} else {
		data.AdvancedToNext = true
		data.NextTier = completedTier + 1
		// Score the next tier's pending IMPLs so callers can display priority
		// order before launching. The caller is responsible for honoring ConcurrencyCap.
		data.ScoredIMPLOrder = ScoreTierIMPLs(manifest, data.NextTier)
	}

	return result.NewSuccess(data)
}

// ScoreTierIMPLs scores all pending IMPLs in tierNumber and returns them sorted by
// descending priority. Also updates PriorityScore and PriorityReasoning on each
// matching ProgramIMPL entry in manifest.Impls (mutates in place).
//
// Returns nil if the tier is not found. Returns slugs in priority order (highest first).
//
// NOTE: The caller is responsible for honoring ConcurrencyCap on the tier.
// ScoreTierIMPLs returns the full priority-ordered list; the caller should launch
// only the first ConcurrencyCap slugs (or all if ConcurrencyCap == 0).
func ScoreTierIMPLs(manifest *protocol.PROGRAMManifest, tierNumber int) []string {
	// Find the tier
	var tier *protocol.ProgramTier
	for i := range manifest.Tiers {
		if manifest.Tiers[i].Number == tierNumber {
			tier = &manifest.Tiers[i]
			break
		}
	}
	if tier == nil {
		return nil
	}

	// Collect pending IMPL slugs from the tier
	var pendingSlugs []string
	for _, slug := range tier.Impls {
		for _, impl := range manifest.Impls {
			if impl.Slug == slug && impl.Status == "pending" {
				pendingSlugs = append(pendingSlugs, slug)
				break
			}
		}
	}

	// Score each pending IMPL and write back into manifest.Impls
	for i := range manifest.Impls {
		impl := &manifest.Impls[i]
		// Only update pending IMPLs in this tier
		if impl.Tier != tierNumber || impl.Status != "pending" {
			continue
		}
		score := protocol.UnblockingScore(manifest, impl.Slug)
		impl.PriorityScore = score
		if score > 0 {
			// unblocking_potential = score / UnblockBonusPerIMPL (age_bonus is 0)
			potential := score / protocol.UnblockBonusPerIMPL
			impl.PriorityReasoning = fmt.Sprintf("unblocking(%dx+100=+%d), age(+0)", potential, score)
		} else {
			impl.PriorityReasoning = "unblocking(0), age(+0)"
		}
	}

	// Return priority-ordered list
	return protocol.PrioritizeIMPLs(manifest, pendingSlugs)
}

// isFinalTier returns true if completedTier is the highest-numbered tier in the manifest.
func isFinalTier(manifest *protocol.PROGRAMManifest, tierNumber int) bool {
	maxTier := 0
	for _, t := range manifest.Tiers {
		if t.Number > maxTier {
			maxTier = t.Number
		}
	}
	return tierNumber >= maxTier
}

// ReplanProgram launches the Planner agent to revise a PROGRAM manifest based on
// execution feedback (tier gate failure, blocked IMPL, etc.).
func ReplanProgram(opts ReplanProgramOpts) result.Result[ReplanResult] {
	// Step 1: Read existing PROGRAM manifest
	manifestData, err := os.ReadFile(opts.ProgramManifestPath)
	if err != nil {
		return result.NewFailure[ReplanResult]([]result.SAWError{
			result.NewFatal(result.CodeIMPLParseFailed,
				fmt.Sprintf("ReplanProgram: read manifest: %s", err)).
				WithContext("manifest_path", opts.ProgramManifestPath),
		})
	}

	// Step 2: Construct revision prompt
	revisionPrompt := buildRevisionPrompt(opts, string(manifestData))

	// Step 3: Derive repo path — manifest lives at <repo>/docs/PROGRAM/PROGRAM-*.yaml
	repoPath := filepath.Dir(filepath.Dir(filepath.Dir(opts.ProgramManifestPath)))

	// Step 4: Launch Planner agent with the revision prompt overwriting the manifest in place
	var chunks []string
	plannerRes := RunPlanner(context.Background(), RunPlannerOpts{
		Description:    revisionPrompt,
		RepoPath:       repoPath,
		ProgramOutPath: opts.ProgramManifestPath,
		PlannerModel:   opts.PlannerModel,
	}, func(chunk string) { chunks = append(chunks, chunk) })
	if plannerRes.IsFatal() {
		msg := "ReplanProgram: planner agent failed"
		if len(plannerRes.Errors) > 0 {
			// Propagate the structured SAWError code upward rather than wrapping
			return result.NewFailure[ReplanResult]([]result.SAWError{
				result.NewFatal(plannerRes.Errors[0].Code,
					fmt.Sprintf("ReplanProgram: planner agent: %s", plannerRes.Errors[0].Message)),
			})
		}
		return result.NewFailure[ReplanResult]([]result.SAWError{
			result.NewFatal(result.CodePlannerFailed, msg),
		})
	}

	// Step 5: Validate revised manifest
	_, parseErr := protocol.ParseProgramManifest(opts.ProgramManifestPath)
	data := ReplanResult{
		RevisedManifestPath: opts.ProgramManifestPath,
		ValidationPassed:    parseErr == nil,
		ChangesSummary:      strings.Join(chunks, ""),
	}
	if parseErr != nil {
		data.Errors = append(data.Errors, parseErr.Error())
		// Validation failure is a partial success: replan ran but manifest is invalid.
		return result.NewPartial(data, []result.SAWError{
			result.NewError(result.CodeIMPLParseFailed,
				fmt.Sprintf("ReplanProgram: revised manifest parse failed: %s", parseErr)).
				WithContext("manifest_path", opts.ProgramManifestPath),
		})
	}
	return result.NewSuccess(data)
}

// buildRevisionPrompt constructs the revision prompt for the Planner agent.
func buildRevisionPrompt(opts ReplanProgramOpts, manifestContent string) string {
	prompt := "You are re-engaging to revise this PROGRAM manifest.\n"
	prompt += fmt.Sprintf("Reason: %s\n", opts.Reason)
	if opts.FailedTier != 0 {
		prompt += fmt.Sprintf("Failed tier: %d\n", opts.FailedTier)
	}
	prompt += "\nCurrent manifest:\n"
	prompt += manifestContent
	prompt += "\nRevise the program contracts or tier structure to address the failure."
	prompt += "\nDo NOT modify completed tiers or frozen contracts."
	return prompt
}
