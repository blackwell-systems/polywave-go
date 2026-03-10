package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newVerifyIsolationCmd() *cobra.Command {
	var expectedBranch string

	cmd := &cobra.Command{
		Use:   "verify-isolation",
		Short: "Verify agent is running in the correct isolated worktree (Field 0 / E12)",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := protocol.VerifyIsolation(repoDir, expectedBranch)
			if err != nil {
				return fmt.Errorf("verify-isolation: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !result.OK {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&expectedBranch, "branch", "", "Expected branch name, e.g. wave1-agent-A (required)")
	_ = cmd.MarkFlagRequired("branch")

	return cmd
}
