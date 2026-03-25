package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	VerifyCommits     map[string]*protocol.VerifyCommitsData `json:"verify_commits"` // repo -> result
	CollisionReports  map[string]*collision.CollisionReport `json:"collision_reports,omitempty"` // repo -> collision report
	StubReport        *protocol.ScanStubsData          `json:"stub_report,omitempty"`
	GateResults       map[string][]protocol.GateResult `json:"gate_results"` // repo -> gates
	MergeResult       map[string]*protocol.MergeAgentsData `json:"merge_result"` // repo -> result
	VerifyBuildResult map[string]*protocol.VerifyBuildData `json:"verify_build"` // repo -> result
	IntegrationReport        *protocol.IntegrationReport       `json:"integration_report,omitempty"`
	WiringReport             *protocol.WiringValidationData    `json:"wiring_report,omitempty"`
	BuildDiagnosis           map[string]*builddiag.Diagnosis   `json:"build_diagnosis,omitempty"` // repo -> diagnosis
	CleanupResult            map[string]*protocol.CleanupData  `json:"cleanup_result"` // repo -> result
	IntegrationActionRequired bool                             `json:"integration_action_required"`
	WiringGaps               []string                          `json:"wiring_gaps,omitempty"`
	Success                  bool                             `json:"success"`
}

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

			// Extract unique repos from file_ownership.
			// Prefer --repo-dir flag; fall back to current directory for single-repo IMPLs.
			defaultRepoPath := repoDir
			if defaultRepoPath == "" {
				defaultRepoPath, _ = os.Getwd()
			}
			repos, _ := extractReposFromManifest(manifest, waveNum, defaultRepoPath)

			result := &FinalizeWaveResult{
				Wave:              waveNum,
				CrossRepo:         len(repos) > 1,
				Success:           false,
				VerifyCommits:     make(map[string]*protocol.VerifyCommitsData),
				CollisionReports:  make(map[string]*collision.CollisionReport),
				GateResults:       make(map[string][]protocol.GateResult),
				MergeResult:       make(map[string]*protocol.MergeAgentsData),
				VerifyBuildResult: make(map[string]*protocol.VerifyBuildData),
				BuildDiagnosis:    make(map[string]*builddiag.Diagnosis),
				CleanupResult:     make(map[string]*protocol.CleanupData),
			}

			// CLI event callback: print step progress to stderr.
			onEvent := engine.EventCallback(func(step, status, detail string) {
				if detail != "" {
					fmt.Fprintf(os.Stderr, "finalize-wave: [%s] %s — %s\n", step, status, detail)
				} else {
					fmt.Fprintf(os.Stderr, "finalize-wave: [%s] %s\n", step, status)
				}
			})

			// Declare variables here to avoid "goto jumps over variable declaration" compile errors.
			var isSolo, allGone bool

			// If --skip-merge flag is set, skip VerifyCommits and MergeAgents (merge already done manually)
			if skipMerge {
				fmt.Fprintf(os.Stderr, "finalize-wave: --skip-merge flag set, skipping to verify-build\n")
				for repoKey := range repos {
					result.MergeResult[repoKey] = &protocol.MergeAgentsData{
						Wave: waveNum, Success: true,
					}
				}
				// Warning if worktrees still exist
				for _, repoPath := range repos {
					if !protocol.WorktreesAbsent(manifest, waveNum, repoPath) {
						fmt.Fprintf(os.Stderr, "finalize-wave: warning: --skip-merge used but worktrees detected; cleanup will remove them\n")
					}
				}
				goto postMerge
			}

			// Solo wave: if the manifest declares exactly 1 agent in this wave AND
			// no worktrees exist, this was a solo-agent wave (no isolation needed).
			// Skip VerifyCommits and MergeAgents.
			// CRITICAL: Do NOT use worktree absence alone — worktrees may have been
			// cleaned up by agents or a prior failed finalize while branches still
			// need merging.
			{
				var waveAgentCount int
				for _, w := range manifest.Waves {
					if w.Number == waveNum {
						waveAgentCount = len(w.Agents)
						break
					}
				}
				if waveAgentCount <= 1 {
					for _, repoPath := range repos {
						if protocol.WorktreesAbsent(manifest, waveNum, repoPath) {
							isSolo = true
							break
						}
					}
				}
			}
			if isSolo {
				fmt.Fprintf(os.Stderr, "finalize-wave: solo wave detected (1 agent, no worktrees), skipping VerifyCommits and MergeAgents\n")
				for repoKey := range repos {
					result.MergeResult[repoKey] = &protocol.MergeAgentsData{
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
				// Safety check: absent branches do NOT prove the work was merged.
				// Branches can be deleted without their commits ever reaching main.
				// Verify each agent's commit SHA (from completion reports) is reachable
				// from HEAD before accepting the "already merged" shortcut.
				for _, w := range manifest.Waves {
					if w.Number != waveNum {
						continue
					}
					for _, agent := range w.Agents {
						report, hasReport := manifest.CompletionReports[agent.ID]
						if !hasReport || report.Commit == "" {
							continue
						}
						for _, repoPath := range repos {
							checkCmd := exec.Command("git", "-C", repoPath, "merge-base", "--is-ancestor", report.Commit, "HEAD")
							if err := checkCmd.Run(); err != nil {
								out, _ := json.MarshalIndent(result, "", "  ")
								fmt.Println(string(out))
								return fmt.Errorf(
									"finalize-wave: agent %s commit %s is NOT reachable from main in %s — "+
										"branches deleted without merging (data loss). "+
										"Recover with: git branch recover-%s %s",
									agent.ID, report.Commit, repoPath, agent.ID, report.Commit)
							}
						}
					}
					break
				}
				fmt.Fprintf(os.Stderr,
					"finalize-wave: all agent branches absent (already merged+cleaned), skipping to verify-build\n")
				for repoKey := range repos {
					result.MergeResult[repoKey] = &protocol.MergeAgentsData{
						Wave: waveNum, Success: true,
					}
				}
				goto postMerge
			}

			// Step 1: VerifyCommits (I5) — delegates to engine.StepVerifyCommits per repo
			for repoKey, repoPath := range repos {
				stepOpts := engine.FinalizeWaveOpts{
					IMPLPath: manifestPath,
					RepoPath: repoPath,
					WaveNum:  waveNum,
					Logger:   newSawLogger(),
				}
				stepResult, stepErr := engine.StepVerifyCommits(cmd.Context(), stepOpts, onEvent)
				if stepErr != nil {
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("finalize-wave: verify-commits failed in %s: %v", repoKey, stepErr)
				}
				if verifyData, ok := stepResult.Data.(protocol.VerifyCommitsData); ok {
					result.VerifyCommits[repoKey] = &verifyData
				}
			}

			// Step 1.1: I4 — Verify completion reports exist for every agent.
			// CLI-only: the engine step functions don't check completion reports.
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
			// CLI-only: block merge if any agent in this wave reports partial or blocked.
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
			// CLI-only: collision detection is not in the engine step functions.
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

			// Step 2: ScanStubs (E20) — delegates to engine.StepScanStubs
			{
				// Use first repo for scan stubs (aggregated across all repos via manifest)
				var firstRepoPath string
				for _, rp := range repos {
					firstRepoPath = rp
					break
				}
				stepOpts := engine.FinalizeWaveOpts{
					IMPLPath: manifestPath,
					RepoPath: firstRepoPath,
					WaveNum:  waveNum,
					Logger:   newSawLogger(),
				}
				stepResult, stepErr := engine.StepScanStubs(cmd.Context(), stepOpts, manifest, onEvent)
				if stepErr != nil {
					return fmt.Errorf("finalize-wave: scan-stubs failed: %v", stepErr)
				}
				if stubData, ok := stepResult.Data.(*protocol.ScanStubsData); ok {
					result.StubReport = stubData
				} else if stubData, ok := stepResult.Data.(protocol.ScanStubsData); ok {
					result.StubReport = &stubData
				}
			}

			// Step 3: RunPreMergeGates (E21) — structural checks that don't require merged state
			// CLI-only: uses RunPreMergeGates (not RunGatesWithCache), has C2 closed-loop retry
			for repoKey, repoPath := range repos {
				stateDir := filepath.Join(repoPath, ".saw-state")
				cache := gatecache.New(stateDir, gatecache.DefaultTTL)
				gateRes := protocol.RunPreMergeGates(manifest, waveNum, repoPath, cache)
				if !gateRes.IsSuccess() {
					return fmt.Errorf("finalize-wave: run-pre-merge-gates failed in %s: %v", repoKey, gateRes.Errors)
				}
				gateResults := gateRes.GetData().Gates
				result.GateResults[repoKey] = gateResults

				for _, gate := range gateResults {
					if gate.Required && !gate.Passed {
						// C2: Attempt closed-loop gate retry before giving up.
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
							cache = gatecache.New(stateDir, gatecache.DefaultTTL)
							rerunRes := protocol.RunPreMergeGates(manifest, waveNum, repoPath, cache)
							if rerunRes.IsSuccess() {
								rerunResults := rerunRes.GetData().Gates
								result.GateResults[repoKey] = rerunResults
								stillFailing := false
								for _, rg := range rerunResults {
									if rg.Required && !rg.Passed {
										stillFailing = true
										break
									}
								}
								if !stillFailing {
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

			// Step 3.5a: ValidateIntegration (E25) — delegates to engine.StepValidateIntegration
			for _, repoPath := range repos {
				stepOpts := engine.FinalizeWaveOpts{
					IMPLPath: manifestPath,
					RepoPath: repoPath,
					WaveNum:  waveNum,
					Logger:   newSawLogger(),
				}
				_, integrationReport, _ := engine.StepValidateIntegration(cmd.Context(), stepOpts, manifest, onEvent)
				if integrationReport != nil {
					result.IntegrationReport = integrationReport
					if !integrationReport.Valid {
						fmt.Fprintf(os.Stderr, "finalize-wave: %d integration gap(s) detected\n", len(integrationReport.Gaps))
					}
				}
				break // single-repo default; cross-repo would need aggregation
			}

			// Step 3.5b: Wiring declaration check (E35 Layer 3B)
			// CLI-only: wiring checks are not in the engine step functions.
			if len(manifest.Wiring) > 0 {
				for _, repoPath := range repos {
					wiringRes := protocol.ValidateWiringDeclarations(manifest, repoPath)
					if wiringRes.IsFatal() {
						fmt.Fprintf(os.Stderr, "finalize-wave: wiring check error: %v\n", wiringRes.Errors)
					} else {
						wiringData := wiringRes.GetData()
						result.WiringReport = wiringData
						if !wiringData.Valid {
							for _, gap := range wiringData.Gaps {
								fmt.Fprintf(os.Stderr, "finalize-wave: WIRING GAP [error]: %s not called in %s\n",
									gap.Declaration.Symbol, gap.Declaration.MustBeCalledFrom)
							}
							result.IntegrationActionRequired = true
							for _, gap := range wiringData.Gaps {
								result.WiringGaps = append(result.WiringGaps,
									fmt.Sprintf("%s not called in %s",
										gap.Declaration.Symbol, gap.Declaration.MustBeCalledFrom))
							}
						}
					}
					break // single-repo default; cross-repo would need aggregation
				}
			}

			// Step 4: MergeAgents (with E9 idempotency check) - run per repo
			// Skip for integration waves (type: integration) — agents commit directly to main.
			if manifest.Waves[waveNum-1].Type == "integration" {
				goto postMerge
			}
			for repoKey, repoPath := range repos {
				mergeRes, err := protocol.MergeAgents(manifestPath, waveNum, repoPath, mergeTarget)
				if err != nil {
					return fmt.Errorf("finalize-wave: merge-agents failed in %s: %w", repoKey, err)
				}
				if !mergeRes.IsSuccess() {
					return fmt.Errorf("finalize-wave: merge-agents failed in %s: %v", repoKey, mergeRes.Errors)
				}
				mergeResult := mergeRes.GetData()
				result.MergeResult[repoKey] = &mergeResult
				if !mergeResult.Success {
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("finalize-wave: merge-agents encountered conflicts in %s", repoKey)
				}
			}
			postMerge:

			// Step 4.5: M5 populate-integration-checklist (non-blocking)
			m5Updated, m5Err := protocol.PopulateIntegrationChecklist(manifest)
			if m5Err != nil {
				fmt.Fprintf(os.Stderr, "finalize-wave: M5 populate-integration-checklist: %v\n", m5Err)
			} else {
				manifest = m5Updated
				if saveErr := protocol.Save(manifest, manifestPath); saveErr != nil {
					fmt.Fprintf(os.Stderr, "finalize-wave: M5 save: %v\n", saveErr)
				} else {
					fmt.Fprintf(os.Stderr, "finalize-wave: M5 integration checklist populated\n")
				}
			}

			// Step 5: VerifyBuild (post-merge tests) — delegates to engine.StepVerifyBuild per repo
			for repoKey, repoPath := range repos {
				stepOpts := engine.FinalizeWaveOpts{
					IMPLPath: manifestPath,
					RepoPath: repoPath,
					WaveNum:  waveNum,
					Logger:   newSawLogger(),
				}
				stepResult, stepErr := engine.StepVerifyBuild(cmd.Context(), stepOpts, onEvent)
				if stepErr != nil {
					// Extract build data from step result for the CLI result struct
					if stepResult != nil && stepResult.Data != nil {
						if verifyData, ok := stepResult.Data.(protocol.VerifyBuildData); ok {
							result.VerifyBuildResult[repoKey] = &verifyData
						} else if dataMap, ok := stepResult.Data.(map[string]interface{}); ok {
							if vb, ok := dataMap["verify_build"]; ok {
								if verifyData, ok := vb.(protocol.VerifyBuildData); ok {
									result.VerifyBuildResult[repoKey] = &verifyData
								}
							}
							if d, ok := dataMap["diagnosis"]; ok {
								if diagnosis, ok := d.(*builddiag.Diagnosis); ok {
									result.BuildDiagnosis[repoKey] = diagnosis
								}
							}
						}
					}
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("finalize-wave: verify-build failed in %s: %v", repoKey, stepErr)
				}
				if stepResult != nil && stepResult.Data != nil {
					if verifyData, ok := stepResult.Data.(protocol.VerifyBuildData); ok {
						result.VerifyBuildResult[repoKey] = &verifyData
					}
				}
			}

			// Step 5.5: RunPostMergeGates (E21) — content/integration checks on merged state
			for repoKey, repoPath := range repos {
				postGateRes := protocol.RunPostMergeGates(manifest, waveNum, repoPath)
				if !postGateRes.IsSuccess() {
					return fmt.Errorf("finalize-wave: run-post-merge-gates failed in %s: %v", repoKey, postGateRes.Errors)
				}
				postGateResults := postGateRes.GetData().Gates
				result.GateResults[repoKey] = append(result.GateResults[repoKey], postGateResults...)

				for _, gate := range postGateResults {
					if gate.Required && !gate.Passed {
						out, _ := json.MarshalIndent(result, "", "  ")
						fmt.Println(string(out))
						return fmt.Errorf("finalize-wave: required post-merge gate '%s' failed in %s", gate.Type, repoKey)
					}
				}
			}

			// Step 6: Cleanup — delegates to engine.StepCleanup per repo
			for repoKey, repoPath := range repos {
				stepOpts := engine.FinalizeWaveOpts{
					IMPLPath: manifestPath,
					RepoPath: repoPath,
					WaveNum:  waveNum,
					Logger:   newSawLogger(),
				}
				stepResult, _ := engine.StepCleanup(cmd.Context(), stepOpts, onEvent)
				if stepResult != nil && stepResult.Data != nil {
					if cleanupData, ok := stepResult.Data.(protocol.CleanupData); ok {
						result.CleanupResult[repoKey] = &cleanupData
					}
				}
			}

			// State transition: WAVE_VERIFIED (non-fatal)
			stateRes := protocol.SetImplState(manifestPath, protocol.StateWaveVerified, protocol.SetImplStateOpts{})
			if stateRes.IsFatal() {
				fmt.Fprintf(os.Stderr, "finalize-wave: warning: SetImplState(WAVE_VERIFIED) failed: %v\n", stateRes.Errors)
			} else {
				fmt.Fprintf(os.Stderr, "finalize-wave: IMPL state -> WAVE_VERIFIED\n")
			}

			// Per-agent update-status: complete (non-fatal)
			for _, agent := range manifest.Waves[waveNum-1].Agents {
				updateRes := protocol.UpdateStatus(manifestPath, waveNum, agent.ID, "complete")
				if updateRes.IsFatal() {
					fmt.Fprintf(os.Stderr, "finalize-wave: warning: update-status for agent %s failed: %v\n",
						agent.ID, updateRes.Errors)
				}
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
	cmd.Flags().BoolVar(&skipMerge, "skip-merge", false, "Skip merge step (merge already done manually); start from verify-build")

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
