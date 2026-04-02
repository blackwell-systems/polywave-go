package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/retry"
	"github.com/spf13/cobra"
)

func newBuildRetryContextCmd() *cobra.Command {
	var agentID string
	var attemptNum int

	cmd := &cobra.Command{
		Use:   "build-retry-context <manifest-path>",
		Short: "Build structured retry context for a failed agent attempt",
		Long: `Reads an agent's completion report from the manifest, classifies the error
type, and outputs a structured JSON retry context to stdout.

The retry context includes:
  - attempt_number: the retry attempt number
  - agent_id: the target agent
  - error_class: classified error type (import_error, type_error, etc.)
  - error_excerpt: first 2000 chars of the error output
  - gate_results: gate type/pass-fail summary from the manifest
  - suggested_fixes: actionable suggestions for the error class
  - prior_notes: Notes field from the prior completion report
  - prompt_text: formatted markdown retry prompt

Exit codes:
  0 – success
  1 – agent has no completion report or manifest cannot be loaded`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			rcResult := retry.BuildRetryAttempt(cmd.Context(), manifestPath, agentID, attemptNum)
			if rcResult.IsFatal() {
				if len(rcResult.Errors) > 0 {
					return fmt.Errorf("build-retry-context: %s", rcResult.Errors[0].Message)
				}
				return fmt.Errorf("build-retry-context: unknown error")
			}

			rc := rcResult.GetData()
			out, err := json.MarshalIndent(rc, "", "  ")
			if err != nil {
				return fmt.Errorf("build-retry-context: failed to marshal output: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID to build retry context for (required)")
	cmd.Flags().IntVar(&attemptNum, "attempt", 1, "Retry attempt number")

	_ = cmd.MarkFlagRequired("agent")

	return cmd
}
