package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"github.com/spf13/cobra"
)

func newCreateWorktreesCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "create-worktrees <manifest-path>",
		Short: "Create git worktrees for all agents in a wave",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			manifestPath := args[0]
			res := protocol.CreateWorktrees(ctx, manifestPath, waveNum, repoDir, nil)
			if !res.IsSuccess() {
				return fmt.Errorf("create-worktrees: %w", errors.Join(result.ToErrors(res.Errors)...))
			}
			result := res.GetData()
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	cmd.MarkFlagRequired("wave")

	return cmd
}
