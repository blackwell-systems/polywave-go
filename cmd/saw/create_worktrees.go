package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCreateWorktreesCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "create-worktrees <manifest-path>",
		Short: "Create git worktrees for all agents in a wave",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			result, err := protocol.CreateWorktrees(manifestPath, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("create-worktrees: %w", err)
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	cmd.MarkFlagRequired("wave")

	return cmd
}
