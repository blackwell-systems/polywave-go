// Registration: add newCreateProgramWorktreesCmd() to main.go AddCommand list.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCreateProgramWorktreesCmd() *cobra.Command {
	var tierNum int

	cmd := &cobra.Command{
		Use:   "create-program-worktrees <program-manifest>",
		Short: "Create IMPL branches and worktrees for all IMPLs in a program tier",
		Long: `Creates long-lived IMPL branches for all IMPLs in a program tier.
Branch naming: saw/program/{program-slug}/tier{N}-impl-{impl-slug}
These branches serve as merge targets for all wave executions within each IMPL.
Waves merge to the IMPL branch, not to main.

Examples:
  sawtools create-program-worktrees docs/PROGRAM.yaml --tier 1
  sawtools create-program-worktrees program.yaml --tier 2 --repo-dir /path/to/repo

Exit codes:
  0 - All worktrees created successfully
  1 - One or more worktrees failed
  2 - Parse error or tier not found`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			res := protocol.CreateProgramWorktrees(manifestPath, tierNum, repoDir)

			if res.IsFatal() {
				// Print structured error and exit 2 for parse/tier-not-found errors
				for _, e := range res.Errors {
					fmt.Fprintf(os.Stderr, "create-program-worktrees: %s: %s\n", e.Code, e.Message)
				}
				os.Exit(2)
			}

			data := res.GetData()
			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			if !res.IsSuccess() {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&tierNum, "tier", 0, "Tier number (required)")
	_ = cmd.MarkFlagRequired("tier")

	return cmd
}
