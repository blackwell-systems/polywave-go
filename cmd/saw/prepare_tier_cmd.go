// Registration: add newPrepareTierCmd() to main.go AddCommand list.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newPrepareTierCmd() *cobra.Command {
	var tierNum int

	cmd := &cobra.Command{
		Use:   "prepare-tier <program-manifest>",
		Short: "Prepare a program tier: check conflicts, validate IMPLs, create branches",
		Long: `Prepares a program tier for execution by:
1. Checking for file ownership conflicts across IMPLs in the tier
2. Validating each IMPL doc (with --fix auto-corrections)
3. Creating IMPL branches for all IMPLs in the tier

This is the counterpart to finalize-tier. Together they bookend tier execution:
  prepare-tier -> orchestrator launches scouts/agents -> finalize-tier

Examples:
  sawtools prepare-tier docs/PROGRAM/PROGRAM.yaml --tier 1
  sawtools prepare-tier program.yaml --tier 2 --repo-dir /path/to/repo

Exit codes:
  0 - All steps succeeded
  1 - Step failure (conflicts found, validation failed, or worktree creation failed)
  2 - Fatal error (manifest not found, tier not found)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			result, err := protocol.PrepareTier(manifestPath, tierNum, repoDir)
			if err != nil {
				if result == nil {
					fmt.Fprintln(cmd.ErrOrStderr(), err)
					os.Exit(2)
				}
				return err
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			if !result.Success {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&tierNum, "tier", 0, "Tier number to prepare (required)")
	_ = cmd.MarkFlagRequired("tier")

	return cmd
}
