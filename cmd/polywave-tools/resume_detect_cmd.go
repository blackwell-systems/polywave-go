package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/polywave-go/pkg/resume"
	"github.com/spf13/cobra"
)

// newResumeDetectCmd returns the cobra.Command for "polywave-tools resume-detect".
// It scans the repository for interrupted Polywave sessions and outputs a JSON array
// of SessionState objects to stdout. Exit code is always 0.
func newResumeDetectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume-detect",
		Short: "Detect interrupted Polywave sessions in the repository",
		Long: `Scans docs/IMPL/ for IMPL manifests that are not complete or unsuitable,
inspects completion reports and git worktrees, and reports the state of any
in-progress Polywave sessions. Output is a JSON array written to stdout.

An empty array is written when no interrupted sessions are found.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			res := resume.Detect(cmd.Context(), repoDir)
			if !res.IsSuccess() {
				if len(res.Errors) > 0 {
					return fmt.Errorf("resume-detect: %s", res.Errors[0].Message)
				}
				return fmt.Errorf("resume-detect: detection failed")
			}
			sessions := res.GetData()

			out, err := json.MarshalIndent(sessions, "", "  ")
			if err != nil {
				return fmt.Errorf("resume-detect: marshal: %w", err)
			}

			fmt.Fprintln(os.Stdout, string(out))
			return nil
		},
	}

	return cmd
}
