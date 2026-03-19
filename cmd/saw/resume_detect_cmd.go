package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/resume"
	"github.com/spf13/cobra"
)

// newResumeDetectCmd returns the cobra.Command for "sawtools resume-detect".
// It scans the repository for interrupted SAW sessions and outputs a JSON array
// of SessionState objects to stdout. Exit code is always 0.
func newResumeDetectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume-detect",
		Short: "Detect interrupted SAW sessions in the repository",
		Long: `Scans docs/IMPL/ for IMPL manifests that are not complete or unsuitable,
inspects completion reports and git worktrees, and reports the state of any
in-progress SAW sessions. Output is a JSON array written to stdout.

An empty array is written when no interrupted sessions are found.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := resume.Detect(repoDir)
			if err != nil {
				return fmt.Errorf("resume-detect: %w", err)
			}

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
