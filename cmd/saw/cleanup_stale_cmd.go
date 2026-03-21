package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
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

	cmd := &cobra.Command{
		Use:   "cleanup-stale",
		Short: "Detect and remove stale SAW worktrees",
		Long: `Scans for stale SAW worktrees (completed IMPLs, orphaned branches,
merged-but-not-cleaned) and optionally removes them.

Use --dry-run to preview what would be cleaned without acting.
Use --force to skip safety checks for uncommitted changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			stale, err := protocol.DetectStaleWorktrees(repoDir)
			if err != nil {
				return fmt.Errorf("detect stale worktrees: %w", err)
			}

			if dryRun {
				out := cleanupStaleOutput{
					Detected: len(stale),
					Cleaned:  []protocol.StaleWorktree{},
					Skipped:  stale,
					Errors:   []cleanupStaleError{},
				}
				return printJSON(out)
			}

			result, err := protocol.CleanStaleWorktrees(stale, force)
			if err != nil {
				return fmt.Errorf("clean stale worktrees: %w", err)
			}

			out := cleanupStaleOutput{
				Detected: len(stale),
				Cleaned:  result.Cleaned,
				Skipped:  result.Skipped,
				Errors:   convertErrors(result.Errors),
			}
			return printJSON(out)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report what would be cleaned without acting")
	cmd.Flags().BoolVar(&force, "force", false, "Skip safety checks for uncommitted changes")

	return cmd
}

func convertErrors(errs []struct {
	Worktree protocol.StaleWorktree `json:"worktree"`
	Error    string                 `json:"error"`
}) []cleanupStaleError {
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
