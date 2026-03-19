package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/builddiag"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// FinalizeWaveResult combines all post-agent verification, merge, and cleanup results.
// For cross-repo IMPLs, contains aggregated results from all repos.
type FinalizeWaveResult struct {
	Wave              int                              `json:"wave"`
	CrossRepo         bool                             `json:"cross_repo"`
	VerifyCommits     map[string]*protocol.VerifyCommitsResult `json:"verify_commits"` // repo -> result
	StubReport        *protocol.ScanStubsResult        `json:"stub_report,omitempty"`
	GateResults       map[string][]protocol.GateResult `json:"gate_results"` // repo -> gates
	MergeResult       map[string]*protocol.MergeAgentsResult `json:"merge_result"` // repo -> result
	VerifyBuildResult map[string]*protocol.VerifyBuildResult `json:"verify_build"` // repo -> result
	IntegrationReport *protocol.IntegrationReport       `json:"integration_report,omitempty"`
	BuildDiagnosis    map[string]*builddiag.Diagnosis  `json:"build_diagnosis,omitempty"` // repo -> diagnosis
	CleanupResult     map[string]*protocol.CleanupResult `json:"cleanup_result"` // repo -> result
	Success           bool                             `json:"success"`
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
3.5. ValidateIntegration - detect unconnected exports (E25, informational)
4. MergeAgents - merge all agent branches to main
5. VerifyBuild - run test_command and lint_command
6. Cleanup - remove worktrees and branches

Stops on first failure and returns partial result.

I1 Integration: When verify-build fails, automatically diagnoses the error using
pattern matching (H7) and appends diagnosis to the output.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Load manifest to detect cross-repo IMPL
			manifest, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("finalize-wave: failed to load manifest: %w", err)
			}

			// Extract unique repos from file_ownership
			repos, _ := extractReposFromManifest(manifest, waveNum, repoDir)

			result := &FinalizeWaveResult{
				Wave:              waveNum,
				CrossRepo:         len(repos) > 1,
				Success:           false,
				VerifyCommits:     make(map[string]*protocol.VerifyCommitsResult),
				GateResults:       make(map[string][]protocol.GateResult),
				MergeResult:       make(map[string]*protocol.MergeAgentsResult),
				VerifyBuildResult: make(map[string]*protocol.VerifyBuildResult),
				BuildDiagnosis:    make(map[string]*builddiag.Diagnosis),
				CleanupResult:     make(map[string]*protocol.CleanupResult),
			}

			// Step 1: VerifyCommits (E7 check) - run per repo
			for repoKey, repoPath := range repos {
				verifyResult, err := protocol.VerifyCommits(manifestPath, waveNum, repoPath)
				if err != nil {
					return fmt.Errorf("finalize-wave: verify-commits failed in %s: %w", repoKey, err)
				}
				result.VerifyCommits[repoKey] = verifyResult
				if !verifyResult.AllValid {
					// Stop immediately if any agent has no commits (I5 violation)
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("finalize-wave: verify-commits found agents with no commits in %s", repoKey)
				}
			}

			// Step 2: ScanStubs (E20 stub detection)
			// Collect changed files from completion reports (aggregated across all repos)
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

			// Step 3: RunGates (E21 quality gates) with caching - run per repo
			for repoKey, repoPath := range repos {
				stateDir := filepath.Join(repoPath, ".saw-state")
				cache := gatecache.New(stateDir, gatecache.DefaultTTL)
				gateResults, err := protocol.RunGatesWithCache(manifest, waveNum, repoPath, cache)
				if err != nil {
					return fmt.Errorf("finalize-wave: run-gates failed in %s: %w", repoKey, err)
				}
				result.GateResults[repoKey] = gateResults

				// Check if any required gates failed
				for _, gate := range gateResults {
					if gate.Required && !gate.Passed {
						// Stop if required gate failed
						out, _ := json.MarshalIndent(result, "", "  ")
						fmt.Println(string(out))
						return fmt.Errorf("finalize-wave: required gate '%s' failed in %s", gate.Type, repoKey)
					}
				}
			}

			// Step 3.5: ValidateIntegration (E25) — informational
			// Uses the first repo path for single-repo; for cross-repo, uses first entry.
			// Integration validation does NOT block merge — gaps are reported for the
			// human orchestrator (CLI) or integration agent (engine/web) to address.
			for _, repoPath := range repos {
				integrationReport, intErr := protocol.ValidateIntegration(manifest, waveNum, repoPath)
				if intErr != nil {
					fmt.Fprintf(os.Stderr, "finalize-wave: validate-integration: %v\n", intErr)
				} else if integrationReport != nil {
					result.IntegrationReport = integrationReport
					if !integrationReport.Valid {
						fmt.Fprintf(os.Stderr, "finalize-wave: %d integration gap(s) detected\n", len(integrationReport.Gaps))
					}
				}
				break // single-repo default; cross-repo would need aggregation
			}

			// Step 4: MergeAgents (with E9 idempotency check) - run per repo
			for repoKey, repoPath := range repos {
				mergeResult, err := protocol.MergeAgents(manifestPath, waveNum, repoPath)
				if err != nil {
					return fmt.Errorf("finalize-wave: merge-agents failed in %s: %w", repoKey, err)
				}
				result.MergeResult[repoKey] = mergeResult
				if !mergeResult.Success {
					// Merge conflict - stop here, do NOT run VerifyBuild or Cleanup
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("finalize-wave: merge-agents encountered conflicts in %s", repoKey)
				}
			}

			// Step 5: VerifyBuild (post-merge tests) - run per repo
			for repoKey, repoPath := range repos {
				verifyBuildResult, err := protocol.VerifyBuild(manifestPath, repoPath)
				if err != nil {
					return fmt.Errorf("finalize-wave: verify-build failed in %s: %w", repoKey, err)
				}
				result.VerifyBuildResult[repoKey] = verifyBuildResult
				if !verifyBuildResult.TestPassed || !verifyBuildResult.LintPassed {
					// I1: Auto-diagnose build failure using H7
					language := inferLanguageFromCommand(manifest.TestCommand)
					if language != "" {
						// Combine test and lint output for diagnosis
						errorLog := verifyBuildResult.TestOutput + "\n" + verifyBuildResult.LintOutput
						diagnosis, diagErr := builddiag.DiagnoseError(errorLog, language)
						if diagErr == nil {
							result.BuildDiagnosis[repoKey] = diagnosis
						}
						// Note: diagnosis errors are non-fatal - we still return the build failure
					}

					// Post-merge tests failed - stop before cleanup
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("finalize-wave: verify-build failed in %s (test_passed=%v, lint_passed=%v)",
						repoKey, verifyBuildResult.TestPassed, verifyBuildResult.LintPassed)
				}
			}

			// Step 6: Cleanup (remove worktrees and branches) - run per repo
			for repoKey, repoPath := range repos {
				cleanupResult, err := protocol.Cleanup(manifestPath, waveNum, repoPath)
				if err != nil {
					return fmt.Errorf("finalize-wave: cleanup failed in %s: %w", repoKey, err)
				}
				result.CleanupResult[repoKey] = cleanupResult
			}

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

// extractReposFromManifest extracts unique repos from file_ownership and returns:
// - repos: map of repo key -> absolute path
// - agentRepos: map of agent ID -> repo key
func extractReposFromManifest(m *protocol.IMPLManifest, waveNum int, defaultRepo string) (map[string]string, map[string]string) {
	repos := make(map[string]string)
	agentRepos := make(map[string]string)

	// Find the target wave
	var targetWave *protocol.Wave
	for i := range m.Waves {
		if m.Waves[i].Number == waveNum {
			targetWave = &m.Waves[i]
			break
		}
	}
	if targetWave == nil {
		// Wave not found - return default repo
		repos["."] = defaultRepo
		return repos, agentRepos
	}

	// Extract repo paths from file_ownership
	repoSet := make(map[string]bool)
	for _, ownership := range m.FileOwnership {
		if ownership.Wave == waveNum {
			if ownership.Repo != "" {
				repoSet[ownership.Repo] = true
				agentRepos[ownership.Agent] = ownership.Repo
			}
		}
	}

	// If no repos specified, use default (single-repo IMPL)
	if len(repoSet) == 0 {
		repos["."] = defaultRepo
		for _, agent := range targetWave.Agents {
			agentRepos[agent.ID] = "."
		}
		return repos, agentRepos
	}

	// Resolve relative paths to absolute paths.
	// defaultRepo may be relative (e.g. "." from --repo-dir flag) — resolve it first
	// so parentDir is always an absolute path, preventing repo names like "scout-and-wave"
	// from resolving to "./scout-and-wave" instead of "/abs/path/to/scout-and-wave".
	absDefaultRepo, err := filepath.Abs(defaultRepo)
	if err != nil {
		absDefaultRepo = defaultRepo
	}
	parentDir := filepath.Dir(absDefaultRepo)
	for repo := range repoSet {
		absPath := repo
		if !filepath.IsAbs(repo) {
			absPath = filepath.Join(parentDir, repo)
		}
		repos[repo] = absPath
	}

	return repos, agentRepos
}

// inferLanguageFromCommand infers the language from test_command using simple heuristics.
// Returns "go", "rust", "javascript", "python", or "" if unrecognized.
func inferLanguageFromCommand(testCommand string) string {
	lower := strings.ToLower(testCommand)

	// Check for language-specific commands (order matters - check more specific patterns first)
	// Rust must be checked before Go since "cargo" contains "go"
	if strings.Contains(lower, "cargo test") || strings.Contains(lower, "cargo build") {
		return "rust"
	}
	if strings.Contains(lower, "go test") || strings.Contains(lower, "go build") {
		return "go"
	}
	if strings.Contains(lower, "npm test") || strings.Contains(lower, "jest") ||
	   strings.Contains(lower, "vitest") || strings.Contains(lower, "mocha") {
		return "javascript"
	}
	if strings.Contains(lower, "pytest") || strings.Contains(lower, "python -m unittest") {
		return "python"
	}

	return "" // Unknown language
}
