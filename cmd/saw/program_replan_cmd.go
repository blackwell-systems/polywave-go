package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newProgramReplanCmd() *cobra.Command {
	var (
		reason     string
		failedTier int
		model      string
	)

	cmd := &cobra.Command{
		Use:   "program-replan <program-manifest>",
		Short: "Re-engage the Planner agent to revise a PROGRAM manifest",
		Long: `Re-engage the Planner agent to revise a PROGRAM manifest.

Used when a tier gate fails or a user explicitly requests re-planning.
Reads the current manifest, constructs a revision prompt with failure
context, launches the Planner agent, and returns the updated manifest path.

Examples:
  sawtools program-replan docs/PROGRAM/PROGRAM.yaml --reason "Tier 2 gate failed: integration tests failing"
  sawtools program-replan docs/PROGRAM/PROGRAM.yaml --reason "User-initiated replan" --tier 0
  sawtools program-replan docs/PROGRAM/PROGRAM.yaml --reason "Blocked IMPL" --tier 3 --model claude-opus-4-6

Exit codes:
  0 - Re-planning succeeded, revised manifest validated
  1 - Re-planning failed or validation failed
  2 - Parse error`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Parse the PROGRAM manifest (exit 2 on parse error)
			_, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "program-replan: parse error: %v\n", err)
				os.Exit(2)
			}

			// Construct opts from flags
			opts := engine.ReplanProgramOpts{
				ProgramManifestPath: manifestPath,
				Reason:              reason,
				FailedTier:          failedTier,
				PlannerModel:        model,
			}

			// Call engine function
			result, err := engine.ReplanProgram(opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "program-replan: %v\n", err)
				os.Exit(1)
			}

			// Output JSON result
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			// Exit 1 if validation failed
			if !result.ValidationPassed {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Why re-planning was triggered (required)")
	cmd.MarkFlagRequired("reason")
	cmd.Flags().IntVar(&failedTier, "tier", 0, "Tier number that failed (0 if user-initiated)")
	cmd.Flags().StringVar(&model, "model", "", "Model override for the Planner agent")

	return cmd
}
