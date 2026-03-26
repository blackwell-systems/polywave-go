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
		Long: `Finalizes a program tier by merging all IMPL branches into main in sequence,
then running the tier-level quality gates.

Analogous to finalize-wave but operates at the program tier level.
Stops on the first merge failure and does not run the tier gate if any merge fails.

When --auto is set, automatically advances to the next tier after the gate passes.

Examples:
  sawtools finalize-tier docs/PROGRAM/PROGRAM.yaml --tier 1
  sawtools finalize-tier program.yaml --tier 2 --repo-dir /path/to/repo
  sawtools finalize-tier program.yaml --tier 1 --auto

Exit codes:
  0 - All merges succeeded and tier gate passed
  1 - One or more merges failed or tier gate failed
  2 - Parse error or tier not found`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			result, err := protocol.FinalizeTier(manifestPath, tierNum, repoDir)
			if err != nil {
				return err
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			if !result.IsSuccess() {
				return fmt.Errorf("finalize-tier: one or more merges failed or tier gate failed")
			}

			// If --auto is set and tier gate passed, advance to next tier
			if autoAdvance {
				manifest, err := protocol.ParseProgramManifest(manifestPath)
				if err != nil {
					return fmt.Errorf("finalize-tier: failed to parse manifest for auto-advance: %w", err)
				}

				advanceResult, err := engine.AdvanceTierAutomatically(manifest, tierNum, repoDir, true)
				if err != nil {
					return fmt.Errorf("finalize-tier: auto-advance failed: %w", err)
				}

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
