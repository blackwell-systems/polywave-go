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
			result, err := protocol.VerifyCommits(manifestPath, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("verify-commits: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !result.AllValid {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
