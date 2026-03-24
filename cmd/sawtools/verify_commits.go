package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
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
			res := protocol.VerifyCommits(manifestPath, waveNum, repoDir)
			if !res.IsSuccess() {
				return fmt.Errorf("verify-commits: %v", res.Errors)
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
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
