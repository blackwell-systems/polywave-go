package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// TierLoopOpts configures the tier execution loop.
type TierLoopOpts struct {
	ManifestPath string
	RepoPath     string
	AutoMode     bool
	Model        string
	OnEvent      func(TierLoopEvent)
	ObsEmitter   ObsEmitter // optional: non-blocking observability emitter
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
	Type   string `json:"type"`   // "tier_started", "scout_launched", "impl_complete", "tier_gate", "contracts_frozen", "tier_advanced", "replan_triggered", "wave_complete"
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
func RunTierLoop(ctx context.Context, opts TierLoopOpts) result.Result[TierLoopResult] {
	loopResult := TierLoopResult{}

	manifest, err := protocol.ParseProgramManifest(opts.ManifestPath)
	if err != nil {
		return result.NewFailure[TierLoopResult]([]result.SAWError{
			result.NewFatal(result.CodeIMPLParseFailed,
				fmt.Sprintf("RunTierLoop: parse manifest: %s", err)).
				WithContext("manifest_path", opts.ManifestPath),
		})
	}

	for {
		select {
		case <-ctx.Done():
			loopResult.FinalState = "cancelled"
			loopResult.Errors = append(loopResult.Errors, ctx.Err().Error())
			return result.NewFailure[TierLoopResult]([]result.SAWError{
				result.NewFatal(result.CodeContextCancelled,
					fmt.Sprintf("RunTierLoop: context cancelled: %s", ctx.Err())),
			})
		default:
		}

		// Step 2: Determine current tier (lowest tier with incomplete IMPLs)
		currentTier := findCurrentTier(manifest)
		if currentTier == -1 {
			// All tiers complete
			loopResult.ProgramComplete = true
			loopResult.FinalState = "complete"
			return result.NewSuccess(loopResult)
		}

		emitEvent(opts.OnEvent, TierLoopEvent{
			Type:   "tier_started",
			Tier:   currentTier,
			Detail: fmt.Sprintf("Starting tier %d", currentTier),
		})

		// Step 3: Partition IMPLs (E28A)
		needsScout, preExisting := PartitionIMPLsByStatus(manifest, currentTier)

		// Step 3a: P1+ conflict check — detect overlapping file ownership across
		// IMPLs in the same tier before launching any scouts or waves.
		tierSlugs := getTierSlugs(manifest, currentTier)
		conflictReport, conflictErr := protocol.CheckIMPLConflicts(tierSlugs, opts.RepoPath)
		if conflictErr != nil {
			loopResult.Errors = append(loopResult.Errors, fmt.Sprintf("P1+ tier conflict check failed: %s", conflictErr))
			loopResult.FinalState = "conflict_check_error"
			return result.NewFailure[TierLoopResult]([]result.SAWError{
				result.NewFatal(result.CodeP1Violation,
					fmt.Sprintf("P1+ tier conflict check failed: %s", conflictErr)).
					WithContext("tier", fmt.Sprintf("%d", currentTier)),
			})
		}
		if len(conflictReport.Conflicts) > 0 {
			for _, c := range conflictReport.Conflicts {
				loopResult.Errors = append(loopResult.Errors,
					fmt.Sprintf("file ownership conflict: %s claimed by %s", c.File, strings.Join(c.Impls, ", ")))
			}
			loopResult.FinalState = "conflict_detected"
			return result.NewFailure[TierLoopResult]([]result.SAWError{
				result.NewFatal(result.CodeP1Violation,
					fmt.Sprintf("RunTierLoop: %d file ownership conflict(s) detected in tier %d", len(conflictReport.Conflicts), currentTier)).
					WithContext("tier", fmt.Sprintf("%d", currentTier)),
			})
		}

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
				loopResult.Errors = append(loopResult.Errors, fmt.Sprintf("scout launch failed: %s", scoutErr))
				loopResult.FinalState = "scout_failed"
				return result.NewFailure[TierLoopResult]([]result.SAWError{
					result.NewFatal(result.CodeScoutFailed,
						fmt.Sprintf("RunTierLoop: launch scouts: %s", scoutErr)).
						WithContext("tier", fmt.Sprintf("%d", currentTier)),
				})
			}
			if len(scoutResult.Failed) > 0 {
				for slug, errMsg := range scoutResult.Errors {
					loopResult.Errors = append(loopResult.Errors, fmt.Sprintf("scout %s failed: %s", slug, errMsg))
				}
				loopResult.FinalState = "scout_partial_failure"
				return result.NewFailure[TierLoopResult]([]result.SAWError{
					result.NewFatal(result.CodeScoutFailed,
						fmt.Sprintf("RunTierLoop: %d scouts failed", len(scoutResult.Failed))).
						WithContext("tier", fmt.Sprintf("%d", currentTier)),
				})
			}
		}

		// Step 5: Validate pre-existing IMPLs
		if len(preExisting) > 0 {
			validationErrs := protocol.ValidateProgramImportMode(manifest, opts.RepoPath)
			if len(validationErrs) > 0 {
				for _, ve := range validationErrs {
					loopResult.Errors = append(loopResult.Errors, fmt.Sprintf("[%s] %s", ve.Code, ve.Message))
				}
			}
		}

		// Step 7: If NOT autoMode, return with RequiresReview
		if !opts.AutoMode {
			loopResult.RequiresReview = true
			loopResult.TiersExecuted = countCompleteTiers(manifest)
			loopResult.TiersRemaining = len(manifest.Tiers) - loopResult.TiersExecuted
			loopResult.FinalState = "awaiting_review"
			return result.NewSuccess(loopResult)
		}

		// Step 8: Execute waves for each IMPL in the tier, respecting serial_waves
		// tierSlugs was already computed in Step 3a above.
		waveProgress := make(map[string]int)

		// Find max wave count in this tier
		maxWaveCount := 0
		for _, slug := range tierSlugs {
			wc := getIMPLWaveCount(manifest, slug)
			if wc > maxWaveCount {
				maxWaveCount = wc
			}
		}

		for waveNum := 1; waveNum <= maxWaveCount; waveNum++ {
			for _, slug := range tierSlugs {
				implWaveCount := getIMPLWaveCount(manifest, slug)
				if waveNum > implWaveCount {
					continue // this IMPL doesn't have wave waveNum
				}

				// Wait for serial wave constraint to clear
				// (in this sequential implementation we simply check and skip if blocked,
				//  relying on the outer wave loop to retry — but for correctness in the
				//  sequential model, we run serial waves in slug order which is sufficient)
				if isCrossImplSerialWaveBlocked(manifest, currentTier, slug, waveNum, waveProgress) {
					// In fully sequential execution, this should not happen because we
					// complete one serial wave at a time. Log and continue.
					loopResult.Errors = append(loopResult.Errors,
						fmt.Sprintf("unexpected serial wave block: impl=%s wave=%d", slug, waveNum))
					continue
				}

				implPath := findIMPLDocPath(opts.RepoPath, slug)
				if implPath == "" {
					loopResult.Errors = append(loopResult.Errors,
						fmt.Sprintf("IMPL doc not found for %s", slug))
					loopResult.FinalState = "impl_not_found"
					return result.NewFailure[TierLoopResult]([]result.SAWError{
						result.NewFatal(result.CodeIMPLNotFound,
							fmt.Sprintf("RunTierLoop: IMPL doc not found for slug %q", slug)).
							WithContext("slug", slug),
					})
				}

				implBranch := protocol.ProgramBranchName(manifest.ProgramSlug, currentTier, slug)
				if !git.BranchExists(opts.RepoPath, implBranch) {
					if _, err := git.Run(opts.RepoPath, "checkout", "-b", implBranch); err != nil {
						loopResult.Errors = append(loopResult.Errors,
							fmt.Sprintf("failed to create IMPL branch %s: %s", implBranch, err))
						continue
					}
				}

				_, waveErr := RunWaveFull(ctx, RunWaveFullOpts{
					ManifestPath: implPath,
					RepoPath:     opts.RepoPath,
					WaveNum:      waveNum,
					MergeTarget:  implBranch,
				})
				if waveErr != nil {
					loopResult.Errors = append(loopResult.Errors,
						fmt.Sprintf("wave %d for %s failed: %s", waveNum, slug, waveErr))
				} else {
					waveProgress[slug] = waveNum

					// Persist wave completion status to disk after each successful wave.
					// Re-read the manifest to ensure we merge with any on-disk changes,
					// then update the in-memory state and save.
					if freshManifest, parseErr := protocol.ParseProgramManifest(opts.ManifestPath); parseErr == nil {
						// Update IMPL status to reflect progress (wave completed for this slug)
						for i := range freshManifest.Impls {
							if freshManifest.Impls[i].Slug == slug {
								// Mark as in-progress if not already complete
								if freshManifest.Impls[i].Status == "pending" || freshManifest.Impls[i].Status == "scouting" || freshManifest.Impls[i].Status == "reviewed" {
									freshManifest.Impls[i].Status = "reviewed"
								}
								break
							}
						}
						// Non-fatal: save error is logged but does not abort execution
						if saveErr := protocol.SaveYAML(opts.ManifestPath, freshManifest); saveErr == nil {
							// Reload in-memory manifest so subsequent iterations see updated state
							if reloaded, reloadErr := protocol.ParseProgramManifest(opts.ManifestPath); reloadErr == nil {
								manifest = reloaded
							}
						}
					}
				}
			}

			emitEvent(opts.OnEvent, TierLoopEvent{
				Type:   "wave_complete",
				Tier:   currentTier,
				Detail: fmt.Sprintf("Wave %d complete for tier %d", waveNum, currentTier),
			})
		}

		// Emit impl_complete events after all waves done
		for _, slug := range tierSlugs {
			emitEvent(opts.OnEvent, TierLoopEvent{
				Type:   "impl_complete",
				Tier:   currentTier,
				Detail: fmt.Sprintf("IMPL %s wave execution finished", slug),
			})
		}

		// Steps 9-12: AdvanceTierAutomatically runs the tier gate (E29), freezes
		// contracts (E30), and advances the tier (E33). Do NOT call RunTierGate
		// separately — AdvanceTierAutomatically owns the canonical gate invocation.
		advRes := AdvanceTierAutomatically(manifest, currentTier, opts.RepoPath, opts.AutoMode)
		if advRes.IsFatal() {
			loopResult.Errors = append(loopResult.Errors, fmt.Sprintf("advance tier error: %s", advRes.Errors[0].Message))
			loopResult.FinalState = "advance_error"
			return result.NewFailure[TierLoopResult]([]result.SAWError{
				result.NewFatal(advRes.Errors[0].Code,
					fmt.Sprintf("RunTierLoop: advance tier: %s", advRes.Errors[0].Message)).
					WithContext("tier", fmt.Sprintf("%d", currentTier)),
			})
		}
		advResult := advRes.GetData()

		// Surface the gate result from AdvanceTierAutomatically.
		gateResult := advResult.GateResult
		emitEvent(opts.OnEvent, TierLoopEvent{
			Type:   "tier_gate",
			Tier:   currentTier,
			Detail: fmt.Sprintf("Tier gate passed=%v", gateResult.Passed),
		})

		if !gateResult.Passed {
			// E40: Emit tier_gate_failed.
			nilSafeEmit(ctx, opts.ObsEmitter, observability.NewTierGateFailedEvent(manifest.ProgramSlug, currentTier, fmt.Sprintf("tier %d gate failed: %d gate(s) did not pass", currentTier, len(gateResult.GateResults))))

			// Step 11: Auto trigger replan (E34)
			if opts.AutoMode {
				emitEvent(opts.OnEvent, TierLoopEvent{
					Type:   "replan_triggered",
					Tier:   currentTier,
					Detail: "Tier gate failed, triggering replan",
				})

				// Write REPLANNING state to manifest before the replan call
				if freshManifest, parseErr := protocol.ParseProgramManifest(opts.ManifestPath); parseErr == nil {
					freshManifest.State = protocol.ProgramState("REPLANNING")
					// Non-fatal: state write error is ignored; replan proceeds regardless
					_ = protocol.SaveYAML(opts.ManifestPath, freshManifest)
				}

				replanRes := AutoTriggerReplan(opts.ManifestPath, currentTier, gateResult, opts.Model)
				if replanRes.IsFatal() {
					loopResult.Errors = append(loopResult.Errors, fmt.Sprintf("replan failed: %s", replanRes.Errors[0].Message))
					loopResult.FinalState = "replan_failed"
					return result.NewFailure[TierLoopResult]([]result.SAWError{
						result.NewFatal(replanRes.Errors[0].Code,
							fmt.Sprintf("RunTierLoop: replan: %s", replanRes.Errors[0].Message)).
							WithContext("tier", fmt.Sprintf("%d", currentTier)),
					})
				}

				// Re-parse manifest after replan
				manifest, err = protocol.ParseProgramManifest(opts.ManifestPath)
				if err != nil {
					return result.NewFailure[TierLoopResult]([]result.SAWError{
						result.NewFatal(result.CodeIMPLParseFailed,
							fmt.Sprintf("RunTierLoop: re-parse after replan: %s", err)).
							WithContext("manifest_path", opts.ManifestPath),
					})
				}
				// Continue loop to retry the tier
				continue
			}

			loopResult.FinalState = "gate_failed"
			loopResult.Errors = append(loopResult.Errors, "tier gate failed")
			loopResult.TiersExecuted = countCompleteTiers(manifest)
			loopResult.TiersRemaining = len(manifest.Tiers) - loopResult.TiersExecuted
			return result.NewSuccess(loopResult)
		}

		// E40: Emit tier_gate_passed.
		nilSafeEmit(ctx, opts.ObsEmitter, observability.NewTierGatePassedEvent(manifest.ProgramSlug, currentTier))

		// Emit contracts_frozen if AdvanceTierAutomatically froze contracts.
		if advResult.FreezeResult != nil && advResult.FreezeResult.Success {
			emitEvent(opts.OnEvent, TierLoopEvent{
				Type:   "contracts_frozen",
				Tier:   currentTier,
				Detail: fmt.Sprintf("Froze %d contracts", len(advResult.FreezeResult.ContractsFrozen)),
			})
		}

		if advResult.ProgramComplete {
			loopResult.ProgramComplete = true
			loopResult.FinalState = "complete"
			loopResult.TiersExecuted = len(manifest.Tiers)
			loopResult.TiersRemaining = 0
			return result.NewSuccess(loopResult)
		}

		if advResult.AdvancedToNext {
			emitEvent(opts.OnEvent, TierLoopEvent{
				Type:   "tier_advanced",
				Tier:   currentTier,
				Detail: fmt.Sprintf("Advancing from tier %d to %d", currentTier, advResult.NextTier),
			})
			nilSafeEmit(ctx, opts.ObsEmitter, observability.NewTierAdvancedEvent(manifest.ProgramSlug, currentTier))
			loopResult.TiersExecuted++
		} else if !opts.AutoMode {
			// Non-auto mode: check if this is the final tier.
			if isFinalTier(manifest, currentTier) {
				loopResult.ProgramComplete = true
				loopResult.FinalState = "complete"
				loopResult.TiersExecuted = len(manifest.Tiers)
				loopResult.TiersRemaining = 0
				return result.NewSuccess(loopResult)
			}
			emitEvent(opts.OnEvent, TierLoopEvent{
				Type:   "tier_advanced",
				Tier:   currentTier,
				Detail: fmt.Sprintf("Advancing from tier %d to %d", currentTier, currentTier+1),
			})
			nilSafeEmit(ctx, opts.ObsEmitter, observability.NewTierAdvancedEvent(manifest.ProgramSlug, currentTier))
			loopResult.TiersExecuted++
		}

		// Re-parse manifest to pick up any status changes
		manifest, err = protocol.ParseProgramManifest(opts.ManifestPath)
		if err != nil {
			return result.NewFailure[TierLoopResult]([]result.SAWError{
				result.NewFatal(result.CodeIMPLParseFailed,
					fmt.Sprintf("RunTierLoop: re-parse manifest: %s", err)).
					WithContext("manifest_path", opts.ManifestPath),
			})
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
func AutoTriggerReplan(manifestPath string, tierNumber int, gateResult *protocol.TierGateData, model string) result.Result[ReplanResult] {
	reason := buildReplanReason(tierNumber, gateResult)

	return replanProgramFunc(ReplanProgramOpts{
		ProgramManifestPath: manifestPath,
		Reason:              reason,
		FailedTier:          tierNumber,
		PlannerModel:        model,
	})
}

// buildReplanReason constructs a human-readable reason string from gate failure details.
func buildReplanReason(tierNumber int, gateResult *protocol.TierGateData) string {
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
	locations := []string{
		filepath.Join(repoPath, "docs/IMPL", fmt.Sprintf("IMPL-%s.yaml", slug)),
		filepath.Join(repoPath, "docs/IMPL/complete", fmt.Sprintf("IMPL-%s.yaml", slug)),
		filepath.Join(repoPath, "docs/IMPL", fmt.Sprintf("IMPL-%s.yml", slug)),
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	return ""
}

// isCrossImplSerialWaveBlocked returns true if starting wave waveNum for impl slug
// would violate cross-IMPL serialization constraints within tierNumber.
// A wave is blocked if: (a) it is listed in slug's SerialWaves AND
// (b) any other IMPL in the same tier has that same wave number still in-progress
// (i.e., its waveProgress value is < waveNum or it hasn't reached waveNum yet
// from the perspective of the serial ordering).
// waveProgress maps slug -> highest wave number that has COMPLETED for that IMPL.
func isCrossImplSerialWaveBlocked(
	manifest *protocol.PROGRAMManifest,
	tierNumber int,
	slug string,
	waveNum int,
	waveProgress map[string]int,
) bool {
	// Find this IMPL's SerialWaves
	var serialWaves []int
	for _, impl := range manifest.Impls {
		if impl.Slug == slug {
			serialWaves = impl.SerialWaves
			break
		}
	}
	if len(serialWaves) == 0 {
		return false // no serial constraint
	}
	isSerial := false
	for _, sw := range serialWaves {
		if sw == waveNum {
			isSerial = true
			break
		}
	}
	if !isSerial {
		return false // this wave number is not constrained
	}

	// Check all other IMPLs in the same tier
	tierSlugs := getTierSlugs(manifest, tierNumber)
	for _, other := range tierSlugs {
		if other == slug {
			continue
		}
		// Check if other IMPL also has waveNum in its SerialWaves
		var otherSerial []int
		for _, impl := range manifest.Impls {
			if impl.Slug == other {
				otherSerial = impl.SerialWaves
				break
			}
		}
		otherIsSerial := false
		for _, sw := range otherSerial {
			if sw == waveNum {
				otherIsSerial = true
				break
			}
		}
		if !otherIsSerial {
			continue // other IMPL doesn't have this wave as serial
		}

		// If other IMPL hasn't completed waveNum yet, this IMPL is blocked
		// (waveProgress[other] < waveNum means other hasn't finished waveNum)
		if waveProgress[other] < waveNum {
			return true
		}
	}
	return false
}

// emitEvent safely calls the OnEvent callback if it is non-nil.
func emitEvent(fn func(TierLoopEvent), event TierLoopEvent) {
	if fn != nil {
		fn(event)
	}
}

// nilSafeEmit calls emitter.Emit only when emitter is non-nil.
// Mirrors the loggerFrom nil-safe pattern in runner.go.
func nilSafeEmit(ctx context.Context, emitter ObsEmitter, event observability.Event) {
	if emitter != nil {
		emitter.Emit(ctx, event)
	}
}
