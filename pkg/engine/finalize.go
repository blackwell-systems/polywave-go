package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/builddiag"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/collision"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// FinalizeWaveOpts configures a full post-agent finalization pipeline.
type FinalizeWaveOpts struct {
	IMPLPath    string                 // absolute path to IMPL manifest
	RepoPath    string                 // absolute path to the target repository
	WaveNum     int                    // wave number to finalize
	MergeTarget string                 // target branch for merge; empty = HEAD (default)
	ObsEmitter  ObsEmitter   // optional: non-blocking observability emitter
	Logger      *slog.Logger // optional: nil falls back to slog.Default()

	// M2 (E25): When true, unconnected exports in the integration report
	// block the merge with a fatal error. Default (false) preserves the
	// existing informational-only behavior.
	EnforceIntegrationValidation bool

	// M3 (E20): When true, TODO/FIXME stubs detected in changed files
	// block the merge with a fatal error. Default (false) preserves the
	// existing informational-only behavior.
	RequireNoStubs bool

	// CollisionDetectionEnabled: When true, StepCheckTypeCollisions runs before
	// StepMergeAgents. The CLI sets this to true; web app leaves false (default).
	CollisionDetectionEnabled bool

	// ClosedLoopRetryEnabled: When true, after a required gate fails in
	// StepRunGates, engine.ClosedLoopGateRetry is invoked per-agent.
	// The CLI sets this to true; web app leaves false (default).
	ClosedLoopRetryEnabled bool

	// SkipMerge: When true, skip steps 1-4 (VerifyCommits, ScanStubs, RunGates, MergeAgents)
	// and start from verify-build. Used for manual merge escape hatch when E11
	// blocks merge with false positive.
	SkipMerge bool

	// OnEvent is an optional callback invoked at the start and end of each
	// finalization step. CLI callers can print progress; web callers can emit
	// SSE events. nil is safe — step functions check before calling.
	OnEvent EventCallback
}

// FinalizeWaveResult combines all post-agent verification, merge, and cleanup results.
// For single-repo IMPLs, each map has a single key ".".
// For cross-repo IMPLs, keys match the repo names from file_ownership.Repo.
type FinalizeWaveResult struct {
	Wave                      int                                    `json:"wave"`
	CrossRepo                 bool                                   `json:"cross_repo"`
	VerifyCommits             map[string]*protocol.VerifyCommitsData `json:"verify_commits"`
	CollisionReports          map[string]*collision.CollisionReport  `json:"collision_reports,omitempty"`
	StubReport                *protocol.ScanStubsData               `json:"stub_report,omitempty"`
	GateResults               map[string][]protocol.GateResult      `json:"gate_results"`
	IntegrationReport         *protocol.IntegrationReport           `json:"integration_report,omitempty"`
	MergeResult               map[string]*protocol.MergeAgentsData  `json:"merge_result"`
	VerifyBuild               map[string]*protocol.VerifyBuildData  `json:"verify_build"`
	BuildDiagnosis            map[string]*builddiag.Diagnosis        `json:"build_diagnosis,omitempty"`
	CleanupResult             map[string]*protocol.CleanupData      `json:"cleanup_result"`
	WiringReport              *protocol.WiringValidationData        `json:"wiring_report,omitempty"`
	IntegrationActionRequired bool                                  `json:"integration_action_required"`
	WiringGaps                []string                              `json:"wiring_gaps,omitempty"`
	BuildPassed               bool                                  `json:"build_passed"`
	Success                   bool                                  `json:"success"`
}

// firstVerifyBuild returns the VerifyBuildData for the first (or only) repo.
// For single-repo IMPLs, this is equivalent to the old scalar field.
// Returns nil if the map is empty.
func (r *FinalizeWaveResult) firstVerifyBuild() *protocol.VerifyBuildData {
	if len(r.VerifyBuild) == 0 {
		return nil
	}
	// Prefer "." key (single-repo), fall back to first key found
	if v, ok := r.VerifyBuild["."]; ok {
		return v
	}
	for _, v := range r.VerifyBuild {
		return v
	}
	return nil
}

// firstGateResults returns the GateResults for the first (or only) repo.
// For single-repo IMPLs, this is equivalent to the old scalar field.
func (r *FinalizeWaveResult) firstGateResults() []protocol.GateResult {
	if len(r.GateResults) == 0 {
		return nil
	}
	if v, ok := r.GateResults["."]; ok {
		return v
	}
	for _, v := range r.GateResults {
		return v
	}
	return nil
}

// ExtractReposFromManifest extracts unique repos from file_ownership for the
// given wave. Returns repos map (repo key -> absolute path) and agentRepos map
// (agent ID -> repo key). For single-repo IMPLs, returns {"." -> defaultRepo}.
func ExtractReposFromManifest(m *protocol.IMPLManifest, waveNum int, defaultRepo string) (map[string]string, map[string]string) {
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

// FinalizeWave runs the full post-agent finalization pipeline for a single wave.
// This is the engine-level equivalent of the CLI's `sawtools finalize-wave` command.
//
// Pipeline:
//  1. VerifyCommits (I5) - ensure all agents committed work
//  2. ScanStubs (E20) - scan changed files for TODO/FIXME markers
//  3. RunPreMergeGates (E21) - execute structural quality gates
//  3.3. CheckTypeCollisions (E27, opt-in) - detect type name collisions
//  3.5. ValidateIntegration (E25) - scan for unconnected exports (informational)
//  3.6. CheckWiringDeclarations (E35, non-fatal) - verify wiring declarations
//  4. MergeAgents - merge agent branches into main
//  5. VerifyBuild - run test_command and lint_command
//  5.5. RunPostMergeGates (E21) - content/integration gates on merged state
//  6. Cleanup - remove worktrees and branches
//
// Stops on first fatal failure and returns partial result.
func FinalizeWave(ctx context.Context, opts FinalizeWaveOpts) (*FinalizeWaveResult, error) {
	if opts.IMPLPath == "" {
		return nil, fmt.Errorf("engine.FinalizeWave: IMPLPath is required")
	}
	if opts.RepoPath == "" {
		return nil, fmt.Errorf("engine.FinalizeWave: RepoPath is required")
	}

	manifest, err := protocol.Load(opts.IMPLPath)
	if err != nil {
		return nil, fmt.Errorf("engine.FinalizeWave: load manifest: %w", err)
	}

	// Extract repos from manifest for multi-repo support.
	repos, _ := ExtractReposFromManifest(manifest, opts.WaveNum, opts.RepoPath)

	result := &FinalizeWaveResult{
		Wave:             opts.WaveNum,
		CrossRepo:        len(repos) > 1,
		VerifyCommits:    make(map[string]*protocol.VerifyCommitsData),
		CollisionReports: make(map[string]*collision.CollisionReport),
		GateResults:      make(map[string][]protocol.GateResult),
		MergeResult:      make(map[string]*protocol.MergeAgentsData),
		VerifyBuild:      make(map[string]*protocol.VerifyBuildData),
		BuildDiagnosis:   make(map[string]*builddiag.Diagnosis),
		CleanupResult:    make(map[string]*protocol.CleanupData),
	}

	onEvent := opts.OnEvent

	// If SkipMerge is true, skip steps 1-4 and jump directly to verify-build.
	// Populate MergeResult with synthetic entry indicating merge already done.
	if opts.SkipMerge {
		for repoKey := range repos {
			result.MergeResult[repoKey] = &protocol.MergeAgentsData{
				Wave: opts.WaveNum,
			}
		}
		// Warning if worktrees still exist
		for _, repoPath := range repos {
			if !protocol.WorktreesAbsent(manifest, opts.WaveNum, repoPath) {
				loggerFrom(opts.Logger).Warn("engine.FinalizeWave: --skip-merge used but worktrees detected; cleanup will remove them")
			}
		}
	} else {
		// Solo wave: if the manifest declares exactly 1 agent in this wave AND
		// no worktrees exist, this was a solo-agent wave (no isolation needed).
		// Skip VerifyCommits and MergeAgents.
		isSolo := false
		var waveAgentCount int
		for _, w := range manifest.Waves {
			if w.Number == opts.WaveNum {
				waveAgentCount = len(w.Agents)
				break
			}
		}
		if waveAgentCount <= 1 {
			for _, repoPath := range repos {
				if protocol.WorktreesAbsent(manifest, opts.WaveNum, repoPath) {
					isSolo = true
					break
				}
			}
		}

		// allBranchesAbsent: if all agent branches are absent (wave already merged and cleaned up),
		// skip VerifyCommits and MergeAgents entirely.
		allGone := false
		for _, repoPath := range repos {
			if protocol.AllBranchesAbsent(manifest, opts.WaveNum, repoPath) {
				allGone = true
				break
			}
		}

		if isSolo || allGone {
			if allGone && !isSolo {
				// Safety check: absent branches do NOT prove the work was merged.
				// Verify each agent's commit SHA is reachable from HEAD.
				for _, w := range manifest.Waves {
					if w.Number != opts.WaveNum {
						continue
					}
					for _, agent := range w.Agents {
						report, hasReport := manifest.CompletionReports[agent.ID]
						if !hasReport || report.Commit == "" {
							continue
						}
						for _, repoPath := range repos {
							if !git.IsAncestor(repoPath, report.Commit, "HEAD") {
								return result, fmt.Errorf(
									"engine.FinalizeWave: agent %s commit %s is NOT reachable from main in %s — "+
										"branches deleted without merging (data loss). "+
										"Recover with: git branch recover-%s %s",
									agent.ID, report.Commit, repoPath, agent.ID, report.Commit)
							}
						}
					}
					break
				}
			}
			// Populate synthetic merge results for all repos
			for repoKey := range repos {
				result.MergeResult[repoKey] = &protocol.MergeAgentsData{
					Wave: opts.WaveNum,
				}
			}
		} else {
			// Full pipeline: VerifyCommits → ScanStubs → RunPreMergeGates → MergeAgents

			// Step 1: VerifyCommits (I5) — per repo
			for repoKey, repoPath := range repos {
				repoOpts := opts
				repoOpts.RepoPath = repoPath
				verifyStepResult, stepErr := StepVerifyCommits(ctx, repoOpts, onEvent)
				if stepErr != nil {
					return result, fmt.Errorf("engine.FinalizeWave: verify-commits failed in %s: %w", repoKey, stepErr)
				}
				if verifyData, ok := verifyStepResult.Data.(protocol.VerifyCommitsData); ok {
					result.VerifyCommits[repoKey] = &verifyData
				}
			}

			// Step 1.1: VerifyCompletionReports (I4) — manifest-level, run once
			{
				firstRepo := opts.RepoPath
				for _, rp := range repos {
					firstRepo = rp
					break
				}
				repoOpts := opts
				repoOpts.RepoPath = firstRepo
				_, stepErr := StepVerifyCompletionReports(ctx, repoOpts, manifest, onEvent)
				if stepErr != nil {
					return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
				}
			}

			// Step 1.2: CheckAgentStatuses (E7) — manifest-level, run once
			{
				firstRepo := opts.RepoPath
				for _, rp := range repos {
					firstRepo = rp
					break
				}
				repoOpts := opts
				repoOpts.RepoPath = firstRepo
				_, stepErr := StepCheckAgentStatuses(ctx, repoOpts, manifest, onEvent)
				if stepErr != nil {
					return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
				}
			}

			// Step 1.3: PredictConflicts (E11) — manifest-level, run once
			{
				firstRepo := opts.RepoPath
				for _, rp := range repos {
					firstRepo = rp
					break
				}
				repoOpts := opts
				repoOpts.RepoPath = firstRepo
				_, stepErr := StepPredictConflicts(ctx, repoOpts, manifest, onEvent)
				if stepErr != nil {
					return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
				}
			}

			// Step 2: ScanStubs (E20) — manifest-level, run once against first repo
			{
				firstRepo := opts.RepoPath
				for _, rp := range repos {
					firstRepo = rp
					break
				}
				repoOpts := opts
				repoOpts.RepoPath = firstRepo
				stubStepResult, stepErr := StepScanStubs(ctx, repoOpts, manifest, onEvent)
				if stepErr != nil {
					return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
				}
				if stubData, ok := stubStepResult.Data.(protocol.ScanStubsData); ok {
					result.StubReport = &stubData
				}
			}

			// Step 3: RunPreMergeGates (E21) — per repo, with C2 closed-loop retry
			for repoKey, repoPath := range repos {
				stateDir := protocol.SAWStateDir(repoPath)
				cache := gatecache.New(ctx, stateDir, gatecache.DefaultTTL)
				gateRes := protocol.RunPreMergeGates(manifest, opts.WaveNum, repoPath, cache, opts.Logger)
				if !gateRes.IsSuccess() {
					return result, fmt.Errorf("engine.FinalizeWave: run-pre-merge-gates failed in %s: %v", repoKey, gateRes.Errors)
				}
				gateResults := gateRes.GetData().Gates
				result.GateResults[repoKey] = gateResults

				// C2 closed-loop retry when enabled
				if opts.ClosedLoopRetryEnabled {
					for _, gate := range gateResults {
						if gate.Required && !gate.Passed {
							retryFixed := false
							var waveAgents []protocol.Agent
							for _, w := range manifest.Waves {
								if w.Number == opts.WaveNum {
									waveAgents = w.Agents
									break
								}
							}
							for _, ag := range waveAgents {
								branch := protocol.BranchName(manifest.FeatureSlug, opts.WaveNum, ag.ID)
								wtPath := protocol.ResolveWorktreePath(repoPath, branch)
								retryOpts := ClosedLoopRetryOpts{
									IMPLPath:     opts.IMPLPath,
									RepoPath:     repoPath,
									WaveNum:      opts.WaveNum,
									AgentID:      ag.ID,
									GateType:     gate.Type,
									GateCommand:  gate.Command,
									GateOutput:   gate.Stdout + "\n" + gate.Stderr,
									WorktreePath: wtPath,
									MaxRetries:   2,
								}
								retryResult, retryErr := ClosedLoopGateRetry(ctx, retryOpts)
								if retryErr != nil {
									loggerFrom(opts.Logger).Warn("engine.FinalizeWave: closed-loop retry error",
										"agent", ag.ID, "err", retryErr)
								} else if retryResult != nil && retryResult.Fixed {
									retryFixed = true
								}
								// Only retry with the first agent — the gate is repo-scoped.
								break
							}
							if retryFixed {
								// Gate was fixed by the retry agent; re-run gates to confirm.
								cache = gatecache.New(ctx, stateDir, gatecache.DefaultTTL)
								rerunRes := protocol.RunPreMergeGates(manifest, opts.WaveNum, repoPath, cache, opts.Logger)
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
							return result, fmt.Errorf("engine.FinalizeWave: required pre-merge gate %q failed in %s", gate.Type, repoKey)
						}
					}
				} else {
					// No retry: fail immediately on required gate failure
					for _, gate := range gateResults {
						if gate.Required && !gate.Passed {
							return result, fmt.Errorf("engine.FinalizeWave: required pre-merge gate %q failed in %s", gate.Type, repoKey)
						}
					}
				}
			}

			// Step 3.3: CheckTypeCollisions (E27, opt-in) — per repo
			for repoKey, repoPath := range repos {
				repoOpts := opts
				repoOpts.RepoPath = repoPath
				_, collisionReport, stepErr := StepCheckTypeCollisions(ctx, repoOpts, onEvent)
				if collisionReport != nil {
					result.CollisionReports[repoKey] = collisionReport
				}
				if stepErr != nil {
					return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
				}
			}

			// Step 3.5: ValidateIntegration (E25) — run once (first repo)
			{
				firstRepo := opts.RepoPath
				for _, rp := range repos {
					firstRepo = rp
					break
				}
				repoOpts := opts
				repoOpts.RepoPath = firstRepo
				_, integrationReport, stepErr := StepValidateIntegration(ctx, repoOpts, manifest, onEvent)
				if stepErr != nil {
					return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
				}
				result.IntegrationReport = integrationReport
			}

			// Step 3.6: CheckWiringDeclarations (E35, non-fatal) — run once
			{
				firstRepo := opts.RepoPath
				for _, rp := range repos {
					firstRepo = rp
					break
				}
				repoOpts := opts
				repoOpts.RepoPath = firstRepo
				_, wiringData, _ := StepCheckWiringDeclarations(ctx, repoOpts, manifest, onEvent)
				if wiringData != nil {
					result.WiringReport = wiringData
					if !wiringData.Valid {
						result.IntegrationActionRequired = true
						for _, gap := range wiringData.Gaps {
							result.WiringGaps = append(result.WiringGaps,
								fmt.Sprintf("%s not called in %s", gap.Declaration.Symbol, gap.Declaration.MustBeCalledFrom))
						}
					}
				}
			}

			// Step 4: MergeAgents — per repo
			// Skip for integration waves (type: integration) — agents commit directly to main.
			skipMergeAgents := false
			for _, w := range manifest.Waves {
				if w.Number == opts.WaveNum && w.Type == "integration" {
					skipMergeAgents = true
					break
				}
			}
			for repoKey, repoPath := range repos {
				if skipMergeAgents {
					result.MergeResult[repoKey] = &protocol.MergeAgentsData{
						Wave: opts.WaveNum,
					}
					continue
				}
				mergeRes, mergeErr := protocol.MergeAgents(protocol.MergeAgentsOpts{
					ManifestPath: opts.IMPLPath,
					WaveNum:      opts.WaveNum,
					RepoDir:      repoPath,
					MergeTarget:  opts.MergeTarget,
					Logger:       opts.Logger,
				})
				if mergeErr != nil {
					return result, fmt.Errorf("engine.FinalizeWave: merge-agents failed in %s: %w", repoKey, mergeErr)
				}
				if !mergeRes.IsSuccess() {
					return result, fmt.Errorf("engine.FinalizeWave: merge-agents failed in %s: %v", repoKey, mergeRes.Errors)
				}
				mergeData := mergeRes.GetData()
				result.MergeResult[repoKey] = &mergeData
			}

			// Step 4.2: PopulateIntegrationChecklist (M5, non-fatal) — run once
			{
				firstRepo := opts.RepoPath
				for _, rp := range repos {
					firstRepo = rp
					break
				}
				repoOpts := opts
				repoOpts.RepoPath = firstRepo
				_, updatedManifest, _ := StepPopulateIntegrationChecklist(ctx, repoOpts, manifest, onEvent)
				if updatedManifest != nil {
					manifest = updatedManifest
				}
			}
		}
	}

	// Step 4.5: Fix go.mod replace paths
	_, _ = StepFixGoMod(ctx, opts, onEvent)

	// Step 5: VerifyBuild — per repo
	allBuildPassed := true
	for repoKey, repoPath := range repos {
		repoOpts := opts
		repoOpts.RepoPath = repoPath
		verifyBuildStepResult, stepErr := StepVerifyBuild(ctx, repoOpts, onEvent)
		if stepErr != nil {
			allBuildPassed = false
			// Extract verify build data for the result
			if verifyBuildStepResult != nil && verifyBuildStepResult.Data != nil {
				if verifyData, ok := verifyBuildStepResult.Data.(protocol.VerifyBuildData); ok {
					result.VerifyBuild[repoKey] = &verifyData
				} else if dataMap, ok := verifyBuildStepResult.Data.(map[string]interface{}); ok {
					if vb, ok := dataMap["verify_build"]; ok {
						if verifyData, ok := vb.(protocol.VerifyBuildData); ok {
							result.VerifyBuild[repoKey] = &verifyData
						}
					}
					if d, ok := dataMap["diagnosis"]; ok {
						if diagnosis, ok := d.(*builddiag.Diagnosis); ok {
							result.BuildDiagnosis[repoKey] = diagnosis
						}
					}
				}
			}
			// Still run cleanup before returning
			cleanupStepResult, _ := StepCleanup(ctx, opts, onEvent)
			if cleanupStepResult != nil {
				if cleanupData, ok := cleanupStepResult.Data.(protocol.CleanupData); ok {
					result.CleanupResult["."] = &cleanupData
				}
			}
			return result, fmt.Errorf("engine.FinalizeWave: verify-build failed in %s: %w", repoKey, stepErr)
		}
		if verifyBuildStepResult != nil && verifyBuildStepResult.Data != nil {
			if verifyData, ok := verifyBuildStepResult.Data.(protocol.VerifyBuildData); ok {
				result.VerifyBuild[repoKey] = &verifyData
				if !verifyData.TestPassed || !verifyData.LintPassed {
					allBuildPassed = false
				}
			}
		}
	}
	result.BuildPassed = allBuildPassed

	// Step 5.5: RunPostMergeGates (E21) — per repo
	for repoKey, repoPath := range repos {
		postGateRes := protocol.RunPostMergeGates(manifest, opts.WaveNum, repoPath, opts.Logger)
		if !postGateRes.IsSuccess() {
			return result, fmt.Errorf("engine.FinalizeWave: run-post-merge-gates failed in %s: %v", repoKey, postGateRes.Errors)
		}
		postGateResults := postGateRes.GetData().Gates
		result.GateResults[repoKey] = append(result.GateResults[repoKey], postGateResults...)
		for _, gate := range postGateResults {
			if gate.Required && !gate.Passed {
				// Still run cleanup before returning
				cleanupStepResult, _ := StepCleanup(ctx, opts, onEvent)
				if cleanupStepResult != nil {
					if cleanupData, ok := cleanupStepResult.Data.(protocol.CleanupData); ok {
						result.CleanupResult["."] = &cleanupData
					}
				}
				return result, fmt.Errorf("engine.FinalizeWave: required post-merge gate %q failed in %s", gate.Type, repoKey)
			}
		}
	}

	// Step 6: Cleanup — per repo
	for repoKey, repoPath := range repos {
		repoOpts := opts
		repoOpts.RepoPath = repoPath
		cleanupStepResult, _ := StepCleanup(ctx, repoOpts, onEvent)
		if cleanupStepResult != nil {
			if cleanupData, ok := cleanupStepResult.Data.(protocol.CleanupData); ok {
				result.CleanupResult[repoKey] = &cleanupData
			}
		}
	}

	result.Success = true
	return result, nil
}

// MarkIMPLCompleteOpts configures IMPL completion marking.
type MarkIMPLCompleteOpts struct {
	IMPLPath   string                 // absolute path to IMPL manifest
	RepoPath   string                 // absolute path to the target repository
	Date       string                 // completion date in YYYY-MM-DD format
	ObsEmitter ObsEmitter   // optional: non-blocking observability emitter
	Logger     *slog.Logger // optional: nil falls back to slog.Default()
}

// MarkIMPLComplete writes the completion marker (E15), updates project context (E18),
// and archives the IMPL doc to docs/IMPL/complete/.
func MarkIMPLComplete(ctx context.Context, opts MarkIMPLCompleteOpts) result.Result[MarkCompleteData] {
	if opts.IMPLPath == "" {
		return result.NewFailure[MarkCompleteData]([]result.SAWError{
			result.NewFatal("ENGINE_MARK_COMPLETE_INVALID_OPTS", "engine.MarkIMPLComplete: IMPLPath is required"),
		})
	}

	date := opts.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// E15: Write completion marker
	if err := protocol.WriteCompletionMarker(opts.IMPLPath, date); err != nil {
		return result.NewFailure[MarkCompleteData]([]result.SAWError{
			result.NewFatal("ENGINE_MARK_COMPLETE_FAILED", "engine.MarkIMPLComplete: write marker failed").WithCause(err),
		})
	}

	// E18: Update project context
	if opts.RepoPath != "" {
		res := protocol.UpdateContext(opts.IMPLPath, opts.RepoPath)
		if res.IsFatal() {
			// Non-fatal: log but continue to archive
			loggerFrom(opts.Logger).Warn("engine.MarkIMPLComplete: update-context", "msg", res.Errors[0].Message)
		}
	}

	// Archive: move IMPL from docs/IMPL/ to docs/IMPL/complete/
	if _, err := protocol.ArchiveIMPL(ctx, opts.IMPLPath); err != nil {
		return result.NewFailure[MarkCompleteData]([]result.SAWError{
			result.NewFatal("ENGINE_MARK_COMPLETE_FAILED", "engine.MarkIMPLComplete: archive failed").WithCause(err),
		})
	}

	// Emit impl_complete after successful archival.
	// Derive the slug from the IMPL path basename (e.g. IMPL-my-feature.yaml → my-feature).
	implSlug := implSlugFromPath(opts.IMPLPath)
	if opts.ObsEmitter != nil {
		if r := opts.ObsEmitter.EmitSync(ctx, observability.NewImplCompleteEvent(implSlug)); !r.IsSuccess() {
			loggerFrom(opts.Logger).Warn("engine: impl_complete emit failed", "slug", implSlug, "err", r.Errors)
		}
	}

	return result.NewSuccess(MarkCompleteData{IMPLPath: opts.IMPLPath, Date: date})
}

// implSlugFromPath derives an IMPL slug from a file path such as
// /path/to/IMPL-my-feature.yaml → "my-feature".
func implSlugFromPath(path string) string {
	base := filepath.Base(path)
	// Strip extension
	if ext := filepath.Ext(base); ext != "" {
		base = base[:len(base)-len(ext)]
	}
	// Strip "IMPL-" prefix if present
	const prefix = "IMPL-"
	if len(base) > len(prefix) && base[:len(prefix)] == prefix {
		return base[len(prefix):]
	}
	return base
}

// inferLanguageFromTestCommand infers the programming language from a test command string.
func inferLanguageFromTestCommand(testCommand string) string {
	if testCommand == "" {
		return ""
	}
	// Reuse the same heuristics as the CLI
	switch {
	case containsAny(testCommand, "cargo test", "cargo build"):
		return "rust"
	case containsAny(testCommand, "go test", "go build"):
		return "go"
	case containsAny(testCommand, "npm test", "jest", "vitest", "mocha"):
		return "javascript"
	case containsAny(testCommand, "pytest", "python -m unittest"):
		return "python"
	default:
		return ""
	}
}

// containsAny reports whether s (case-insensitive) contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}
