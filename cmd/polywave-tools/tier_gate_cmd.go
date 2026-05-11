package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newTierGateCmd() *cobra.Command {
	var (
		tier    int
		repoDir string
	)

	cmd := &cobra.Command{
		Use:   "tier-gate <program-manifest>",
		Short: "Verify tier gate: check all IMPLs in tier are complete and run quality gates",
		Long: `Tier gate verification for PROGRAM manifests.

Checks that:
1. All IMPLs in the specified tier are complete
2. All required quality gates pass

Examples:
  sawtools tier-gate docs/PROGRAM/PROGRAM.yaml --tier 1
  sawtools tier-gate program.yaml --tier 2 --repo-dir /path/to/repo

Exit codes:
  0 - Tier gate passed
  1 - Tier gate failed (incomplete IMPLs or gates failed)
  2 - Parse error or tier not found`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Resolve repo directory (default: current working directory)
			if repoDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("tier-gate: failed to get current directory: %w", err)
				}
				repoDir = cwd
			}

			// Parse the PROGRAM manifest
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("tier-gate: parse error: %w", err)
			}

			// Run tier gate verification
			res := protocol.RunTierGate(cmd.Context(), manifest, tier, repoDir)
			if res.IsFatal() {
				return fmt.Errorf("tier-gate: %s", res.Errors[0].Message)
			}
			data := res.GetData()

			// Output JSON result
			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			// Return error if gate failed
			if !data.Passed {
				return fmt.Errorf("tier gate failed")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&tier, "tier", 0, "Tier number to verify (required)")
	cmd.MarkFlagRequired("tier")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "Repository directory (default: current directory)")

	return cmd
}
