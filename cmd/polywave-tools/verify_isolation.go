package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newVerifyIsolationCmd() *cobra.Command {
	var expectedBranch string
	var cwd string

	cmd := &cobra.Command{
		Use:   "verify-isolation",
		Short: "Verify agent is running in the correct isolated worktree (Field 0 / E12)",
		Long: `Verify agent is running in the correct isolated worktree.

The --cwd flag specifies the working directory to check. If omitted, uses the
global --repo-dir flag (defaults to ".").

Agents should explicitly pass their working directory:
  polywave-tools verify-isolation --cwd "$(pwd)" --branch polywave/slug/wave1-agent-A

This prevents false negatives when the agent's cwd differs from the orchestrator's cwd.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use explicit --cwd if provided, otherwise fall back to global repoDir
			workDir := cwd
			if workDir == "" {
				workDir = repoDir
			}

			res := protocol.VerifyIsolation(workDir, expectedBranch)
			if res.IsFatal() {
				return fmt.Errorf("verify-isolation: %s", res.Errors[0].Message)
			}
			data := res.GetData()

			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(out))

			if !data.OK {
				return fmt.Errorf("verify-isolation: isolation check failed")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&expectedBranch, "branch", "", "Expected branch name, e.g. polywave/slug/wave1-agent-A (required)")
	cmd.Flags().StringVar(&cwd, "cwd", "", "Working directory to verify (defaults to --repo-dir if omitted)")
	_ = cmd.MarkFlagRequired("branch")

	return cmd
}
