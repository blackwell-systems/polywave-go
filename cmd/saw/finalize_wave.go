package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/builddiag"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/collision"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
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
	CollisionReports  map[string]*collision.CollisionReport `json:"collision_reports,omitempty"` // repo -> collision report
	StubReport        *protocol.ScanStubsResult        `json:"stub_report,omitempty"`
	GateResults       map[string][]protocol.GateResult `json:"gate_results"` // repo -> gates
	MergeResult       map[string]*protocol.MergeAgentsResult `json:"merge_result"` // repo -> result
	VerifyBuildResult map[string]*protocol.VerifyBuildResult `json:"verify_build"` // repo -> result
	IntegrationReport *protocol.IntegrationReport       `json:"integration_report,omitempty"`
	WiringReport      *protocol.WiringValidationResult `json:"wiring_report,omitempty"`
	BuildDiagnosis    map[string]*builddiag.Diagnosis  `json:"build_diagnosis,omitempty"` // repo -> diagnosis
	CleanupResult     map[string]*protocol.CleanupResult `json:"cleanup_result"` // repo -> result
	Success           bool                             `json:"success"`
}

func newFinalizeWaveCmd() *cobra.Command {
	var waveNum int
	var mergeTarget string

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
				CollisionReports:  make(map[string]*collision.CollisionReport),
				GateResults:       make(map[string][]protocol.GateResult),
				MergeResult:       make(map[string]*protocol.MergeAgentsResult),
				VerifyBuildResult: make(map[string]*protocol.VerifyBuildResult),
				BuildDiagnosis:    make(map[string]*builddiag.Diagnosis),
				CleanupResult:     make(map[string]*protocol.CleanupResult),
			}

			// Declare variables here to avoid "goto jumps over variable declaration" compile errors.
			var changedFiles []string
			var isSolo, allGone bool

			// Solo wave: if no worktrees were created, skip VerifyCommits and MergeAgents.
			for _, repoPath := range repos {
				if protocol.WorktreesAbsent(manifest, waveNum, repoPath) {
					isSolo = true
					break
				}
			}
			if isSolo {
				fmt.Fprintf(os.Stderr, "finalize-wave: solo wave detected (no worktrees), skipping VerifyCommits and MergeAgents\n")
				for repoKey := range repos {
					result.MergeResult[repoKey] = &protocol.MergeAgentsResult{
						Wave: waveNum, Success: true,
					}
				}
				goto postMerge
			}

			// If all agent branches are absent (wave already merged and cleaned up),
			// skip VerifyCommits and MergeAgents entirely.
			for _, repoPath := range repos {
				if protocol.AllBranchesAbsent(manifest, waveNum, repoPath) {
					allGone = true
					break
				}
			}
			if allGone {
				fmt.Fprintf(os.Stderr,
					"finalize-wave: all agent branches absent (already merged+cleaned), skipping to verify-build\n")
				for repoKey := range repos {
					result.MergeResult[repoKey] = &protocol.MergeAgentsResult{
						Wave: waveNum, Success: true,
					}
				}
				goto postMerge
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

			// Step 1.1: I4 — Verify completion reports exist for every agent.
			// The IMPL doc is the single source of truth. If an agent didn't call
			// sawtools set-completion, it has not completed the protocol.
			{
				var missing []string
				for _, agent := range manifest.Waves[waveNum-1].Agents {
					if _, ok := manifest.CompletionReports[agent.ID]; !ok {
						missing = append(missing, agent.ID)
					}
				}
				if len(missing) > 0 {
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("finalize-wave: missing completion reports for agents: %v — agents must call sawtools set-completion before merge (I4)", missing)
				}
			}

			// Step 1.2: E7 — Check completion report statuses before merge.
			// Block merge if any agent in this wave reports partial or blocked.
			for _, agent := range manifest.Waves[waveNum-1].Agents {
				if report, ok := manifest.CompletionReports[agent.ID]; ok {
					if report.Status == "partial" || report.Status == "blocked" {
						out, _ := json.MarshalIndent(result, "", "  ")
						fmt.Println(string(out))
						return fmt.Errorf("finalize-wave: agent %s has status %q — cannot merge wave with partial/blocked agents (E7)",
							agent.ID, report.Status)
					}
				}
			}

			// Step 1.2: E11 — Predict merge conflicts from completion reports.
			if err := protocol.PredictConflictsFromReports(manifest, waveNum); err != nil {
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(out))
				return fmt.Errorf("finalize-wave: %w", err)
			}

			// Step 1.5: Check type collisions (per repo)
			for repoKey, repoPath := range repos {
				collisionReport, err := collision.DetectCollisions(manifestPath, waveNum, repoPath)
				if err != nil {
					return fmt.Errorf("finalize-wave: check-type-collisions failed in %s: %w", repoKey, err)
				}
				result.CollisionReports[repoKey] = &collisionReport
				if !collisionReport.Valid {
					// Stop immediately - collision detected
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("finalize-wave: type collisions detected in %s", repoKey)
				}
			}

			// Step 2: ScanStubs (E20 stub detection)
			// Collect changed files from completion reports (aggregated across all repos)
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

			// Step 3: RunPreMergeGates (E21) — structural checks that don't require merged state
			for repoKey, repoPath := range repos {
				stateDir := filepath.Join(repoPath, ".saw-state")
				cache := gatecache.New(stateDir, gatecache.DefaultTTL)
				gateResults, err := protocol.RunPreMergeGates(manifest, waveNum, repoPath, cache)
				if err != nil {
					return fmt.Errorf("finalize-wave: run-pre-merge-gates failed in %s: %w", repoKey, err)
				}
				result.GateResults[repoKey] = gateResults

				for _, gate := range gateResults {
					if gate.Required && !gate.Passed {
						// C2: Attempt closed-loop gate retry before giving up.
						// Pre-merge gates run per-repo; pick the first agent in this
						// wave (scoped to this repo) as the retry target.
						retryFixed := false
						agents := manifest.Waves[waveNum-1].Agents
						for _, ag := range agents {
							branch := protocol.BranchName(manifest.FeatureSlug, waveNum, ag.ID)
							wtPath := protocol.ResolveWorktreePath(repoPath, branch)
							retryOpts := engine.ClosedLoopRetryOpts{
								IMPLPath:     manifestPath,
								RepoPath:     repoPath,
								WaveNum:      waveNum,
								AgentID:      ag.ID,
								GateType:     gate.Type,
								GateCommand:  gate.Command,
								GateOutput:   gate.Stdout + "\n" + gate.Stderr,
								WorktreePath: wtPath,
								MaxRetries:   2,
							}
							retryResult, retryErr := engine.ClosedLoopGateRetry(cmd.Context(), retryOpts)
							if retryErr != nil {
								fmt.Fprintf(os.Stderr, "finalize-wave: closed-loop retry error for agent %s: %v\n", ag.ID, retryErr)
							} else if retryResult != nil && retryResult.Fixed {
								fmt.Fprintf(os.Stderr, "finalize-wave: closed-loop retry fixed gate '%s' for agent %s after %d attempt(s)\n",
									gate.Type, ag.ID, retryResult.Attempts)
								retryFixed = true
							}
							// Only retry with the first agent — the gate is repo-scoped.
							break
						}
						if retryFixed {
							// Gate was fixed by the retry agent; re-run gates to confirm.
							// Invalidate cache so re-run gets fresh results.
							cache = gatecache.New(stateDir, gatecache.DefaultTTL)
							rerunResults, rerunErr := protocol.RunPreMergeGates(manifest, waveNum, repoPath, cache)
							if rerunErr == nil {
								result.GateResults[repoKey] = rerunResults
								// Check if the gate passes now.
								stillFailing := false
								for _, rg := range rerunResults {
									if rg.Required && !rg.Passed {
										stillFailing = true
										break
									}
								}
								if !stillFailing {
									// All gates pass after retry — continue to next repo.
									break
								}
							}
						}
						out, _ := json.MarshalIndent(result, "", "  ")
						fmt.Println(string(out))
						return fmt.Errorf("finalize-wave: required pre-merge gate '%s' failed in %s", gate.Type, repoKey)
					}
				}
			}

			// Step 3.5a: Heuristic integration scan (E25) — informational
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

			// Step 3.5b: Wiring declaration check (E35 Layer 3B) — blocking if gaps found
			// Checks that each declared wiring symbol is actually called in must_be_called_from.
			// Does NOT block merge — gaps are surfaced as structured errors for the human or
			// integration agent to address via validate-integration post-merge.
			if len(manifest.Wiring) > 0 {
				for _, repoPath := range repos {
					wiringResult, wiringErr := protocol.ValidateWiringDeclarations(manifest, repoPath)
					if wiringErr != nil {
						fmt.Fprintf(os.Stderr, "finalize-wave: wiring check error: %v\n", wiringErr)
					} else {
						result.WiringReport = wiringResult
						if !wiringResult.Valid {
							// Wiring gaps are errors — surface them clearly
							for _, gap := range wiringResult.Gaps {
								fmt.Fprintf(os.Stderr, "finalize-wave: WIRING GAP [error]: %s not called in %s\n",
									gap.Declaration.Symbol, gap.Declaration.MustBeCalledFrom)
							}
							// Do NOT block merge here — the gap is surfaced as a structured
							// error in the result and returned to the caller (web runner, CLI).
							// Blocking at this stage would prevent cleanup. The human or
							// integration agent addresses it via validate-integration.
						}
					}
					break // single-repo default; cross-repo would need aggregation
				}
			}

			// Step 4: MergeAgents (with E9 idempotency check) - run per repo
			// Skip for integration waves (type: integration) — agents commit directly to main.
			if manifest.Waves[waveNum-1].Type == "integration" {
				// Integration wave agents commit directly to main; no branches to merge.
				// Proceed to VerifyBuild.
				goto postMerge
			}
			for repoKey, repoPath := range repos {
				mergeResult, err := protocol.MergeAgents(manifestPath, waveNum, repoPath, mergeTarget)
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
			postMerge:

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

			// Step 5.5: RunPostMergeGates (E21) — content/integration checks on merged state
			for repoKey, repoPath := range repos {
				postGateResults, err := protocol.RunPostMergeGates(manifest, waveNum, repoPath)
				if err != nil {
					return fmt.Errorf("finalize-wave: run-post-merge-gates failed in %s: %w", repoKey, err)
				}
				// Merge post-merge results into GateResults for this repo
				result.GateResults[repoKey] = append(result.GateResults[repoKey], postGateResults...)

				for _, gate := range postGateResults {
					if gate.Required && !gate.Passed {
						out, _ := json.MarshalIndent(result, "", "  ")
						fmt.Println(string(out))
						return fmt.Errorf("finalize-wave: required post-merge gate '%s' failed in %s", gate.Type, repoKey)
					}
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
	cmd.Flags().StringVar(&mergeTarget, "merge-target", "", "Target branch for merge (default: current HEAD)")

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
