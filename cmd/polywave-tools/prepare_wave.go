package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newPrepareWaveCmd() *cobra.Command {
	var waveNum int
	var noCache bool
	var mergeTarget string
	var commitBaseline bool
	var commitState bool
	var jsonOnly bool
	var skipCritic bool
	var noGoWork bool
	var noWorkspaceSetup bool

	cmd := &cobra.Command{
		Use:   "prepare-wave <manifest-path>",
		Short: "Prepare all agents in a wave (check deps + create worktrees + extract briefs + init journals)",
		Long: `Prepares all agents in a wave for parallel execution by:
0. Checking for dependency conflicts (missing packages, version conflicts)
1. Creating git worktrees for all agents in the wave
2. Extracting each agent's brief to their worktree root (.polywave-agent-brief.md)
3. Initializing journal observers for all agents

This combines check-deps + create-worktrees + prepare-agent into a single atomic operation,
reducing wave setup from 6+ commands to 1 command.

If dependency conflicts are detected (missing packages or version conflicts), the command
exits with code 1 and prints a JSON report. Resolve conflicts in main branch and re-run.

NOTE: For solo agents (1 agent in wave), use prepare-agent --no-worktree instead.
prepare-wave always creates worktrees, which is unnecessary overhead for single-agent
waves that execute on the main branch.

SAW-owned state changes (IMPL yaml, gate-cache, docs/IMPL/, docs/CONTEXT.md)
are auto-committed before the working-directory check by default, so tools
that write to the IMPL doc between the critic commit and prepare-wave
(e.g., pre-wave-validate --fix, set-injection-method) do not cause false
dirty-dir failures. User code changes are NOT auto-committed; they still
cause the command to fail with a descriptive error. Use --commit-state=false
to disable auto-commit of SAW state files.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if waveNum == 0 {
				return fmt.Errorf("--wave is required")
			}

			manifestPath := args[0]

			// Load manifest for E37 pre-flight check
			manifest, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("prepare-wave: failed to load manifest: %w", err)
			}

			// E37: Critic gate — only enforce when threshold is met (3+ agents in wave 1 OR 2+ repos).
			if protocol.E37Required(manifest) && !protocol.CriticGatePasses(manifest, true) {
				if skipCritic {
					skipRes := protocol.SkipCriticForIMPL(context.TODO(), manifestPath, manifest)
						if skipRes.IsFatal() {
						return fmt.Errorf("prepare-wave: E37 critic gate: --skip-critic failed: %s", skipRes.Errors[0].Message)
					}
					if skipRes.GetData() {
						fmt.Fprintln(os.Stderr, "prepare-wave: E37 critic gate auto-skipped (--skip-critic)")
					}
				} else {
					if manifest.CriticReport == nil {
						return fmt.Errorf("prepare-wave: E37 critic gate: no critic report found — run 'sawtools run-critic' first or use --skip-critic")
					}
					return fmt.Errorf("prepare-wave: E37 critic gate: ISSUES verdict contains blocking errors — review critic report and fix errors before proceeding")
				}
			}

			// Determine project root from manifest path or --repo-dir flag
			projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(manifestPath)))
			if repoDir != "" {
				projectRoot = repoDir
			}

			// Build engine options
			opts := engine.PrepareWaveOpts{
				IMPLPath:         manifestPath,
				RepoPath:         projectRoot,
				WaveNum:          waveNum,
				MergeTarget:      mergeTarget,
				NoCache:          noCache,
				CommitBaseline:   commitBaseline,
				CommitState:      commitState,
				NoWorkspaceSetup: noWorkspaceSetup || noGoWork,
				Logger:           newSawLogger(),
				OnEvent: func(step string, status string, detail string) {
					if !jsonOnly {
						fmt.Fprintf(os.Stderr, "prepare-wave: [%s] %s — %s\n", step, status, detail)
					}
				},
			}

			// Auto-install M4 pre-commit lint gate hook if missing
			if err := runInstallHooks(projectRoot); err != nil {
				fmt.Fprintf(os.Stderr, "prepare-wave: could not auto-install M4 hook: %v\n", err)
			}

			// Call engine
			result, err := engine.PrepareWave(context.Background(), opts)
			if err != nil {
				// Check if this is a baseline gate failure
				if result != nil {
					for _, step := range result.Steps {
						if step.Step == "baseline_gates" && step.Status == "failed" && step.Data != nil {
							// Extract BaselineData and format diagnostics
							if baselineData, ok := step.Data.(*protocol.BaselineData); ok {
								fmt.Fprintln(os.Stderr, FormatBaselineOutput(baselineData))
							}
						}
					}
					// Output partial result for full diagnostics
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
	cmd.Flags().BoolVar(&commitBaseline, "commit-baseline", false, "Auto-commit baseline fixes if working directory is dirty")
	cmd.Flags().BoolVar(&commitState, "commit-state", true, "Auto-commit SAW-owned state changes (IMPL yaml, gate-cache, docs/IMPL/, docs/CONTEXT.md) before working-dir check. Does not commit user code. Use --commit-state=false to disable.")
	cmd.Flags().BoolVar(&jsonOnly, "json-only", false, "Suppress progress messages (only output JSON result)")
	cmd.Flags().BoolVar(&skipCritic, "skip-critic", false, "Auto-skip E37 critic gate if no critic report exists")
	cmd.Flags().BoolVar(&noGoWork, "no-gowork", false, "Skip go.work setup for LSP cross-package resolution (Go repos only)")
	cmd.Flags().BoolVar(&noWorkspaceSetup, "no-workspace-setup", false,
		"Skip all WorkspaceManager setup (go.work, tsconfig.json, Cargo.toml, pyrightconfig.json)")

	return cmd
}
