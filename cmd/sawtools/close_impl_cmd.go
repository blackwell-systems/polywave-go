package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
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
			ctx := cmd.Context()
			manifestPath := args[0]
			logger := newSawLogger()

			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			// Load manifest before archiving to get slug
			manifest, err := protocol.Load(ctx, manifestPath)
			if err != nil {
				return fmt.Errorf("close-impl: failed to load manifest: %w", err)
			}

			// Step 1a: Transition state to COMPLETE (best-effort; failure does not abort)
			stateRes := protocol.SetImplState(ctx, manifestPath, protocol.StateComplete, protocol.SetImplStateOpts{})
			if !stateRes.IsSuccess() {
				logger.Warn("close-impl: state transition to COMPLETE failed", "errs", stateRes.Errors)
			}

			// Step 1b: Write completion marker
			if err := protocol.WriteCompletionMarker(manifestPath, date); err != nil {
				return fmt.Errorf("close-impl: mark-complete failed: %w", err)
			}
			fmt.Fprintf(os.Stderr, "close-impl: marked complete (date=%s)\n", date)

			// Step 2: Archive to complete/
			archRes := protocol.ArchiveIMPL(cmd.Context(), manifestPath)
			if archRes.IsFatal() {
				return fmt.Errorf("close-impl: archive failed: %s", archRes.Errors[0].Message)
			}
			archivedPath := archRes.GetData().NewPath
			fmt.Fprintf(os.Stderr, "close-impl: archived to %s\n", archivedPath)

			// Step 3: Update CONTEXT.md
			projectRoot := repoDir
			if projectRoot == "" || projectRoot == "." {
				projectRoot = filepath.Dir(filepath.Dir(filepath.Dir(manifestPath)))
			}
			contextRes := protocol.UpdateContext(cmd.Context(), archivedPath, projectRoot)
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

			// Step 6: Restore original branch if currently on a SAW-managed branch.
			// Priority: (1) manifest.OriginalBranch, (2) .saw-state prepare-result.json, (3) "main".
			branchRestored := false
			branchOut, branchErr := git.Run(projectRoot, "branch", "--show-current")
			if branchErr == nil {
				currentBranch := strings.TrimSpace(branchOut)
				isSAWBranch := strings.HasPrefix(currentBranch, "saw/") ||
					(strings.HasPrefix(currentBranch, "wave") && strings.Contains(currentBranch, "-agent-"))
				if isSAWBranch {
					restoreBranch := "main"
					// 1. Check manifest field (most reliable — survives .saw-state cleanup)
					if manifest.OriginalBranch != "" {
						restoreBranch = manifest.OriginalBranch
					} else {
						// 2. Fall back to .saw-state prepare-result.json
						if entries, err := os.ReadDir(sawStatePath); err == nil {
							for _, e := range entries {
								if !e.IsDir() || len(e.Name()) < 4 || e.Name()[:4] != "wave" {
									continue
								}
								data, err := os.ReadFile(filepath.Join(sawStatePath, e.Name(), "prepare-result.json"))
								if err != nil {
									continue
								}
								var result struct {
									OriginalBranch string `json:"original_branch"`
								}
								if err := json.Unmarshal(data, &result); err == nil && result.OriginalBranch != "" {
									restoreBranch = result.OriginalBranch
									break
								}
							}
						}
					}
					if _, checkoutErr := git.Run(projectRoot, "checkout", restoreBranch); checkoutErr != nil {
						fmt.Fprintf(os.Stderr, "close-impl: warning: could not restore branch %q: %v\n", restoreBranch, checkoutErr)
					} else {
						fmt.Fprintf(os.Stderr, "close-impl: restored branch %q (was on %s)\n", restoreBranch, currentBranch)
						branchRestored = true
					}
				}
			}

			// Step 7: Stage deletion of original path, add archived file + CONTEXT.md, commit.
			commitSHA := ""
			// Stage deletion of original IMPL path (it was moved by os.Rename; git sees it deleted).
			if rmErr := git.Rm(projectRoot, manifestPath); rmErr != nil {
				fmt.Fprintf(os.Stderr, "close-impl: warning: git rm %s failed: %v\n", manifestPath, rmErr)
			} else {
				// Stage the new archived file.
				if addErr := git.Add(projectRoot, archivedPath); addErr != nil {
					fmt.Fprintf(os.Stderr, "close-impl: warning: git add %s failed: %v\n", archivedPath, addErr)
				}
				// Stage CONTEXT.md if it was updated.
				if !contextFailed && contextData != nil && contextData.ContextPath != "" {
					if addErr := git.Add(projectRoot, contextData.ContextPath); addErr != nil {
						fmt.Fprintf(os.Stderr, "close-impl: warning: git add %s failed: %v\n", contextData.ContextPath, addErr)
					}
				}
				// Commit.
				commitMsg := fmt.Sprintf("chore: close-impl %s [SAW:complete]", manifest.FeatureSlug)
				var commitErr error
				commitSHA, commitErr = git.Commit(projectRoot, commitMsg)
				if commitErr != nil {
					fmt.Fprintf(os.Stderr, "close-impl: warning: commit failed: %v\n", commitErr)
				} else {
					fmt.Fprintf(os.Stderr, "close-impl: committed %s\n", commitSHA)
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
				"branch_restored":   branchRestored,
				"committed":         commitSHA,
			})
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Completion date (YYYY-MM-DD, defaults to today)")

	return cmd
}
