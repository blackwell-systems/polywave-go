package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/config"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newFinalizeWaveCmd() *cobra.Command {
	var waveNum int
	var mergeTarget string
	var skipMerge bool
	var dryRun bool
	var crossRepoVerify bool
	var commitState bool

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

			// Auto-detect repo-dir for cross-repo IMPLs when not explicitly set.
			var loadedManifest *protocol.IMPLManifest
			if repoDir == "" || repoDir == "." {
				m, loadErr := protocol.Load(cmd.Context(), manifestPath)
				if loadErr == nil {
					loadedManifest = m
					// Check if any file_ownership for this wave has a Repo field.
					hasCrossRepo := false
					for _, fo := range m.FileOwnership {
						if fo.Wave == waveNum && fo.Repo != "" {
							hasCrossRepo = true
							break
						}
					}
					if hasCrossRepo {
						if detectedPath, detected, repoName, _ := autoDetectRepoDir(manifestPath, waveNum); detected {
							defaultRepoPath = detectedPath
							fmt.Fprintf(os.Stderr, "finalize-wave: auto-detected repo-dir: %s (from saw.config.json -- primary repo for wave %d: %s)\n",
								detectedPath, waveNum, repoName)
						}
					}
				}
			}

			// Pre-flight warning when --repo-dir is explicitly set for a cross-repo IMPL.
			if repoDir != "" && repoDir != "." {
				m := loadedManifest
				if m == nil {
					m, _ = protocol.Load(cmd.Context(), manifestPath)
				}
				if m != nil {
					hasCrossRepo := false
					for _, fo := range m.FileOwnership {
						if fo.Wave == waveNum && fo.Repo != "" {
							hasCrossRepo = true
							break
						}
					}
					if hasCrossRepo && !crossRepoOwnsFiles(m, waveNum, defaultRepoPath) {
						suggestedPath, _, _, _ := autoDetectRepoDir(manifestPath, waveNum)
						return fmt.Errorf("finalize-wave: IMPL is cross-repo but --repo-dir %s owns 0 files for wave %d -- did you mean --repo-dir %s?",
							defaultRepoPath, waveNum, suggestedPath)
					}
				}
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
				CrossRepoVerify:           crossRepoVerify,
			CommitState:               commitState,
			})
			out, _ := json.MarshalIndent(r.Data, "", "  ")
			fmt.Println(string(out))

			// apply-cascade-hotfix step: runs when verify-build fails exclusively
			// due to caller cascade errors in future-wave files.
			if r.Data != nil && r.Data.CallerCascadeOnly && len(r.Data.CallerCascadeErrors) > 0 {
				if dryRun {
					// --dry-run: describe what would be fixed, exit 0.
					dryRunResult := map[string]interface{}{
						"step":        "apply-cascade-hotfix",
						"dry_run":     true,
						"error_count": len(r.Data.CallerCascadeErrors),
						"files":       uniqueFiles(r.Data.CallerCascadeErrors),
						"errors":      r.Data.CallerCascadeErrors,
					}
					dryOut, _ := json.MarshalIndent(dryRunResult, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(dryOut))
					return nil
				}

				fmt.Fprintf(os.Stderr, "finalize-wave: apply-cascade-hotfix: %d error(s) in future-wave files — launching hotfix agent\n",
					len(r.Data.CallerCascadeErrors))
				hotfixRes := engine.RunHotfixAgent(cmd.Context(), engine.RunHotfixAgentOpts{
					IMPLPath: manifestPath,
					RepoPath: defaultRepoPath,
					WaveNum:  waveNum,
					Errors:   r.Data.CallerCascadeErrors,
					Logger:   newSawLogger(),
				}, func(ev engine.Event) {
					if ev.Event == "hotfix_agent_output" {
						if data, ok := ev.Data.(map[string]string); ok {
							fmt.Fprint(os.Stderr, data["chunk"])
						}
					}
				})
				if hotfixRes.IsFatal() {
					errMsg := "hotfix agent failed"
					if len(hotfixRes.Errors) > 0 {
						errMsg = hotfixRes.Errors[0].Message
					}
					return fmt.Errorf("finalize-wave: apply-cascade-hotfix failed: %s", errMsg)
				}
				hotfixData := hotfixRes.GetData()
				if !hotfixData.BuildPassed {
					return fmt.Errorf("finalize-wave: apply-cascade-hotfix: build still fails after hotfix")
				}
				hotfixStepResult := map[string]interface{}{
					"step":         "apply-cascade-hotfix",
					"status":       "success",
					"files_fixed":  hotfixData.FilesFixed,
					"commit":       hotfixData.Commit,
					"build_passed": hotfixData.BuildPassed,
				}
				hotfixOut, _ := json.MarshalIndent(hotfixStepResult, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(hotfixOut))
				fmt.Fprintf(os.Stderr, "finalize-wave: apply-cascade-hotfix complete — build passes\n")
				return nil
			}

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
	cmd.Flags().BoolVar(&crossRepoVerify, "cross-repo-verify", false,
		"After primary repo verification, run baseline gates on all cross-repo dependencies")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Show what cascade errors would be hotfixed without running the agent")
	cmd.Flags().BoolVar(&commitState, "commit-state", false,
		"Auto-commit SAW-owned state files (IMPL docs, .saw-state/) before merge. Prevents dirty-workdir merge failures.")
	return cmd
}

// uniqueFiles returns a deduplicated list of file paths from caller cascade errors.
func uniqueFiles(errors []engine.CallerCascadeError) []string {
	seen := make(map[string]bool)
	var files []string
	for _, e := range errors {
		if !seen[e.File] {
			seen[e.File] = true
			files = append(files, e.File)
		}
	}
	return files
}

// autoDetectRepoDir finds the primary repo for a given wave by counting
// file_ownership entries per repo name, then resolving the name to a path
// via saw.config.json.
// Returns ("", false, "", nil) when auto-detect is not applicable.
func autoDetectRepoDir(manifestPath string, waveNum int) (string, bool, string, error) {
	cfgResult := config.Load(filepath.Dir(manifestPath))
	if !cfgResult.IsSuccess() {
		return "", false, "", nil
	}
	cfg := cfgResult.GetData()

	// Count file_ownership entries per repo name for the given wave.
	m, err := protocol.Load(context.Background(), manifestPath)
	if err != nil {
		return "", false, "", nil
	}

	counts := make(map[string]int)
	for _, fo := range m.FileOwnership {
		if fo.Wave == waveNum && fo.Repo != "" {
			counts[fo.Repo]++
		}
	}
	if len(counts) == 0 {
		return "", false, "", nil
	}

	// Find the repo name with the most entries.
	var bestName string
	var bestCount int
	for name, count := range counts {
		if count > bestCount {
			bestCount = count
			bestName = name
		}
	}

	// Look up that name in cfg.Repos.
	for _, repo := range cfg.Repos {
		if repo.Name == bestName {
			resolved, err := filepath.Abs(repo.Path)
			if err != nil {
				return "", false, "", nil
			}
			return resolved, true, bestName, nil
		}
	}

	return "", false, "", nil
}

// crossRepoOwnsFiles returns true if repoDir owns at least one file
// in file_ownership for the given wave.
func crossRepoOwnsFiles(manifest *protocol.IMPLManifest, waveNum int, repoDir string) bool {
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		absRepoDir = repoDir
	}
	baseName := filepath.Base(absRepoDir)

	hasCrossRepo := false
	for _, fo := range manifest.FileOwnership {
		if fo.Wave == waveNum && fo.Repo != "" {
			hasCrossRepo = true
			if fo.Repo == baseName {
				return true
			}
		}
	}

	// If no entries have Repo set (single-repo IMPL), return true.
	if !hasCrossRepo {
		return true
	}
	return false
}
