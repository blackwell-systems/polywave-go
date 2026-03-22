// Registration: add newFinalizeTierCmd() to main.go AddCommand list.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newFinalizeTierCmd() *cobra.Command {
	var tierNum int

	cmd := &cobra.Command{
		Use:   "finalize-tier <program-manifest>",
		Short: "Finalize a program tier: merge all IMPL branches and run tier gate",
		Long: `Finalizes a program tier by merging all IMPL branches into main in sequence,
then running the tier-level quality gates.

Analogous to finalize-wave but operates at the program tier level.
Stops on the first merge failure and does not run the tier gate if any merge fails.

Examples:
  sawtools finalize-tier docs/PROGRAM.yaml --tier 1
  sawtools finalize-tier program.yaml --tier 2 --repo-dir /path/to/repo

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

			if !result.Success {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&tierNum, "tier", 0, "Tier number to finalize (required)")
	_ = cmd.MarkFlagRequired("tier")

	return cmd
}
