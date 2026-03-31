package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// checklistGroupSummary is used for structured JSON output.
type checklistGroupSummary struct {
	Title     string `json:"title"`
	ItemCount int    `json:"item_count"`
}

// populateIntegrationChecklistResult is the JSON output structure.
type populateIntegrationChecklistResult struct {
	Success    bool                    `json:"success"`
	GroupsAdded int                   `json:"groups_added"`
	ItemsAdded  int                   `json:"items_added"`
	Groups      []checklistGroupSummary `json:"groups"`
}

func newPopulateIntegrationChecklistCmd() *cobra.Command {
	var repoRoot string

	cmd := &cobra.Command{
		Use:   "populate-integration-checklist <manifest-path>",
		Short: "M5: Auto-generate post_merge_checklist from file_ownership patterns",
		Long: `Determinism tool that scans file_ownership for integration-requiring
patterns (new API handlers, React components, CLI commands, background services)
and populates post_merge_checklist groups.

Detection patterns:
- API handlers: pkg/api/*_handler.go (action:new) → register routes in server.go
- React components: web/src/components/*.tsx (action:new) → add to App.tsx navigation
- CLI commands: cmd/saw/*_cmd.go (action:new) → register in main.go
- Background services: goroutine spawn patterns → init in Server constructor

Output: Updated IMPL manifest with populated post_merge_checklist.
Idempotent: safe to run multiple times (won't duplicate items).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Load manifest
			manifest, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("populate-integration-checklist: failed to load manifest: %w", err)
			}

			// Capture pre-existing group count to compute what was added
			existingGroupCount := 0
			if manifest.PostMergeChecklist != nil {
				existingGroupCount = len(manifest.PostMergeChecklist.Groups)
			}

			// Call PopulateIntegrationChecklist
			updated, err := protocol.PopulateIntegrationChecklist(manifest)
			if err != nil {
				return fmt.Errorf("populate-integration-checklist: %w", err)
			}

			// Save updated manifest
			if saveRes := protocol.Save(context.TODO(), updated, manifestPath); saveRes.IsFatal() {
				saveErrMsg := "save failed"
				if len(saveRes.Errors) > 0 {
					saveErrMsg = saveRes.Errors[0].Message
				}
				return fmt.Errorf("populate-integration-checklist: failed to save manifest: %s", saveErrMsg)
			}

			// Build JSON output
			result := populateIntegrationChecklistResult{
				Success: true,
				Groups:  []checklistGroupSummary{},
			}

			if updated.PostMergeChecklist != nil {
				allGroups := updated.PostMergeChecklist.Groups
				// Only count groups added in this run
				result.GroupsAdded = len(allGroups) - existingGroupCount
				// Summarize all groups (not just newly added) for observability
				for _, g := range allGroups {
					result.ItemsAdded += len(g.Items)
					result.Groups = append(result.Groups, checklistGroupSummary{
						Title:     g.Title,
						ItemCount: len(g.Items),
					})
				}
			}

			// Marshal to pretty JSON
			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("populate-integration-checklist: failed to marshal result: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			return nil
		},
	}

	cmd.Flags().StringVar(&repoRoot, "repo-root", ".", "Repository root for file parsing")

	return cmd
}
