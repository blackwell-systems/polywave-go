// Registration: add newPrepareTierCmd() to main.go AddCommand list.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newPrepareTierCmd() *cobra.Command {
	var tierNum int
	var skipCritic bool
	var runPrepareWave bool
	var waveNumFlag int
	var mergeTargetFlag string

	cmd := &cobra.Command{
		Use:   "prepare-tier <program-manifest>",
		Short: "Prepare a program tier: check conflicts, validate IMPLs, create branches",
		Long: `Prepares a program tier for execution by:
1. Checking for file ownership conflicts across IMPLs in the tier
2. Validating each IMPL doc (with --fix auto-corrections)
3. Creating IMPL branches for all IMPLs in the tier

This is the counterpart to finalize-tier. Together they bookend tier execution:
  prepare-tier -> orchestrator launches scouts/agents -> finalize-tier

When --run-prepare-wave is set, prepare-tier also runs prepare-wave for
each IMPL in the tier after worktree creation, using --commit-state
internally to auto-commit SAW state between IMPL preparations. The JSON
output gains a prepare_wave_results array with per-IMPL worktrees and
success status.

Examples:
  sawtools prepare-tier docs/PROGRAM/PROGRAM.yaml --tier 1
  sawtools prepare-tier program.yaml --tier 2 --repo-dir /path/to/repo
  sawtools prepare-tier program.yaml --tier 1 --run-prepare-wave --wave 1

Exit codes:
  0 - All steps succeeded
  1 - Step failure (conflicts found, validation failed, or worktree creation failed)
  2 - Fatal error (manifest not found, tier not found)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			prepRes := protocol.PrepareTier(protocol.PrepareTierOpts{
				Ctx:                 cmd.Context(),
				ProgramManifestPath: manifestPath,
				TierNumber:          tierNum,
				RepoDir:             repoDir,
				SkipCritic:          skipCritic,
				RunPrepareWave:      runPrepareWave,
				WaveNum:             waveNumFlag,
				MergeTarget:         mergeTargetFlag,
			})
			if prepRes.IsFatal() {
				return fmt.Errorf("prepare-tier: %s", prepRes.Errors[0].Message)
			}
			result := prepRes.GetData()

			// Run prepare-wave for each IMPL if requested and prior steps succeeded.
			if runPrepareWave && result.Success {
				projectRoot := repoDir
				if projectRoot == "" {
					projectRoot = "."
				}

				waveNum := waveNumFlag
				if waveNum == 0 {
					waveNum = 1
				}

				// Parse the manifest to get the tier's IMPL slug list.
				var tier *protocol.ProgramTier
				manifest, parseErr := protocol.ParseProgramManifest(manifestPath)
				if parseErr == nil {
					for i := range manifest.Tiers {
						if manifest.Tiers[i].Number == tierNum {
							tier = &manifest.Tiers[i]
							break
						}
					}
				}

				if tier != nil {
					for _, slug := range tier.Impls {
						implPath, resolveErr := protocol.ResolveIMPLPath(repoDir, slug)
						if resolveErr != nil {
							result.PrepareWaveResults = append(result.PrepareWaveResults,
								protocol.IMPLPrepareWaveResult{
									ImplSlug: slug,
									Success:  false,
									Error:    resolveErr.Error(),
								})
							continue
						}

						pwResult, pwErr := engine.PrepareWave(context.Background(), engine.PrepareWaveOpts{
							IMPLPath:    implPath,
							RepoPath:    projectRoot,
							WaveNum:     waveNum,
							MergeTarget: mergeTargetFlag,
							CommitState: true,
							Logger:      newSawLogger(),
							OnEvent: func(step, status, detail string) {
								fmt.Fprintf(os.Stderr, "prepare-tier[%s]: [%s] %s — %s\n", slug, step, status, detail)
							},
						})

						entry := protocol.IMPLPrepareWaveResult{
							ImplSlug: slug,
						}
						if pwErr != nil {
							entry.Success = false
							entry.Error = pwErr.Error()
						} else {
							entry.Success = pwResult.Success
							entry.Worktrees = pwResult.Worktrees
						}
						result.PrepareWaveResults = append(result.PrepareWaveResults, entry)
					}

					// Mark overall success false if any prepare-wave failed.
					for _, pw := range result.PrepareWaveResults {
						if !pw.Success {
							result.Success = false
							break
						}
					}
				}
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			if !result.Success {
				return fmt.Errorf("prepare-tier: step failure")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&tierNum, "tier", 0, "Tier number to prepare (required)")
	_ = cmd.MarkFlagRequired("tier")
	cmd.Flags().BoolVar(&skipCritic, "skip-critic", false, "Auto-skip E37 critic gate for IMPLs missing critic reports")
	cmd.Flags().BoolVar(&runPrepareWave, "run-prepare-wave", false,
		"Also run prepare-wave for each IMPL in the tier. Auto-commits SAW state between IMPLs.")
	cmd.Flags().IntVar(&waveNumFlag, "wave", 1,
		"Wave number to pass to prepare-wave (default 1). Only used with --run-prepare-wave.")
	cmd.Flags().StringVar(&mergeTargetFlag, "merge-target", "",
		"Baseline branch for prepare-wave calls (default: current HEAD). Only used with --run-prepare-wave.")

	return cmd
}
