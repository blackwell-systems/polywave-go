package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// splitCSV splits a comma-separated string into trimmed, non-empty parts.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func newSetCompletionCmd() *cobra.Command {
	var agentID string
	var status string
	var commit string
	var worktree string
	var branch string
	var filesChanged string
	var filesCreated string
	var testsAdded string
	var verification string

	cmd := &cobra.Command{
		Use:   "set-completion <manifest-path>",
		Short: "Set completion report for an agent in manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Validate status
			switch status {
			case "complete", "partial", "blocked":
				// valid
			default:
				return fmt.Errorf("--status must be one of: complete, partial, blocked (got %q)", status)
			}

			// Load manifest
			m, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("set-completion: %w", err)
			}

			// Build completion report
			report := protocol.CompletionReport{
				Status:       status,
				Commit:       commit,
				Worktree:     worktree,
				Branch:       branch,
				FilesChanged: splitCSV(filesChanged),
				FilesCreated: splitCSV(filesCreated),
				TestsAdded:   splitCSV(testsAdded),
				Verification: verification,
			}

			// Set completion report
			if err := protocol.SetCompletionReport(m, agentID, report); err != nil {
				return fmt.Errorf("set-completion: %w", err)
			}

			// Save manifest
			if err := protocol.Save(m, manifestPath); err != nil {
				return fmt.Errorf("set-completion: %w", err)
			}

			// Print result
			out, _ := json.Marshal(map[string]interface{}{
				"agent":  agentID,
				"status": status,
				"saved":  true,
			})
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID (required)")
	cmd.Flags().StringVar(&status, "status", "", "Status: complete|partial|blocked (required)")
	cmd.Flags().StringVar(&commit, "commit", "", "Commit SHA (required)")
	cmd.Flags().StringVar(&worktree, "worktree", "", "Worktree path")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name")
	cmd.Flags().StringVar(&filesChanged, "files-changed", "", "Comma-separated list of changed files")
	cmd.Flags().StringVar(&filesCreated, "files-created", "", "Comma-separated list of created files")
	cmd.Flags().StringVar(&testsAdded, "tests-added", "", "Comma-separated list of tests added")
	cmd.Flags().StringVar(&verification, "verification", "", "Verification result text")

	_ = cmd.MarkFlagRequired("agent")
	_ = cmd.MarkFlagRequired("status")
	_ = cmd.MarkFlagRequired("commit")

	return cmd
}
