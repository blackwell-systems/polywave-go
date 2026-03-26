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
			logger := newSawLogger()

			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			// Load manifest before archiving to get slug
			manifest, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("close-impl: failed to load manifest: %w", err)
			}

			// Step 1a: Transition state to COMPLETE (best-effort; failure does not abort)
			stateRes := protocol.SetImplState(manifestPath, protocol.StateComplete, protocol.SetImplStateOpts{})
			if !stateRes.IsSuccess() {
				logger.Warn("close-impl: state transition to COMPLETE failed", "errs", stateRes.Errors)
			}

			// Step 1b: Write completion marker
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
			contextRes := protocol.UpdateContext(archivedPath, projectRoot)
			contextFailed := contextRes.IsFatal()
			var contextData *protocol.UpdateContextData
			if contextFailed {
				fmt.Fprintf(os.Stderr, "close-impl: update-context warning: %s\n", contextRes.Errors[0].Message)
			} else {
				d := contextRes.GetData()
				contextData = d
				fmt.Fprintf(os.Stderr, "close-impl: updated %s\n", contextData.ContextPath)
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
					cleanRes := protocol.CleanStaleWorktrees(matching, true)
					if !cleanRes.IsFatal() {
						cleanedCount = len(cleanRes.GetData().Cleaned)
					}
					if cleanedCount > 0 {
						fmt.Fprintf(os.Stderr, "close-impl: cleaned %d stale worktree(s)\n", cleanedCount)
					}
				}
			}

			// Step 5: Clean .saw-state wave directories
			stateCleanedCount := 0
			sawStatePath := protocol.SAWStateDir(projectRoot)
			if entries, err := os.ReadDir(sawStatePath); err == nil {
				// Check if any active IMPLs exist in this repo
				activeIMPLs := 0
				implDir := protocol.IMPLDir(projectRoot)
				if implEntries, err := os.ReadDir(implDir); err == nil {
					for _, e := range implEntries {
						if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
							activeIMPLs++
						}
					}
				}
				// Only clean wave dirs if no active IMPLs remain
				if activeIMPLs == 0 {
					for _, e := range entries {
						if e.IsDir() && (len(e.Name()) >= 4 && e.Name()[:4] == "wave" || e.Name() == "archive") {
							_ = os.RemoveAll(filepath.Join(sawStatePath, e.Name()))
							stateCleanedCount++
						}
					}
					if stateCleanedCount > 0 {
						fmt.Fprintf(os.Stderr, "close-impl: cleaned %d .saw-state dir(s) (no active IMPLs)\n", stateCleanedCount)
					}
				}
			}

			out, _ := json.Marshal(map[string]interface{}{
				"marked":            true,
				"date":              date,
				"archived_path":     archivedPath,
				"context_updated":   !contextFailed,
				"context_path":      contextData,
				"worktrees_cleaned": cleanedCount,
				"state_cleaned":     stateCleanedCount,
			})
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Completion date (YYYY-MM-DD, defaults to today)")

	return cmd
}
