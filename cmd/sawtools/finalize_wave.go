package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/spf13/cobra"
)

func newFinalizeWaveCmd() *cobra.Command {
	var waveNum int
	var mergeTarget string
	var skipMerge bool

	cmd := &cobra.Command{
		Use:   "finalize-wave <manifest-path>",
		Short: "Finalize wave: verify, scan stubs, run gates, merge, verify build, and cleanup",
		Long: `Combines the post-agent verification and merge workflow into a single atomic operation.
Execution order:
1. VerifyCommits - check all agents have commits (I5 trip wire)
1.5. CheckTypeCollisions - detect type name collisions across agent branches (blocking)
2. ScanStubs - scan changed files for TODO/FIXME markers (E20)
3. RunPreMergeGates - structural gates that don't require merged state (E21)
3.5. ValidateIntegration - detect unconnected exports (E25, informational)
4. MergeAgents - merge all agent branches to main
5. VerifyBuild - run test_command and lint_command
5.5. RunPostMergeGates - content/integration gates on merged state (E21)
6. Cleanup - remove worktrees and branches

Stops on first failure and returns partial result.

All pipeline steps are handled by the engine. The engine supports:
- Solo-wave shortcut (1 agent, no worktrees)
- AllBranchesAbsent shortcut with ancestry verification
- State transitions (WAVE_VERIFIED)
- Per-agent update-status calls
- C2 closed-loop gate retry
- Multi-repo via file_ownership`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			defaultRepoPath := repoDir
			if defaultRepoPath == "" {
				defaultRepoPath, _ = os.Getwd()
			}
			onEvent := engine.EventCallback(func(step, status, detail string) {
				if detail != "" {
					fmt.Fprintf(os.Stderr, "finalize-wave: [%s] %s — %s\n", step, status, detail)
				} else {
					fmt.Fprintf(os.Stderr, "finalize-wave: [%s] %s\n", step, status)
				}
			})
			r := engine.FinalizeWave(cmd.Context(), engine.FinalizeWaveOpts{
				IMPLPath:                  manifestPath,
				RepoPath:                  defaultRepoPath,
				WaveNum:                   waveNum,
				MergeTarget:               mergeTarget,
				SkipMerge:                 skipMerge,
				Logger:                    newSawLogger(),
				CollisionDetectionEnabled: true,
				ClosedLoopRetryEnabled:    true,
				OnEvent:                   onEvent,
			})
			out, _ := json.MarshalIndent(r.Data, "", "  ")
			fmt.Println(string(out))
			if r.IsFatal() {
				if len(r.Errors) > 0 {
					return fmt.Errorf("finalize-wave: [%s] %s", r.Errors[0].Code, r.Errors[0].Message)
				}
				return fmt.Errorf("finalize-wave failed")
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")
	cmd.Flags().StringVar(&mergeTarget, "merge-target", "", "Target branch for merge (default: current HEAD)")
	cmd.Flags().BoolVar(&skipMerge, "skip-merge", false, "Skip merge step (merge already done manually); start from verify-build")
	return cmd
}
