package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCleanupCmd() *cobra.Command {
	var waveNum int
	var slugFlag string
	var allStale bool
	var force bool

	cmd := &cobra.Command{
		Use:   "cleanup [manifest-path]",
		Short: "Remove worktrees and branches after merge",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if slugFlag != "" && allStale {
				return fmt.Errorf("--slug and --all-stale are mutually exclusive")
			}

			if slugFlag != "" {
				res := protocol.CleanupBySlug(repoDir, slugFlag, force)
				if res.IsFatal() {
					return fmt.Errorf("cleanup-by-slug: %v", res.Errors)
				}
				out, _ := json.MarshalIndent(res.GetData(), "", "  ")
				fmt.Println(string(out))
				return nil
			}

			if allStale {
				result, err := protocol.CleanupAllStale(repoDir, force)
				if err != nil {
					return fmt.Errorf("cleanup-all-stale: %w", err)
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(out))
				return nil
			}

			// Default path: manifest-based cleanup
			if len(args) == 0 {
				return fmt.Errorf("manifest-path is required when --slug and --all-stale are not set")
			}
			manifestPath := args[0]

			result, err := protocol.Cleanup(manifestPath, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("cleanup: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required for manifest-based cleanup)")
	cmd.Flags().StringVar(&slugFlag, "slug", "", "Clean up worktrees from a specific IMPL slug (no manifest required)")
	cmd.Flags().BoolVar(&allStale, "all-stale", false, "Clean all stale worktrees across all slugs (no manifest required)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip safety checks for uncommitted changes")

	return cmd
}
