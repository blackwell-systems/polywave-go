package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/spf13/cobra"
)

func newMarkProgramCompleteCmd() *cobra.Command {
	var (
		date    string
		repoDir string
	)

	cmd := &cobra.Command{
		Use:   "mark-program-complete <program-manifest>",
		Short: "Mark a PROGRAM manifest as complete and update CONTEXT.md",
		Long: `Mark a PROGRAM manifest as complete.

Verifies all tiers are complete, updates the manifest state to PROGRAM_COMPLETE,
sets the completion_date, writes the SAW:PROGRAM:COMPLETE marker, updates CONTEXT.md,
and commits both files.

Exit codes:
  0 - Success
  1 - Not all tiers complete
  2 - Parse error or other failure`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Default repoDir to current working directory
			if repoDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("mark-program-complete: failed to get current directory: %w", err)
				}
				repoDir = cwd
			}

			res := engine.MarkProgramComplete(cmd.Context(), engine.MarkProgramCompleteOpts{
				ManifestPath: manifestPath,
				RepoDir:      repoDir,
				Date:         date,
			})
			if res.IsFatal() {
				return fmt.Errorf("mark-program-complete: %s", res.Errors[0].Message)
			}
			result := res.GetData()

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Completion date (YYYY-MM-DD, defaults to today)")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "Repository directory (default: current directory)")

	return cmd
}
