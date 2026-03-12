package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// FinalizeWaveResult combines all post-agent verification, merge, and cleanup results.
type FinalizeWaveResult struct {
	Wave              int                         `json:"wave"`
	VerifyCommits     *protocol.VerifyCommitsResult `json:"verify_commits"`
	StubReport        *protocol.ScanStubsResult     `json:"stub_report,omitempty"`
	GateResults       []protocol.GateResult         `json:"gate_results"`
	MergeResult       *protocol.MergeAgentsResult   `json:"merge_result"`
	VerifyBuildResult *protocol.VerifyBuildResult   `json:"verify_build"`
	CleanupResult     *protocol.CleanupResult       `json:"cleanup_result"`
	Success           bool                          `json:"success"`
}

func newFinalizeWaveCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "finalize-wave <manifest-path>",
		Short: "Finalize wave: verify, scan stubs, run gates, merge, verify build, and cleanup",
		Long: `Combines the post-agent verification and merge workflow into a single atomic operation.
Execution order:
1. VerifyCommits - check all agents have commits (I5 trip wire)
2. ScanStubs - scan changed files for TODO/FIXME markers (E20)
3. RunGates - execute quality gates if defined (E21)
4. MergeAgents - merge all agent branches to main
5. VerifyBuild - run test_command and lint_command
6. Cleanup - remove worktrees and branches

Stops on first failure and returns partial result.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			result := &FinalizeWaveResult{
				Wave:    waveNum,
				Success: false,
			}

			// Step 1: VerifyCommits (E7 check)
			verifyResult, err := protocol.VerifyCommits(manifestPath, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("finalize-wave: verify-commits failed: %w", err)
			}
			result.VerifyCommits = verifyResult
			if !verifyResult.AllValid {
				// Stop immediately if any agent has no commits (I5 violation)
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(out))
				return fmt.Errorf("finalize-wave: verify-commits found agents with no commits")
			}

			// Step 2: ScanStubs (E20 stub detection)
			// Collect changed files from completion reports
			manifest, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("finalize-wave: failed to load manifest: %w", err)
			}

			var changedFiles []string
			for _, agent := range manifest.Waves[waveNum-1].Agents {
				if report, ok := manifest.CompletionReports[agent.ID]; ok {
					changedFiles = append(changedFiles, report.FilesChanged...)
					changedFiles = append(changedFiles, report.FilesCreated...)
				}
			}

			if len(changedFiles) > 0 {
				stubResult, err := protocol.ScanStubs(changedFiles)
				if err != nil {
					return fmt.Errorf("finalize-wave: scan-stubs failed: %w", err)
				}
				result.StubReport = stubResult
			}

			// Step 3: RunGates (E21 quality gates)
			gateResults, err := protocol.RunGates(manifest, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("finalize-wave: run-gates failed: %w", err)
			}
			result.GateResults = gateResults

			// Check if any required gates failed
			for _, gate := range gateResults {
				if gate.Required && !gate.Passed {
					// Stop if required gate failed
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("finalize-wave: required gate '%s' failed", gate.Type)
				}
			}

			// Step 4: MergeAgents
			mergeResult, err := protocol.MergeAgents(manifestPath, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("finalize-wave: merge-agents failed: %w", err)
			}
			result.MergeResult = mergeResult
			if !mergeResult.Success {
				// Merge conflict - stop here, do NOT run VerifyBuild or Cleanup
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(out))
				return fmt.Errorf("finalize-wave: merge-agents encountered conflicts")
			}

			// Step 5: VerifyBuild (post-merge tests)
			verifyBuildResult, err := protocol.VerifyBuild(manifestPath, repoDir)
			if err != nil {
				return fmt.Errorf("finalize-wave: verify-build failed: %w", err)
			}
			result.VerifyBuildResult = verifyBuildResult
			if !verifyBuildResult.TestPassed || !verifyBuildResult.LintPassed {
				// Post-merge tests failed - stop before cleanup
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(out))
				return fmt.Errorf("finalize-wave: verify-build failed (test_passed=%v, lint_passed=%v)",
					verifyBuildResult.TestPassed, verifyBuildResult.LintPassed)
			}

			// Step 6: Cleanup (remove worktrees and branches)
			cleanupResult, err := protocol.Cleanup(manifestPath, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("finalize-wave: cleanup failed: %w", err)
			}
			result.CleanupResult = cleanupResult

			// All steps succeeded
			result.Success = true
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
