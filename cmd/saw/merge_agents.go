package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newMergeAgentsCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "merge-agents <manifest-path>",
		Short: "Merge all agent branches for a wave",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			result, err := protocol.MergeAgents(manifestPath, waveNum, repoDir, "")
			if err != nil {
				return fmt.Errorf("merge-agents: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !result.Success {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	cmd.MarkFlagRequired("wave")

	return cmd
}
