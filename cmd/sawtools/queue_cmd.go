package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/queue"
	"github.com/spf13/cobra"
)

// newQueueCmd returns the parent "queue" subcommand with its children.
func newQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage IMPL queue",
	}
	cmd.AddCommand(newQueueAddCmd(), newQueueListCmd(), newQueueNextCmd())
	return cmd
}

// newQueueAddCmd creates a queue item and prints JSON confirming the add.
func newQueueAddCmd() *cobra.Command {
	var (
		title       string
		priority    int
		description string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an item to the IMPL queue",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := queue.NewManager(repoDir)

			item := queue.Item{
				Title:              title,
				Priority:           priority,
				FeatureDescription: description,
			}

			addRes := mgr.Add(item)
			if addRes.IsFatal() {
				return fmt.Errorf("queue add: %s", addRes.Errors[0].Message)
			}

			slug := addRes.GetData().Slug
			path := addRes.GetData().FilePath

			result := map[string]interface{}{
				"added": true,
				"slug":  slug,
				"path":  path,
			}
			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("queue add: marshal output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Item title (required)")
	cmd.Flags().IntVar(&priority, "priority", 50, "Item priority (lower = higher priority)")
	cmd.Flags().StringVar(&description, "description", "", "Feature description")
	_ = cmd.MarkFlagRequired("title")

	return cmd
}

// newQueueListCmd lists all queue items sorted by priority.
func newQueueListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all IMPL queue items",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := queue.NewManager(repoDir)

			listRes := mgr.List()
			if listRes.IsFatal() {
				return fmt.Errorf("queue list: %s", listRes.Errors[0].Message)
			}
			items := listRes.GetData().Items

			// Build a simplified output slice.
			type listItem struct {
				Slug     string `json:"slug"`
				Title    string `json:"title"`
				Priority int    `json:"priority"`
				Status   string `json:"status"`
			}

			out := make([]listItem, 0, len(items))
			for _, it := range items {
				out = append(out, listItem{
					Slug:     it.Slug,
					Title:    it.Title,
					Priority: it.Priority,
					Status:   it.Status,
				})
			}

			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return fmt.Errorf("queue list: marshal output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	return cmd
}

// newQueueNextCmd returns the next eligible queue item.
func newQueueNextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Return the next eligible IMPL queue item",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := queue.NewManager(repoDir)

			nextRes := mgr.Next()

			var output interface{}
			if nextRes.IsFatal() {
				// QUEUE_EMPTY is not an error to the CLI caller — report next: nil.
				if len(nextRes.Errors) > 0 && nextRes.Errors[0].Code == "QUEUE_EMPTY" {
					output = map[string]interface{}{"next": nil}
				} else {
					return fmt.Errorf("queue next: %s", nextRes.Errors[0].Message)
				}
			} else {
				item := nextRes.GetData()
				output = map[string]interface{}{
					"slug":     item.Slug,
					"title":    item.Title,
					"priority": item.Priority,
				}
			}

			out, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("queue next: marshal output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	return cmd
}
