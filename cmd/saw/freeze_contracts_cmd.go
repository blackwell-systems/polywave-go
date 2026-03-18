package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
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
  sawtools freeze-contracts docs/PROGRAM.yaml --tier 1
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
					fmt.Fprintf(os.Stderr, "freeze-contracts: failed to get current directory: %v\n", err)
					os.Exit(2)
				}
				repoDir = cwd
			}

			// Parse the PROGRAM manifest
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "freeze-contracts: parse error: %v\n", err)
				os.Exit(2)
			}

			// Freeze contracts
			result, err := protocol.FreezeContracts(manifest, tier, repoDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "freeze-contracts: %v\n", err)
				os.Exit(2)
			}

			// Output JSON result
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			// Exit code based on success/errors
			if !result.Success {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&tier, "tier", 0, "Tier number (required)")
	cmd.MarkFlagRequired("tier")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "Repository directory (default: current directory)")

	return cmd
}
