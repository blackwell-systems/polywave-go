package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newUpdateAgentPromptCmd() *cobra.Command {
	var agentID string
	var newPrompt string

	cmd := &cobra.Command{
		Use:   "update-agent-prompt <manifest-path>",
		Short: "Update an agent's prompt/task in the manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			m, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("update-agent-prompt: %w", err)
			}

			if promptRes := protocol.UpdateAgentPrompt(m, agentID, newPrompt); promptRes.IsFatal() {
				msg := ""
				if len(promptRes.Errors) > 0 {
					msg = promptRes.Errors[0].Message
				}
				return fmt.Errorf("update-agent-prompt: %s", msg)
			}

			if saveRes := protocol.Save(context.TODO(), m, manifestPath); saveRes.IsFatal() {
				saveErrMsg := "save failed"
				if len(saveRes.Errors) > 0 {
					saveErrMsg = saveRes.Errors[0].Message
				}
				return fmt.Errorf("update-agent-prompt: %s", saveErrMsg)
			}

			result := struct {
				Agent   string `json:"agent"`
				Updated bool   `json:"updated"`
			}{
				Agent:   agentID,
				Updated: true,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID (required)")
	cmd.Flags().StringVar(&newPrompt, "prompt", "", "New prompt/task text (required)")

	_ = cmd.MarkFlagRequired("agent")
	_ = cmd.MarkFlagRequired("prompt")

	return cmd
}
