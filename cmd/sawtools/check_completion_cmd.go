package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// checkCompletionResult is the JSON output schema for check-completion.
type checkCompletionResult struct {
	Found        bool     `json:"found"`
	AgentID      string   `json:"agent_id"`
	Status       string   `json:"status"`
	HasCommit    bool     `json:"has_commit"`
	FilesChanged []string `json:"files_changed"`
}

func newCheckCompletionCmd() *cobra.Command {
	var agentID string

	cmd := &cobra.Command{
		Use:   "check-completion <manifest-path>",
		Short: "Check if an agent has a completion report in the manifest",
		Long:  "Verifies a specific agent's completion report exists in the IMPL doc and has valid structure. Used by the SubagentStop hook for wave agent validation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentID == "" {
				return fmt.Errorf("--agent flag is required")
			}

			manifestPath := args[0]

			// Load manifest
			m, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("check-completion: %w", err)
			}

			// Look up agent in completion_reports
			result := checkCompletionResult{
				AgentID: agentID,
			}

			if m.CompletionReports != nil {
				if report, ok := m.CompletionReports[agentID]; ok && report.Status != "" {
					result.Found = true
					result.Status = report.Status
					result.HasCommit = report.Commit != ""
					result.FilesChanged = report.FilesChanged
					if result.FilesChanged == nil {
						result.FilesChanged = []string{}
					}
				}
			}

			// Output JSON to stdout
			out, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("check-completion: failed to marshal result: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			// Return error if not found or status is empty
			if !result.Found {
				return fmt.Errorf("agent completion report not found or status is empty")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID to check (required)")
	_ = cmd.MarkFlagRequired("agent")

	return cmd
}
