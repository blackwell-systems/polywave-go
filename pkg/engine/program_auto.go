package engine

import (
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TierAdvanceResult is returned by AdvanceTierAutomatically.
// It captures tier gate status, contract freeze status, and the decision
// on whether to advance to the next tier or enter human review.
type TierAdvanceResult struct {
	TierNumber      int                            `json:"tier_number"`
	GateResult      *protocol.TierGateResult       `json:"gate_result"`
	FreezeResult    *protocol.FreezeContractsResult `json:"freeze_result,omitempty"`
	AdvancedToNext  bool                           `json:"advanced_to_next"`
	RequiresReview  bool                           `json:"requires_review"`
	NextTier        int                            `json:"next_tier,omitempty"`
	ProgramComplete bool                           `json:"program_complete"`
	Errors          []string                       `json:"errors,omitempty"`
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
// to next tier if --auto mode is active. Returns true if advanced, false if
// human review required.
//
// Logic:
//  1. Run RunTierGate to verify tier completion.
//  2. If gate fails: RequiresReview=true, AdvancedToNext=false.
//  3. If gate passes and autoMode=false: RequiresReview=true (human gate).
//  4. If gate passes and autoMode=true: freeze contracts, then either mark
//     ProgramComplete (final tier) or set AdvancedToNext=true, NextTier=completedTier+1.
func AdvanceTierAutomatically(manifest *protocol.PROGRAMManifest, completedTier int, repoPath string, autoMode bool) (*TierAdvanceResult, error) {
	result := &TierAdvanceResult{
		TierNumber: completedTier,
	}

	// Step 1: Run tier gate
	gateResult, err := protocol.RunTierGate(manifest, completedTier, repoPath)
	if err != nil {
		return nil, fmt.Errorf("AdvanceTierAutomatically: run tier gate: %w", err)
	}
	result.GateResult = gateResult

	// Step 2: Gate failed — requires human review
	if !gateResult.Passed {
		result.RequiresReview = true
		result.AdvancedToNext = false
		return result, nil
	}

	// Step 3: Gate passed but auto mode is off — defer to human
	if !autoMode {
		result.RequiresReview = true
		result.AdvancedToNext = false
		return result, nil
	}

	// Step 4: Auto mode — freeze contracts
	freezeResult, err := protocol.FreezeContracts(manifest, completedTier, repoPath)
	if err != nil {
		return nil, fmt.Errorf("AdvanceTierAutomatically: freeze contracts: %w", err)
	}
	result.FreezeResult = freezeResult

	if !freezeResult.Success {
		result.RequiresReview = true
		result.AdvancedToNext = false
		result.Errors = append(result.Errors, freezeResult.Errors...)
		return result, nil
	}

	// Determine if this was the final tier
	finalTier := isFinalTier(manifest, completedTier)
	if finalTier {
		result.ProgramComplete = true
		result.AdvancedToNext = false
	} else {
		result.AdvancedToNext = true
		result.NextTier = completedTier + 1
	}

	return result, nil
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
//
// NOTE: Planner agent launch is not yet implemented. This function returns
// "not yet implemented" until Wave 3 wires the saw-skill.md command.
// Tests should mock Planner completion by writing a revised manifest file directly.
func ReplanProgram(opts ReplanProgramOpts) (*ReplanResult, error) {
	// Step 1: Read existing PROGRAM manifest
	manifestData, err := os.ReadFile(opts.ProgramManifestPath)
	if err != nil {
		return nil, fmt.Errorf("ReplanProgram: read manifest: %w", err)
	}

	// Step 2: Construct revision prompt
	prompt := buildRevisionPrompt(opts, string(manifestData))
	_ = prompt // used by Planner agent launch (below)

	// Step 3: Launch Planner agent
	// TODO(Wave3): Launch Planner agent via Agent SDK with revision prompt.
	// See saw-skill.md `/saw program replan` for integration details.
	// Full implementation happens when the CLI command is wired in Wave 3.
	return nil, fmt.Errorf("ReplanProgram: not yet implemented")
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
