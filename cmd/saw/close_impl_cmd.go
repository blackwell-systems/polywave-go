package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCloseImplCmd() *cobra.Command {
	var date string

	cmd := &cobra.Command{
		Use:   "close-impl <manifest-path>",
		Short: "Close an IMPL: mark complete, update CONTEXT.md, archive, and clean worktrees",
		Long: `Batches the full IMPL close lifecycle into one command:

  1. Write SAW:COMPLETE marker (mark-complete)
  2. Archive to docs/IMPL/complete/
  3. Update CONTEXT.md with completion data (update-context)
  4. Clean stale worktrees for this IMPL

This replaces the manual sequence of mark-complete + update-context + git add + git commit.

Examples:
  sawtools close-impl docs/IMPL/IMPL-feature.yaml
  sawtools close-impl docs/IMPL/IMPL-feature.yaml --date 2026-03-22`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			// Load manifest before archiving to get slug
			manifest, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("close-impl: failed to load manifest: %w", err)
			}

			// Step 1: Write completion marker
			if err := protocol.WriteCompletionMarker(manifestPath, date); err != nil {
				return fmt.Errorf("close-impl: mark-complete failed: %w", err)
			}
			fmt.Fprintf(os.Stderr, "close-impl: marked complete (date=%s)\n", date)

			// Step 2: Archive to complete/
			archivedPath, err := protocol.ArchiveIMPL(manifestPath)
			if err != nil {
				return fmt.Errorf("close-impl: archive failed: %w", err)
			}
			fmt.Fprintf(os.Stderr, "close-impl: archived to %s\n", archivedPath)

			// Step 3: Update CONTEXT.md
			projectRoot := repoDir
			if projectRoot == "" || projectRoot == "." {
				projectRoot = filepath.Dir(filepath.Dir(filepath.Dir(manifestPath)))
			}
			contextPath, contextErr := protocol.UpdateContext(archivedPath, projectRoot)
			if contextErr != nil {
				fmt.Fprintf(os.Stderr, "close-impl: update-context warning: %v\n", contextErr)
			} else {
				fmt.Fprintf(os.Stderr, "close-impl: updated %s\n", contextPath)
			}

			// Step 4: Clean stale worktrees
			cleanedCount := 0
			stale, detectErr := protocol.DetectStaleWorktrees(projectRoot)
			if detectErr == nil {
				var matching []protocol.StaleWorktree
				for _, s := range stale {
					if s.Slug == manifest.FeatureSlug {
						matching = append(matching, s)
					}
				}
				if len(matching) > 0 {
					result, cleanErr := protocol.CleanStaleWorktrees(matching, true)
					if cleanErr == nil {
						cleanedCount = len(result.Cleaned)
					}
					if cleanedCount > 0 {
						fmt.Fprintf(os.Stderr, "close-impl: cleaned %d stale worktree(s)\n", cleanedCount)
					}
				}
			}

			out, _ := json.Marshal(map[string]interface{}{
				"marked":            true,
				"date":              date,
				"archived_path":     archivedPath,
				"context_updated":   contextErr == nil,
				"context_path":      contextPath,
				"worktrees_cleaned": cleanedCount,
			})
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Completion date (YYYY-MM-DD, defaults to today)")

	return cmd
}
