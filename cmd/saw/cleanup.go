package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCleanupCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "cleanup <manifest-path>",
		Short: "Remove worktrees and branches after merge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			result, err := protocol.Cleanup(manifestPath, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("cleanup: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
