// Registration: add newFinalizeTierCmd() to main.go AddCommand list.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newFinalizeTierCmd() *cobra.Command {
	var tierNum int
	var autoAdvance bool

	cmd := &cobra.Command{
		Use:   "finalize-tier <program-manifest>",
		Short: "Finalize a program tier: merge all IMPL branches and run tier gate",
		Long: `Finalizes a program tier using the full thick-orchestrator sequence:
  1. Close all complete IMPLs (mark-complete + archive + update context)
  2. Merge IMPL branches to main, handling worktree-checked-out branches
  3. Run tier-level quality gates
  4. Update IMPL statuses in the PROGRAM manifest and commit

Analogous to finalize-wave and prepare-wave: no manual preconditions required.
Idempotent: already-closed IMPLs and already-merged branches are skipped.

When --auto is set, automatically advances to the next tier after the gate passes.

Examples:
  sawtools finalize-tier docs/PROGRAM/PROGRAM.yaml --tier 1
  sawtools finalize-tier program.yaml --tier 2 --repo-dir /path/to/repo
  sawtools finalize-tier program.yaml --tier 1 --auto

Exit codes:
  0 - All steps succeeded
  1 - One or more steps failed
  2 - Parse error or tier not found`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			engineRes := engine.FinalizeTierEngine(cmd.Context(), engine.FinalizeTierOpts{
				ManifestPath: manifestPath,
				TierNumber:   tierNum,
				RepoDir:      repoDir,
				Logger:       newSawLogger(),
			})
			engineResult := engineRes.GetData()

			out, _ := json.MarshalIndent(engineRes, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			if engineRes.IsFatal() {
				if len(engineRes.Errors) > 0 {
					return fmt.Errorf("finalize-tier: %s", engineRes.Errors[0].Message)
				}
				return fmt.Errorf("finalize-tier: failed")
			}
			if len(engineResult.Errors) > 0 {
				return fmt.Errorf("finalize-tier: %s", engineResult.Errors[0].Message)
			}

			// If --auto is set and tier gate passed, advance to next tier
			if autoAdvance {
				manifest, err := protocol.ParseProgramManifest(manifestPath)
				if err != nil {
					return fmt.Errorf("finalize-tier: failed to parse manifest for auto-advance: %w", err)
				}

				advRes := engine.AdvanceTierAutomatically(manifest, tierNum, repoDir, true)
				if advRes.IsFatal() {
					return fmt.Errorf("finalize-tier: auto-advance failed: %s", advRes.Errors[0].Message)
				}
				advanceResult := advRes.GetData()

				advOut, _ := json.MarshalIndent(advanceResult, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(advOut))

				if advanceResult.AdvancedToNext {
					fmt.Fprintf(cmd.ErrOrStderr(), "Advanced to tier %d\n", advanceResult.NextTier)
				}
				if advanceResult.ProgramComplete {
					fmt.Fprintln(cmd.ErrOrStderr(), "Program complete!")
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&tierNum, "tier", 0, "Tier number to finalize (required)")
	_ = cmd.MarkFlagRequired("tier")
	cmd.Flags().BoolVar(&autoAdvance, "auto", false, "Automatically advance to next tier after gate passes")

	return cmd
}
