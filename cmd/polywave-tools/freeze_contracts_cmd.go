package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newFreezeContractsCmd() *cobra.Command {
	var (
		tier    int
		repoDir string
	)

	cmd := &cobra.Command{
		Use:   "freeze-contracts <program-manifest>",
		Short: "Freeze program contracts at a tier boundary",
		Long: `Freeze program contracts at a tier boundary.

Verifies that contract source files exist and are committed to HEAD.
Updates the manifest's contract state.

Examples:
  sawtools freeze-contracts docs/PROGRAM/PROGRAM.yaml --tier 1
  sawtools freeze-contracts program.yaml --tier 2 --repo-dir /path/to/repo

Exit codes:
  0 - Success (all matching contracts frozen)
  1 - Freeze errors (contracts missing or uncommitted)
  2 - Parse error or tier not found`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Resolve repo directory (default: current working directory)
			if repoDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("freeze-contracts: failed to get current directory: %w", err)
				}
				repoDir = cwd
			}

			// Parse the PROGRAM manifest
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("freeze-contracts: parse error: %w", err)
			}

			// Freeze contracts
			res := protocol.FreezeContracts(manifest, tier, repoDir)
			if res.IsFatal() {
				return fmt.Errorf("freeze-contracts: %s", res.Errors[0].Message)
			}
			data := res.GetData()

			// Output JSON result
			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			// Return error if not successful
			if !data.Success {
				return fmt.Errorf("freeze-contracts: freeze errors detected")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&tier, "tier", 0, "Tier number (required)")
	cmd.MarkFlagRequired("tier")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "Repository directory (default: current directory)")

	return cmd
}
