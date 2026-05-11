package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newUpdateStatusCmd() *cobra.Command {
	var waveNum int
	var agentID string
	var status string

	cmd := &cobra.Command{
		Use:   "update-status <manifest-path>",
		Short: "Update agent status in manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			manifestPath := args[0]
			res := protocol.UpdateStatus(ctx, manifestPath, waveNum, agentID, protocol.CompletionStatus(status), protocol.UpdateStatusOpts{}) // commit not accepted by this command
			if res.IsFatal() {
				return fmt.Errorf("update-status: %s", res.Errors[0].Message)
			}
			data := res.GetData()

			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID (required)")
	cmd.Flags().StringVar(&status, "status", "", "Status: complete|partial|blocked (required)")

	_ = cmd.MarkFlagRequired("wave")
	_ = cmd.MarkFlagRequired("agent")
	_ = cmd.MarkFlagRequired("status")

	return cmd
}
