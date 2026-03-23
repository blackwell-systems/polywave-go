package engine

import (
	"context"
	"fmt"
	"os"
	"time"

	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/builddiag"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// FinalizeWaveOpts configures a full post-agent finalization pipeline.
type FinalizeWaveOpts struct {
	IMPLPath    string                 // absolute path to IMPL manifest
	RepoPath    string                 // absolute path to the target repository
	WaveNum     int                    // wave number to finalize
	MergeTarget string                 // target branch for merge; empty = HEAD (default)
	ObsEmitter  *observability.Emitter // optional: non-blocking observability emitter

	// M2 (E25): When true, unconnected exports in the integration report
	// block the merge with a fatal error. Default (false) preserves the
	// existing informational-only behavior.
	EnforceIntegrationValidation bool

	// M3 (E20): When true, TODO/FIXME stubs detected in changed files
	// block the merge with a fatal error. Default (false) preserves the
	// existing informational-only behavior.
	RequireNoStubs bool

	// SkipMerge: When true, skip steps 1-4 (VerifyCommits, ScanStubs, RunGates, MergeAgents)
	// and start from verify-build. Used for manual merge escape hatch when E11
	// blocks merge with false positive.
	SkipMerge bool
}

// FinalizeWaveResult combines all post-agent verification, merge, and cleanup results.
type FinalizeWaveResult struct {
	Wave           int                        `json:"wave"`
	VerifyCommits  *protocol.VerifyCommitsResult `json:"verify_commits,omitempty"`
	StubReport     *protocol.ScanStubsResult  `json:"stub_report,omitempty"`
	GateResults       []protocol.GateResult         `json:"gate_results,omitempty"`
	IntegrationReport *protocol.IntegrationReport   `json:"integration_report,omitempty"`
	MergeResult       *protocol.MergeAgentsResult   `json:"merge_result,omitempty"`
	VerifyBuild    *protocol.VerifyBuildResult `json:"verify_build,omitempty"`
	BuildDiagnosis *builddiag.Diagnosis        `json:"build_diagnosis,omitempty"`
	CleanupResult  *protocol.CleanupResult    `json:"cleanup_result,omitempty"`
	BuildPassed    bool                       `json:"build_passed"`
	Success        bool                       `json:"success"`
}

// FinalizeWave runs the full post-agent finalization pipeline for a single wave.
// This is the engine-level equivalent of the CLI's `sawtools finalize-wave` command.
//
// Pipeline:
//  1. VerifyCommits (I5) - ensure all agents committed work
//  2. ScanStubs (E20) - scan changed files for TODO/FIXME markers
//  3. RunGates (E21) - execute quality gates
//  3.5. ValidateIntegration (E25) - scan for unconnected exports (informational)
//  4. MergeAgents - merge agent branches into main
//  5. VerifyBuild - run test_command and lint_command
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

	result := &FinalizeWaveResult{
		Wave: opts.WaveNum,
	}

	// If SkipMerge is true, skip steps 1-4 and jump directly to verify-build.
	// Populate MergeResult with synthetic entry indicating merge already done.
	if !opts.SkipMerge {
		// Step 1: VerifyCommits (I5) — C4: VerifyCommits runs BEFORE MergeAgents and is fatal.
		// Agents with no commits are a protocol violation and must block the merge entirely.
		// MergeResult will be nil in FinalizeWaveResult whenever this step fails.
		verifyRes := protocol.VerifyCommits(opts.IMPLPath, opts.WaveNum, opts.RepoPath)
		if !verifyRes.IsSuccess() {
			return result, fmt.Errorf("engine.FinalizeWave: verify-commits: %v", verifyRes.Errors)
		}
		verifyResult := verifyRes.GetData()
		result.VerifyCommits = &verifyResult
		// Check if all agents have commits
		allValid := true
		for _, agent := range verifyResult.Agents {
			if !agent.HasCommits {
				allValid = false
				break
			}
		}
		if !allValid {
			return result, fmt.Errorf("engine.FinalizeWave: verify-commits found agents with no commits")
		}

		// Step 2: ScanStubs (E20) - informational, does not block
		var changedFiles []string
		if opts.WaveNum > 0 && opts.WaveNum <= len(manifest.Waves) {
			for _, agent := range manifest.Waves[opts.WaveNum-1].Agents {
				if report, ok := manifest.CompletionReports[agent.ID]; ok {
					changedFiles = append(changedFiles, report.FilesChanged...)
					changedFiles = append(changedFiles, report.FilesCreated...)
				}
			}
		}
		if len(changedFiles) > 0 {
			stubRes := protocol.ScanStubs(changedFiles)
			if !stubRes.IsSuccess() {
				// Non-fatal: log but continue
				fmt.Fprintf(os.Stderr, "engine.FinalizeWave: scan-stubs: %v\n", stubRes.Errors)
			} else {
				stubData := stubRes.GetData()
				result.StubReport = &stubData
				// M3 (E20): When RequireNoStubs is true, any stubs block the pipeline.
				if opts.RequireNoStubs && len(stubData.Hits) > 0 {
					return result, fmt.Errorf("engine.FinalizeWave: %d stub(s) detected in changed files (RequireNoStubs=true)", len(stubData.Hits))
				}
			}
		}

		// Step 3: RunGates (E21) — with caching support
		stateDir := filepath.Join(opts.RepoPath, ".saw-state")
		cache := gatecache.New(stateDir, 5*time.Minute)
		gateRes := protocol.RunGatesWithCache(manifest, opts.WaveNum, opts.RepoPath, cache)
		if !gateRes.IsSuccess() {
			return result, fmt.Errorf("engine.FinalizeWave: run-gates: %v", gateRes.Errors)
		}
		gateResults := gateRes.GetData().Gates
		result.GateResults = gateResults
		for _, gate := range gateResults {
			opts.ObsEmitter.Emit(ctx, observability.NewGateExecutedEvent(manifest.FeatureSlug, opts.WaveNum, gate.Type, gate.Passed))
			if gate.Required && !gate.Passed {
				opts.ObsEmitter.Emit(ctx, observability.NewWaveFailedEvent(manifest.FeatureSlug, opts.WaveNum, fmt.Sprintf("required gate %q failed", gate.Type)))
				return result, fmt.Errorf("engine.FinalizeWave: required gate %q failed", gate.Type)
			}
		}

		// Step 3.5: ValidateIntegration (E25) - informational by default, fatal when enforced
		integrationReport, err := protocol.ValidateIntegration(manifest, opts.WaveNum, opts.RepoPath)
		if err != nil {
			// Non-fatal: integration validation errors don't block the pipeline
			fmt.Fprintf(os.Stderr, "engine.FinalizeWave: validate-integration: %v\n", err)
		} else {
			result.IntegrationReport = integrationReport
			// Persist to manifest
			waveKey := fmt.Sprintf("wave%d", opts.WaveNum)
			_ = protocol.AppendIntegrationReport(opts.IMPLPath, waveKey, integrationReport)
			// M2 (E25): When EnforceIntegrationValidation is true, unconnected exports block the pipeline.
			if opts.EnforceIntegrationValidation && len(integrationReport.Gaps) > 0 {
				return result, fmt.Errorf("engine.FinalizeWave: %d unconnected export(s) detected (EnforceIntegrationValidation=true)", len(integrationReport.Gaps))
			}
		}

		// Step 4: MergeAgents
		mergeRes, err := protocol.MergeAgents(opts.IMPLPath, opts.WaveNum, opts.RepoPath, opts.MergeTarget)
		if err != nil {
			return result, fmt.Errorf("engine.FinalizeWave: merge-agents: %w", err)
		}
		if !mergeRes.IsSuccess() {
			return result, fmt.Errorf("engine.FinalizeWave: merge-agents: %v", mergeRes.Errors)
		}
		mergeResult := mergeRes.GetData()
		result.MergeResult = &mergeResult
		if !mergeResult.Success {
			opts.ObsEmitter.Emit(ctx, observability.NewWaveFailedEvent(manifest.FeatureSlug, opts.WaveNum, "merge-agents encountered conflicts"))
			return result, fmt.Errorf("engine.FinalizeWave: merge-agents encountered conflicts")
		}
		opts.ObsEmitter.Emit(ctx, observability.NewWaveMergeEvent(manifest.FeatureSlug, opts.WaveNum))
	} else {
		// SkipMerge mode: populate synthetic MergeResult
		result.MergeResult = &protocol.MergeAgentsResult{
			Wave:    opts.WaveNum,
			Success: true,
		}
		// Warning if worktrees still exist
		if !protocol.WorktreesAbsent(manifest, opts.WaveNum, opts.RepoPath) {
			fmt.Fprintf(os.Stderr, "engine.FinalizeWave: --skip-merge used but worktrees detected; cleanup will remove them\n")
		}
	}

	// Step 4.5: Fix go.mod replace paths (worktree artifact defense-in-depth)
	if fixed, err := protocol.FixGoModReplacePaths(opts.RepoPath); err != nil {
		fmt.Fprintf(os.Stderr, "engine.FinalizeWave: go.mod fixup: %v\n", err)
	} else if fixed {
		fmt.Fprintf(os.Stderr, "engine.FinalizeWave: auto-corrected go.mod replace paths\n")
	}

	// Step 5: VerifyBuild
	verifyBuildRes := protocol.VerifyBuild(opts.IMPLPath, opts.RepoPath)
	if !verifyBuildRes.IsSuccess() {
		return result, fmt.Errorf("engine.FinalizeWave: verify-build: %v", verifyBuildRes.Errors)
	}
	verifyBuildResult := verifyBuildRes.GetData()
	result.VerifyBuild = &verifyBuildResult
	result.BuildPassed = verifyBuildResult.TestPassed && verifyBuildResult.LintPassed

	// Step 6: Cleanup — runs after merge regardless of build result.
	// Once branches are merged, worktrees serve no purpose. Cleaning up here
	// prevents stale worktrees from accumulating when builds fail.
	cleanupResult, err := protocol.Cleanup(opts.IMPLPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		// Non-fatal: cleanup failure shouldn't fail the wave
		fmt.Fprintf(os.Stderr, "engine.FinalizeWave: cleanup: %v\n", err)
	} else {
		result.CleanupResult = cleanupResult
	}

	if !result.BuildPassed {
		// Auto-diagnose build failure using H7 pattern matching
		language := inferLanguageFromTestCommand(manifest.TestCommand)
		if language != "" {
			errorLog := verifyBuildResult.TestOutput + "\n" + verifyBuildResult.LintOutput
			diagnosis, diagErr := builddiag.DiagnoseError(errorLog, language)
			if diagErr == nil {
				result.BuildDiagnosis = diagnosis
			}
		}
		return result, fmt.Errorf("engine.FinalizeWave: verify-build failed (test_passed=%v, lint_passed=%v)",
			verifyBuildResult.TestPassed, verifyBuildResult.LintPassed)
	}

	result.Success = true
	return result, nil
}

// MarkIMPLCompleteOpts configures IMPL completion marking.
type MarkIMPLCompleteOpts struct {
	IMPLPath   string                 // absolute path to IMPL manifest
	RepoPath   string                 // absolute path to the target repository
	Date       string                 // completion date in YYYY-MM-DD format
	ObsEmitter *observability.Emitter // optional: non-blocking observability emitter
}

// MarkIMPLComplete writes the completion marker (E15), updates project context (E18),
// and archives the IMPL doc to docs/IMPL/complete/.
func MarkIMPLComplete(ctx context.Context, opts MarkIMPLCompleteOpts) error {
	if opts.IMPLPath == "" {
		return fmt.Errorf("engine.MarkIMPLComplete: IMPLPath is required")
	}

	date := opts.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// E15: Write completion marker
	if err := protocol.WriteCompletionMarker(opts.IMPLPath, date); err != nil {
		return fmt.Errorf("engine.MarkIMPLComplete: write marker: %w", err)
	}

	// E18: Update project context
	if opts.RepoPath != "" {
		if _, err := protocol.UpdateContext(opts.IMPLPath, opts.RepoPath); err != nil {
			// Non-fatal: log but continue to archive
			fmt.Fprintf(os.Stderr, "engine.MarkIMPLComplete: update-context: %v\n", err)
		}
	}

	// Archive: move IMPL from docs/IMPL/ to docs/IMPL/complete/
	if _, err := protocol.ArchiveIMPL(opts.IMPLPath); err != nil {
		return fmt.Errorf("engine.MarkIMPLComplete: archive: %w", err)
	}

	// Emit impl_complete after successful archival.
	// Derive the slug from the IMPL path basename (e.g. IMPL-my-feature.yaml → my-feature).
	implSlug := implSlugFromPath(opts.IMPLPath)
	opts.ObsEmitter.Emit(ctx, observability.NewImplCompleteEvent(implSlug))

	return nil
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
	case contains(testCommand, "cargo test", "cargo build"):
		return "rust"
	case contains(testCommand, "go test", "go build"):
		return "go"
	case contains(testCommand, "npm test", "jest", "vitest", "mocha"):
		return "javascript"
	case contains(testCommand, "pytest", "python -m unittest"):
		return "python"
	default:
		return ""
	}
}

// contains checks if s contains any of the substrings.
func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			// Case-insensitive check
			for i := 0; i <= len(s)-len(sub); i++ {
				match := true
				for j := 0; j < len(sub); j++ {
					sc := s[i+j]
					tc := sub[j]
					if sc >= 'A' && sc <= 'Z' {
						sc += 32
					}
					if tc >= 'A' && tc <= 'Z' {
						tc += 32
					}
					if sc != tc {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}
