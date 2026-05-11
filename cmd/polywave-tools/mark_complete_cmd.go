package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newMarkCompleteCmd() *cobra.Command {
	var date string

	cmd := &cobra.Command{
		Use:   "mark-complete <manifest-path>",
		Short: "Write completion marker to IMPL manifest and archive to complete/ subdirectory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Default date to today if not provided
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			// Load manifest to get FeatureSlug before archiving
			manifest, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("mark-complete: failed to load manifest: %w", err)
			}

			if err := protocol.WriteCompletionMarker(manifestPath, date); err != nil {
				return fmt.Errorf("mark-complete: %w", err)
			}

			// Always archive to docs/IMPL/complete/
			archRes := protocol.ArchiveIMPL(cmd.Context(), manifestPath)
			if archRes.IsFatal() {
				return fmt.Errorf("mark-complete: archive failed: %s", archRes.Errors[0].Message)
			}
			archivedPath := archRes.GetData().NewPath

			// Auto-clean worktrees for the completed IMPL
			projectRoot := repoDir
			if projectRoot == "" || projectRoot == "." {
				projectRoot = filepath.Dir(filepath.Dir(filepath.Dir(manifestPath)))
			}
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
					cleanRes := protocol.CleanStaleWorktrees(matching, true) // force=true, we just archived
					if !cleanRes.IsFatal() {
						cleanedCount = len(cleanRes.GetData().Cleaned)
					}
					if cleanedCount > 0 {
						fmt.Fprintf(os.Stderr, "mark-complete: cleaned %d stale worktree(s) for %s\n", cleanedCount, manifest.FeatureSlug)
					}
				}
			}

			out, _ := json.Marshal(map[string]interface{}{
				"marked":           true,
				"date":             date,
				"path":             archivedPath,
				"archived":         true,
				"worktrees_cleaned": cleanedCount,
			})
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Completion date (YYYY-MM-DD, defaults to today)")

	return cmd
}
