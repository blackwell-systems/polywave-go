package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
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
  polywave-tools program-replan docs/PROGRAM/PROGRAM.yaml --reason "Tier 2 gate failed: integration tests failing"
  polywave-tools program-replan docs/PROGRAM/PROGRAM.yaml --reason "User-initiated replan" --tier 0
  polywave-tools program-replan docs/PROGRAM/PROGRAM.yaml --reason "Blocked IMPL" --tier 3 --model claude-opus-4-6

Exit codes:
  0 - Re-planning succeeded, revised manifest validated
  1 - Re-planning failed or validation failed
  2 - Parse error`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Parse the PROGRAM manifest (return error on parse error)
			_, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("program-replan: parse error: %w", err)
			}

			// Construct opts from flags
			opts := engine.ReplanProgramOpts{
				ProgramManifestPath: manifestPath,
				Reason:              reason,
				FailedTier:          failedTier,
				PlannerModel:        model,
			}

			// Call engine function
			res := engine.ReplanProgram(opts)
			if res.IsFatal() {
				return fmt.Errorf("program-replan: %s", res.Errors[0].Message)
			}
			result := res.GetData()

			// Output JSON result
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			// Return error if validation failed
			if !result.ValidationPassed {
				return fmt.Errorf("program-replan: validation failed")
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
