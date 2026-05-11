package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
	"github.com/spf13/cobra"
)

func newVerifyCommitsCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "verify-commits <manifest-path>",
		Short: "Verify each agent branch has commits (I5 trip wire)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			res := protocol.VerifyCommits(context.Background(), manifestPath, waveNum, repoDir)
			if !res.IsSuccess() {
				return fmt.Errorf("verify-commits: %w", errors.Join(result.ToErrors(res.Errors)...))
			}
			result := res.GetData()

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			// Check if all agents have commits
			allValid := true
			for _, agent := range result.Agents {
				if !agent.HasCommits {
					allValid = false
					break
				}
			}
			if !allValid {
				return fmt.Errorf("verify-commits: not all agents have commits")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
