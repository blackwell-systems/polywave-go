package main

import (
	"context"
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
	var notes string
	var failureType string

	cmd := &cobra.Command{
		Use:   "set-completion <manifest-path>",
		Short: "Set completion report for an agent in manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Build completion report using the canonical builder.
			builder := protocol.NewCompletionReport(agentID).
				WithStatus(protocol.CompletionStatus(status)).
				WithCommit(commit).
				WithWorktree(worktree).
				WithBranch(branch).
				WithFiles(splitCSV(filesChanged), splitCSV(filesCreated)).
				WithTestsAdded(splitCSV(testsAdded)).
				WithVerification(verification).
				WithNotes(notes)

			if failureType != "" {
				builder = builder.WithFailureType(failureType)
			}

			// Validate before touching disk.
			if err := builder.Validate(); err != nil {
				return fmt.Errorf("set-completion: %w", err)
			}

			// Persist with consolidated lock.
			if err := protocol.WithCompletionReportLock(context.TODO(), func(ctx context.Context) error {
				m, loadErr := protocol.Load(ctx, manifestPath)
				if loadErr != nil {
					return fmt.Errorf("set-completion: %w", loadErr)
				}
				if appendErr := builder.AppendToManifest(m); appendErr != nil {
					return fmt.Errorf("set-completion: %w", appendErr)
				}
				if saveRes := protocol.Save(ctx, m, manifestPath); saveRes.IsFatal() {
					if len(saveRes.Errors) > 0 {
						return fmt.Errorf("set-completion: %s", saveRes.Errors[0].Message)
					}
					return fmt.Errorf("set-completion: failed to save manifest")
				}
				return nil
			}); err != nil {
				return err
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
	cmd.Flags().StringVar(&commit, "commit", "", "Commit SHA")
	cmd.Flags().StringVar(&worktree, "worktree", "", "Worktree path")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name")
	cmd.Flags().StringVar(&filesChanged, "files-changed", "", "Comma-separated list of changed files")
	cmd.Flags().StringVar(&filesCreated, "files-created", "", "Comma-separated list of created files")
	cmd.Flags().StringVar(&testsAdded, "tests-added", "", "Comma-separated list of tests added")
	cmd.Flags().StringVar(&verification, "verification", "", "Verification result text")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-text notes")
	cmd.Flags().StringVar(&failureType, "failure-type", "", "Failure type: transient|fixable|needs_replan|escalate|timeout")

	_ = cmd.MarkFlagRequired("agent")
	_ = cmd.MarkFlagRequired("status")

	return cmd
}
