package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
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
  sawtools tier-gate docs/PROGRAM.yaml --tier 1
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
					fmt.Fprintf(os.Stderr, "tier-gate: failed to get current directory: %v\n", err)
					os.Exit(2)
				}
				repoDir = cwd
			}

			// Parse the PROGRAM manifest
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "tier-gate: parse error: %v\n", err)
				os.Exit(2)
			}

			// Run tier gate verification
			result, err := protocol.RunTierGate(manifest, tier, repoDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "tier-gate: %v\n", err)
				os.Exit(2)
			}

			// Output JSON result
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			// Exit code based on pass/fail
			if !result.Passed {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&tier, "tier", 0, "Tier number to verify (required)")
	cmd.MarkFlagRequired("tier")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "Repository directory (default: current directory)")

	return cmd
}
