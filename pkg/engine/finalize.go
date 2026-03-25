package engine

import (
	"context"
	"fmt"
	"os"
	"time"

	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/builddiag"
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

	// OnEvent is an optional callback invoked at the start and end of each
	// finalization step. CLI callers can print progress; web callers can emit
	// SSE events. nil is safe — step functions check before calling.
	OnEvent EventCallback
}

// FinalizeWaveResult combines all post-agent verification, merge, and cleanup results.
type FinalizeWaveResult struct {
	Wave           int                          `json:"wave"`
	VerifyCommits  *protocol.VerifyCommitsData  `json:"verify_commits,omitempty"`
	StubReport     *protocol.ScanStubsData      `json:"stub_report,omitempty"`
	GateResults       []protocol.GateResult         `json:"gate_results,omitempty"`
	IntegrationReport *protocol.IntegrationReport   `json:"integration_report,omitempty"`
	MergeResult       *protocol.MergeAgentsData     `json:"merge_result,omitempty"`
	VerifyBuild    *protocol.VerifyBuildData    `json:"verify_build,omitempty"`
	BuildDiagnosis *builddiag.Diagnosis         `json:"build_diagnosis,omitempty"`
	CleanupResult  *protocol.CleanupData        `json:"cleanup_result,omitempty"`
	BuildPassed    bool                         `json:"build_passed"`
	Success        bool                         `json:"success"`
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

	onEvent := opts.OnEvent

	// If SkipMerge is true, skip steps 1-4 and jump directly to verify-build.
	// Populate MergeResult with synthetic entry indicating merge already done.
	if !opts.SkipMerge {
		// Step 1: VerifyCommits (I5)
		verifyStepResult, stepErr := StepVerifyCommits(ctx, opts, onEvent)
		if stepErr != nil {
			return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
		}
		if verifyData, ok := verifyStepResult.Data.(protocol.VerifyCommitsData); ok {
			result.VerifyCommits = &verifyData
		}

		// Step 2: ScanStubs (E20)
		stubStepResult, stepErr := StepScanStubs(ctx, opts, manifest, onEvent)
		if stepErr != nil {
			return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
		}
		if stubData, ok := stubStepResult.Data.(protocol.ScanStubsData); ok {
			result.StubReport = &stubData
		}

		// Step 3: RunGates (E21)
		_, gatesData, stepErr := StepRunGates(ctx, opts, manifest, onEvent)
		if stepErr != nil {
			return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
		}
		if gatesData != nil {
			result.GateResults = gatesData.Gates
		}

		// Step 3.5: ValidateIntegration (E25)
		_, integrationReport, stepErr := StepValidateIntegration(ctx, opts, manifest, onEvent)
		if stepErr != nil {
			return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
		}
		result.IntegrationReport = integrationReport

		// Step 4: MergeAgents
		mergeStepResult, stepErr := StepMergeAgents(ctx, opts, onEvent)
		if stepErr != nil {
			return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
		}
		if mergeData, ok := mergeStepResult.Data.(protocol.MergeAgentsData); ok {
			result.MergeResult = &mergeData
		}
	} else {
		// SkipMerge mode: populate synthetic MergeResult
		result.MergeResult = &protocol.MergeAgentsData{
			Wave:    opts.WaveNum,
			Success: true,
		}
		// Warning if worktrees still exist
		if !protocol.WorktreesAbsent(manifest, opts.WaveNum, opts.RepoPath) {
			fmt.Fprintf(os.Stderr, "engine.FinalizeWave: --skip-merge used but worktrees detected; cleanup will remove them\n")
		}
	}

	// Step 4.5: Fix go.mod replace paths
	_, _ = StepFixGoMod(ctx, opts, onEvent)

	// Step 5: VerifyBuild
	verifyBuildStepResult, stepErr := StepVerifyBuild(ctx, opts, onEvent)
	if stepErr != nil {
		// Extract verify build data for the result
		if verifyData, ok := verifyBuildStepResult.Data.(protocol.VerifyBuildData); ok {
			result.VerifyBuild = &verifyData
			result.BuildPassed = false
		} else if dataMap, ok := verifyBuildStepResult.Data.(map[string]interface{}); ok {
			// Data contains both verify_build and diagnosis
			if vb, ok := dataMap["verify_build"]; ok {
				if verifyData, ok := vb.(protocol.VerifyBuildData); ok {
					result.VerifyBuild = &verifyData
				}
			}
			if d, ok := dataMap["diagnosis"]; ok {
				if diagnosis, ok := d.(*builddiag.Diagnosis); ok {
					result.BuildDiagnosis = diagnosis
				}
			}
			result.BuildPassed = false
		}

		// Still run cleanup before returning
		cleanupStepResult, _ := StepCleanup(ctx, opts, onEvent)
		if cleanupData, ok := cleanupStepResult.Data.(protocol.CleanupData); ok {
			result.CleanupResult = &cleanupData
		}
		return result, fmt.Errorf("engine.FinalizeWave: %w", stepErr)
	}
	if verifyData, ok := verifyBuildStepResult.Data.(protocol.VerifyBuildData); ok {
		result.VerifyBuild = &verifyData
		result.BuildPassed = verifyData.TestPassed && verifyData.LintPassed
	}

	// Step 6: Cleanup
	cleanupStepResult, _ := StepCleanup(ctx, opts, onEvent)
	if cleanupStepResult != nil {
		if cleanupData, ok := cleanupStepResult.Data.(protocol.CleanupData); ok {
			result.CleanupResult = &cleanupData
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
		res := protocol.UpdateContext(opts.IMPLPath, opts.RepoPath)
		if res.IsFatal() {
			// Non-fatal: log but continue to archive
			fmt.Fprintf(os.Stderr, "engine.MarkIMPLComplete: update-context: %s\n", res.Errors[0].Message)
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
