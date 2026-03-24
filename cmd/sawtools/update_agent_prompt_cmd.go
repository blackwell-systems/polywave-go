package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
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

			m, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("update-agent-prompt: %w", err)
			}

			if err := protocol.UpdateAgentPrompt(m, agentID, newPrompt); err != nil {
				return fmt.Errorf("update-agent-prompt: %w", err)
			}

			if err := protocol.Save(m, manifestPath); err != nil {
				return fmt.Errorf("update-agent-prompt: %w", err)
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
