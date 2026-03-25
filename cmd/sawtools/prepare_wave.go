package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/spf13/cobra"
)

func newPrepareWaveCmd() *cobra.Command {
	var waveNum int
	var noCache bool
	var mergeTarget string

	cmd := &cobra.Command{
		Use:   "prepare-wave <manifest-path>",
		Short: "Prepare all agents in a wave (check deps + create worktrees + extract briefs + init journals)",
		Long: `Prepares all agents in a wave for parallel execution by:
0. Checking for dependency conflicts (missing packages, version conflicts)
1. Creating git worktrees for all agents in the wave
2. Extracting each agent's brief to their worktree root (.saw-agent-brief.md)
3. Initializing journal observers for all agents

This combines check-deps + create-worktrees + prepare-agent into a single atomic operation,
reducing wave setup from 6+ commands to 1 command.

If dependency conflicts are detected (missing packages or version conflicts), the command
exits with code 1 and prints a JSON report. Resolve conflicts in main branch and re-run.

NOTE: For solo agents (1 agent in wave), use prepare-agent --no-worktree instead.
prepare-wave always creates worktrees, which is unnecessary overhead for single-agent
waves that execute on the main branch.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if waveNum == 0 {
				return fmt.Errorf("--wave is required")
			}

			manifestPath := args[0]

			// Determine project root from manifest path or --repo-dir flag
			projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(manifestPath)))
			if repoDir != "" {
				projectRoot = repoDir
			}

			// Build engine options
			opts := engine.PrepareWaveOpts{
				IMPLPath:    manifestPath,
				RepoPath:    projectRoot,
				WaveNum:     waveNum,
				MergeTarget: mergeTarget,
				NoCache:     noCache,
				Logger:      newSawLogger(),
				OnEvent: func(step string, status string, detail string) {
					fmt.Fprintf(os.Stderr, "prepare-wave: [%s] %s — %s\n", step, status, detail)
				},
			}

			// Auto-install M4 pre-commit lint gate hook if missing
			if err := runInstallHooks(projectRoot); err != nil {
				fmt.Fprintf(os.Stderr, "prepare-wave: could not auto-install M4 hook: %v\n", err)
			}

			// Call engine
			result, err := engine.PrepareWave(context.Background(), opts)
			if err != nil {
				// On failure, output partial result if available (for diagnostics)
				if result != nil {
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(out))
				}
				return err
			}

			// Output result as JSON
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable baseline gate result caching")
	cmd.Flags().StringVar(&mergeTarget, "merge-target", "", "Baseline branch for verification (default: current HEAD)")

	return cmd
}
