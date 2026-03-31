package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"github.com/spf13/cobra"
)

// cleanupStaleOutput is the JSON output shape for the cleanup-stale command.
type cleanupStaleOutput struct {
	Detected int                      `json:"detected"`
	Cleaned  []protocol.StaleWorktree `json:"cleaned"`
	Skipped  []protocol.StaleWorktree `json:"skipped"`
	Errors   []cleanupStaleError      `json:"errors"`
}

type cleanupStaleError struct {
	Worktree protocol.StaleWorktree `json:"worktree"`
	Error    string                 `json:"error"`
}

func newCleanupStaleCmd() *cobra.Command {
	var dryRun bool
	var force bool
	var slug string
	var all bool

	cmd := &cobra.Command{
		Use:   "cleanup-stale",
		Short: "Detect and remove stale SAW worktrees",
		Long: `Scans for stale SAW worktrees (completed IMPLs, orphaned branches,
merged-but-not-cleaned) and optionally removes them.

Use --slug <slug> to target a specific IMPL slug.
Use --all to clean stale worktrees across all slugs.
Use --dry-run to preview what would be cleaned without acting.
Use --force to skip safety checks for uncommitted changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if slug != "" && all {
				return fmt.Errorf("--slug and --all are mutually exclusive")
			}
			if slug == "" && !all {
				return fmt.Errorf("specify --slug <slug> or --all to target cleanup")
			}

			var stale []protocol.StaleWorktree
			var err error
			if slug != "" {
				stale, err = protocol.DetectStaleWorktreesForSlug(repoDir, slug)
			} else {
				stale, err = protocol.DetectStaleWorktrees(repoDir)
			}
			if err != nil {
				return fmt.Errorf("detect stale worktrees: %w", err)
			}

			if dryRun {
				skipped := stale
				if skipped == nil {
					skipped = []protocol.StaleWorktree{}
				}
				out := cleanupStaleOutput{
					Detected: len(stale),
					Cleaned:  []protocol.StaleWorktree{},
					Skipped:  skipped,
					Errors:   []cleanupStaleError{},
				}
				return printJSON(out)
			}

			cleanRes := protocol.CleanStaleWorktrees(stale, force)
			if cleanRes.IsFatal() {
				return fmt.Errorf("clean stale worktrees: %w", errors.Join(result.ToErrors(cleanRes.Errors)...))
			}
			cleanData := cleanRes.GetData()

			out := cleanupStaleOutput{
				Detected: len(stale),
				Cleaned:  cleanData.Cleaned,
				Skipped:  cleanData.Skipped,
				Errors:   convertStaleErrors(cleanData.Errors),
			}
			return printJSON(out)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report what would be cleaned without acting")
	cmd.Flags().BoolVar(&force, "force", false, "Skip safety checks for uncommitted changes")
	cmd.Flags().StringVar(&slug, "slug", "", "Only clean stale worktrees matching this IMPL slug")
	cmd.Flags().BoolVar(&all, "all", false, "Clean stale worktrees across all slugs")

	return cmd
}

func convertStaleErrors(errs []protocol.StaleCleanupFailure) []cleanupStaleError {
	out := make([]cleanupStaleError, len(errs))
	for i, e := range errs {
		out[i] = cleanupStaleError{
			Worktree: e.Worktree,
			Error:    e.Error,
		}
	}
	return out
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
