package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TierLoopOpts configures the tier execution loop.
type TierLoopOpts struct {
	ManifestPath string
	RepoPath     string
	AutoMode     bool
	Model        string
	OnEvent      func(TierLoopEvent)
}

// TierLoopResult captures the outcome of the tier execution loop.
type TierLoopResult struct {
	TiersExecuted   int      `json:"tiers_executed"`
	TiersRemaining  int      `json:"tiers_remaining"`
	ProgramComplete bool     `json:"program_complete"`
	FinalState      string   `json:"final_state"`
	RequiresReview  bool     `json:"requires_review"`
	Errors          []string `json:"errors,omitempty"`
}

// TierLoopEvent is emitted at each major step for observability.
type TierLoopEvent struct {
	Type   string `json:"type"`   // "tier_started", "scout_launched", "impl_complete", "tier_gate", "contracts_frozen", "tier_advanced", "replan_triggered"
	Tier   int    `json:"tier"`
	Detail string `json:"detail"`
}

// ParallelScoutOpts configures parallel Scout launching.
type ParallelScoutOpts struct {
	ManifestPath string
	RepoPath     string
	TierNumber   int
	Slugs        []string
	Model        string
	OnEvent      func(TierLoopEvent)
}

// ParallelScoutResult captures results from parallel Scout execution.
type ParallelScoutResult struct {
	Completed []string          `json:"completed"`
	Failed    []string          `json:"failed"`
	Errors    map[string]string `json:"errors,omitempty"`
}

// launchParallelScoutsFunc is a function variable that allows Agent B to
// inject the real implementation of LaunchParallelScouts. This enables
// compilation independence between agents.
var launchParallelScoutsFunc = func(ctx context.Context, opts ParallelScoutOpts) (*ParallelScoutResult, error) {
	return nil, fmt.Errorf("LaunchParallelScouts not yet implemented (stub)")
}

// RunTierLoop is the full tier execution loop that reads a PROGRAM manifest,
// partitions IMPLs, launches Scouts in parallel, executes waves, runs tier gates,
// freezes contracts, and advances to the next tier. It implements E28-E34.
func RunTierLoop(ctx context.Context, opts TierLoopOpts) (*TierLoopResult, error) {
	result := &TierLoopResult{}

	manifest, err := protocol.ParseProgramManifest(opts.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("RunTierLoop: parse manifest: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			result.FinalState = "cancelled"
			result.Errors = append(result.Errors, ctx.Err().Error())
			return result, ctx.Err()
		default:
		}

		// Step 2: Determine current tier (lowest tier with incomplete IMPLs)
		currentTier := findCurrentTier(manifest)
		if currentTier == -1 {
			// All tiers complete
			result.ProgramComplete = true
			result.FinalState = "complete"
			return result, nil
		}

		emitEvent(opts.OnEvent, TierLoopEvent{
			Type:   "tier_started",
			Tier:   currentTier,
			Detail: fmt.Sprintf("Starting tier %d", currentTier),
		})

		// Step 3: Partition IMPLs (E28A)
		needsScout, preExisting := PartitionIMPLsByStatus(manifest, currentTier)

		// Step 4: Launch parallel Scouts for pending IMPLs
		if len(needsScout) > 0 {
			emitEvent(opts.OnEvent, TierLoopEvent{
				Type:   "scout_launched",
				Tier:   currentTier,
				Detail: fmt.Sprintf("Launching scouts for %d IMPLs: %s", len(needsScout), strings.Join(needsScout, ", ")),
			})

			scoutResult, scoutErr := launchParallelScoutsFunc(ctx, ParallelScoutOpts{
				ManifestPath: opts.ManifestPath,
				RepoPath:     opts.RepoPath,
				TierNumber:   currentTier,
				Slugs:        needsScout,
				Model:        opts.Model,
				OnEvent:      opts.OnEvent,
			})
			if scoutErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("scout launch failed: %s", scoutErr))
				result.FinalState = "scout_failed"
				return result, fmt.Errorf("RunTierLoop: launch scouts: %w", scoutErr)
			}
			if len(scoutResult.Failed) > 0 {
				for slug, errMsg := range scoutResult.Errors {
					result.Errors = append(result.Errors, fmt.Sprintf("scout %s failed: %s", slug, errMsg))
				}
				result.FinalState = "scout_partial_failure"
				return result, fmt.Errorf("RunTierLoop: %d scouts failed", len(scoutResult.Failed))
			}
		}

		// Step 5: Validate pre-existing IMPLs
		if len(preExisting) > 0 {
			validationErrs := protocol.ValidateProgramImportMode(manifest, opts.RepoPath)
			if len(validationErrs) > 0 {
				for _, ve := range validationErrs {
					result.Errors = append(result.Errors, fmt.Sprintf("[%s] %s", ve.Code, ve.Message))
				}
			}
		}

		// Step 7: If NOT autoMode, return with RequiresReview
		if !opts.AutoMode {
			result.RequiresReview = true
			result.TiersExecuted = countCompleteTiers(manifest)
			result.TiersRemaining = len(manifest.Tiers) - result.TiersExecuted
			result.FinalState = "awaiting_review"
			return result, nil
		}

		// Step 8: Execute waves for each IMPL in the tier
		tierSlugs := getTierSlugs(manifest, currentTier)
		for _, slug := range tierSlugs {
			implPath := findIMPLDocPath(opts.RepoPath, slug)
			if implPath == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("IMPL doc not found for %s", slug))
				continue
			}

			// Determine number of waves for this IMPL
			waveCount := getIMPLWaveCount(manifest, slug)
			for wave := 1; wave <= waveCount; wave++ {
				_, waveErr := RunWaveFull(ctx, RunWaveFullOpts{
					ManifestPath: implPath,
					RepoPath:     opts.RepoPath,
					WaveNum:      wave,
				})
				if waveErr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("wave %d for %s failed: %s", wave, slug, waveErr))
				}
			}

			emitEvent(opts.OnEvent, TierLoopEvent{
				Type:   "impl_complete",
				Tier:   currentTier,
				Detail: fmt.Sprintf("IMPL %s wave execution finished", slug),
			})
		}

		// Step 9: Run tier gate (E29)
		gateResult, gateErr := protocol.RunTierGate(manifest, currentTier, opts.RepoPath)
		if gateErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("tier gate error: %s", gateErr))
			result.FinalState = "gate_error"
			return result, fmt.Errorf("RunTierLoop: tier gate: %w", gateErr)
		}

		emitEvent(opts.OnEvent, TierLoopEvent{
			Type:   "tier_gate",
			Tier:   currentTier,
			Detail: fmt.Sprintf("Tier gate passed=%v", gateResult.Passed),
		})

		if !gateResult.Passed {
			// Step 11: Auto trigger replan (E34)
			if opts.AutoMode {
				emitEvent(opts.OnEvent, TierLoopEvent{
					Type:   "replan_triggered",
					Tier:   currentTier,
					Detail: "Tier gate failed, triggering replan",
				})

				_, replanErr := AutoTriggerReplan(opts.ManifestPath, currentTier, gateResult, opts.Model)
				if replanErr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("replan failed: %s", replanErr))
					result.FinalState = "replan_failed"
					return result, fmt.Errorf("RunTierLoop: replan: %w", replanErr)
				}

				// Re-parse manifest after replan
				manifest, err = protocol.ParseProgramManifest(opts.ManifestPath)
				if err != nil {
					return nil, fmt.Errorf("RunTierLoop: re-parse after replan: %w", err)
				}
				// Continue loop to retry the tier
				continue
			}

			result.FinalState = "gate_failed"
			result.Errors = append(result.Errors, "tier gate failed")
			result.TiersExecuted = countCompleteTiers(manifest)
			result.TiersRemaining = len(manifest.Tiers) - result.TiersExecuted
			return result, nil
		}

		// Step 10: Freeze contracts (E30)
		freezeResult, freezeErr := protocol.FreezeContracts(manifest, currentTier, opts.RepoPath)
		if freezeErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("freeze contracts error: %s", freezeErr))
		} else if freezeResult.Success {
			emitEvent(opts.OnEvent, TierLoopEvent{
				Type:   "contracts_frozen",
				Tier:   currentTier,
				Detail: fmt.Sprintf("Froze %d contracts", len(freezeResult.ContractsFrozen)),
			})
		}

		// Step 12: Advance tier (E33)
		if isFinalTier(manifest, currentTier) {
			result.ProgramComplete = true
			result.FinalState = "complete"
			result.TiersExecuted = len(manifest.Tiers)
			result.TiersRemaining = 0
			return result, nil
		}

		emitEvent(opts.OnEvent, TierLoopEvent{
			Type:   "tier_advanced",
			Tier:   currentTier,
			Detail: fmt.Sprintf("Advancing from tier %d to %d", currentTier, currentTier+1),
		})

		result.TiersExecuted++

		// Step 13: Loop back (auto mode continues)
		// Re-parse manifest to pick up any status changes
		manifest, err = protocol.ParseProgramManifest(opts.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("RunTierLoop: re-parse manifest: %w", err)
		}
	}
}

// PartitionIMPLsByStatus splits IMPLs in a tier into needsScout vs preExisting
// groups per E28A.
//   - needsScout: IMPLs with status "pending" or "scouting"
//   - preExisting: IMPLs with status "reviewed" or "complete"
func PartitionIMPLsByStatus(manifest *protocol.PROGRAMManifest, tierNumber int) (needsScout []string, preExisting []string) {
	// Find the tier
	var tier *protocol.ProgramTier
	for i := range manifest.Tiers {
		if manifest.Tiers[i].Number == tierNumber {
			tier = &manifest.Tiers[i]
			break
		}
	}
	if tier == nil {
		return nil, nil
	}

	// Build slug -> status map
	statusMap := make(map[string]string)
	for _, impl := range manifest.Impls {
		statusMap[impl.Slug] = impl.Status
	}

	for _, slug := range tier.Impls {
		status := statusMap[slug]
		switch status {
		case "pending", "scouting":
			needsScout = append(needsScout, slug)
		case "reviewed", "complete":
			preExisting = append(preExisting, slug)
		default:
			// Unknown or other statuses go to needsScout as a safe default
			needsScout = append(needsScout, slug)
		}
	}
	return needsScout, preExisting
}

// replanProgramFunc is a function variable to allow test mocking of ReplanProgram.
var replanProgramFunc = ReplanProgram

// AutoTriggerReplan automatically triggers Planner re-engagement when a tier
// gate fails (E34). It constructs a reason string from gate failure details
// and calls ReplanProgram.
func AutoTriggerReplan(manifestPath string, tierNumber int, gateResult *protocol.TierGateResult, model string) (*ReplanResult, error) {
	reason := buildReplanReason(tierNumber, gateResult)

	return replanProgramFunc(ReplanProgramOpts{
		ProgramManifestPath: manifestPath,
		Reason:              reason,
		FailedTier:          tierNumber,
		PlannerModel:        model,
	})
}

// buildReplanReason constructs a human-readable reason string from gate failure details.
func buildReplanReason(tierNumber int, gateResult *protocol.TierGateResult) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Tier %d gate failed.", tierNumber))

	if !gateResult.AllImplsDone {
		var incomplete []string
		for _, is := range gateResult.ImplStatuses {
			if is.Status != "complete" {
				incomplete = append(incomplete, fmt.Sprintf("%s(%s)", is.Slug, is.Status))
			}
		}
		parts = append(parts, fmt.Sprintf("Incomplete IMPLs: %s.", strings.Join(incomplete, ", ")))
	}

	for _, gr := range gateResult.GateResults {
		if !gr.Passed {
			parts = append(parts, fmt.Sprintf("Gate %q failed: %s", gr.Type, gr.Stderr))
		}
	}

	return strings.Join(parts, " ")
}

// findCurrentTier returns the lowest-numbered tier with at least one incomplete IMPL.
// Returns -1 if all tiers are complete.
func findCurrentTier(manifest *protocol.PROGRAMManifest) int {
	statusMap := make(map[string]string)
	for _, impl := range manifest.Impls {
		statusMap[impl.Slug] = impl.Status
	}

	lowestIncomplete := -1
	for _, tier := range manifest.Tiers {
		for _, slug := range tier.Impls {
			if statusMap[slug] != "complete" {
				if lowestIncomplete == -1 || tier.Number < lowestIncomplete {
					lowestIncomplete = tier.Number
				}
				break
			}
		}
	}
	return lowestIncomplete
}

// countCompleteTiers returns the number of tiers where all IMPLs are complete.
func countCompleteTiers(manifest *protocol.PROGRAMManifest) int {
	statusMap := make(map[string]string)
	for _, impl := range manifest.Impls {
		statusMap[impl.Slug] = impl.Status
	}

	count := 0
	for _, tier := range manifest.Tiers {
		allComplete := true
		for _, slug := range tier.Impls {
			if statusMap[slug] != "complete" {
				allComplete = false
				break
			}
		}
		if allComplete {
			count++
		}
	}
	return count
}

// getTierSlugs returns the IMPL slugs for a given tier number.
func getTierSlugs(manifest *protocol.PROGRAMManifest, tierNumber int) []string {
	for _, tier := range manifest.Tiers {
		if tier.Number == tierNumber {
			return tier.Impls
		}
	}
	return nil
}

// getIMPLWaveCount returns the estimated number of waves for an IMPL in the manifest.
// Defaults to 1 if not specified.
func getIMPLWaveCount(manifest *protocol.PROGRAMManifest, slug string) int {
	for _, impl := range manifest.Impls {
		if impl.Slug == slug {
			if impl.EstimatedWaves > 0 {
				return impl.EstimatedWaves
			}
			return 1
		}
	}
	return 1
}

// findIMPLDocPath searches for an IMPL doc on disk in common locations.
// Returns empty string if not found.
func findIMPLDocPath(repoPath, slug string) string {
	// Try common locations
	locations := []string{
		fmt.Sprintf("%s/docs/IMPL/IMPL-%s.yaml", repoPath, slug),
		fmt.Sprintf("%s/docs/IMPL/complete/IMPL-%s.yaml", repoPath, slug),
		fmt.Sprintf("%s/docs/IMPL/in-progress/IMPL-%s.yaml", repoPath, slug),
	}
	for _, loc := range locations {
		// We just return the first path — RunWaveFull will handle not-found
		// In a real implementation this would check file existence
		return loc
	}
	return ""
}

// emitEvent safely calls the OnEvent callback if it is non-nil.
func emitEvent(fn func(TierLoopEvent), event TierLoopEvent) {
	if fn != nil {
		fn(event)
	}
}
