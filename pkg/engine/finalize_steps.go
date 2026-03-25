package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/builddiag"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// emitStepEvent is a nil-safe helper for calling an EventCallback.
func emitStepEvent(onEvent EventCallback, step, status, detail string) {
	if onEvent != nil {
		onEvent(step, status, detail)
	}
}

// StepVerifyCommits verifies that all agents in the wave have committed work (I5).
// This is a fatal gate: agents with no commits block the merge entirely.
func StepVerifyCommits(ctx context.Context, opts FinalizeWaveOpts, onEvent EventCallback) (*StepResult, error) {
	const stepName = "verify-commits"
	emitStepEvent(onEvent, stepName, "running", "")

	verifyRes := protocol.VerifyCommits(opts.IMPLPath, opts.WaveNum, opts.RepoPath)
	if !verifyRes.IsSuccess() {
		emitStepEvent(onEvent, stepName, "failed", fmt.Sprintf("%v", verifyRes.Errors))
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: fmt.Sprintf("%v", verifyRes.Errors),
		}, fmt.Errorf("verify-commits: %v", verifyRes.Errors)
	}

	verifyData := verifyRes.GetData()
	allValid := true
	for _, agent := range verifyData.Agents {
		if !agent.HasCommits {
			allValid = false
			break
		}
	}

	if !allValid {
		emitStepEvent(onEvent, stepName, "failed", "agents with no commits")
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: "agents with no commits",
			Data:   verifyData,
		}, fmt.Errorf("verify-commits found agents with no commits")
	}

	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Data:   verifyData,
	}, nil
}

// StepScanStubs scans changed files for TODO/FIXME markers (E20).
// Informational by default; fatal when opts.RequireNoStubs is true.
func StepScanStubs(ctx context.Context, opts FinalizeWaveOpts, manifest *protocol.IMPLManifest, onEvent EventCallback) (*StepResult, error) {
	const stepName = "scan-stubs"
	emitStepEvent(onEvent, stepName, "running", "")

	var changedFiles []string
	if opts.WaveNum > 0 && opts.WaveNum <= len(manifest.Waves) {
		for _, agent := range manifest.Waves[opts.WaveNum-1].Agents {
			if report, ok := manifest.CompletionReports[agent.ID]; ok {
				changedFiles = append(changedFiles, report.FilesChanged...)
				changedFiles = append(changedFiles, report.FilesCreated...)
			}
		}
	}

	if len(changedFiles) == 0 {
		emitStepEvent(onEvent, stepName, "complete", "no changed files")
		return &StepResult{
			Step:   stepName,
			Status: "skipped",
			Detail: "no changed files to scan",
		}, nil
	}

	stubRes := protocol.ScanStubs(changedFiles)
	if !stubRes.IsSuccess() {
		// Non-fatal scan errors
		loggerFrom(opts.Logger).Warn("engine.StepScanStubs", "errors", stubRes.Errors)
		emitStepEvent(onEvent, stepName, "complete", fmt.Sprintf("scan error: %v", stubRes.Errors))
		return &StepResult{
			Step:   stepName,
			Status: "success",
			Detail: fmt.Sprintf("scan error (non-fatal): %v", stubRes.Errors),
		}, nil
	}

	stubData := stubRes.GetData()

	// M3 (E20): When RequireNoStubs is true, any stubs block the pipeline.
	if opts.RequireNoStubs && len(stubData.Hits) > 0 {
		emitStepEvent(onEvent, stepName, "failed", fmt.Sprintf("%d stub(s) detected", len(stubData.Hits)))
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: fmt.Sprintf("%d stub(s) detected (RequireNoStubs=true)", len(stubData.Hits)),
			Data:   stubData,
		}, fmt.Errorf("%d stub(s) detected in changed files (RequireNoStubs=true)", len(stubData.Hits))
	}

	emitStepEvent(onEvent, stepName, "complete", fmt.Sprintf("%d stub(s) found (informational)", len(stubData.Hits)))
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Detail: fmt.Sprintf("%d stub(s) found", len(stubData.Hits)),
		Data:   stubData,
	}, nil
}

// StepRunGates executes quality gates for the wave (E21) with caching support.
// Returns the gate results data for the caller to inspect.
func StepRunGates(ctx context.Context, opts FinalizeWaveOpts, manifest *protocol.IMPLManifest, onEvent EventCallback) (*StepResult, *protocol.GatesData, error) {
	const stepName = "run-gates"
	emitStepEvent(onEvent, stepName, "running", "")

	stateDir := filepath.Join(opts.RepoPath, ".saw-state")
	cache := gatecache.New(stateDir, 5*time.Minute)
	gateRes := protocol.RunGatesWithCache(manifest, opts.WaveNum, opts.RepoPath, cache)
	if !gateRes.IsSuccess() {
		emitStepEvent(onEvent, stepName, "failed", fmt.Sprintf("%v", gateRes.Errors))
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: fmt.Sprintf("%v", gateRes.Errors),
		}, nil, fmt.Errorf("run-gates: %v", gateRes.Errors)
	}

	gatesData := gateRes.GetData()
	for _, gate := range gatesData.Gates {
		opts.ObsEmitter.Emit(ctx, observability.NewGateExecutedEvent(manifest.FeatureSlug, opts.WaveNum, gate.Type, gate.Passed))
		if gate.Required && !gate.Passed {
			opts.ObsEmitter.Emit(ctx, observability.NewWaveFailedEvent(manifest.FeatureSlug, opts.WaveNum, fmt.Sprintf("required gate %q failed", gate.Type)))
			emitStepEvent(onEvent, stepName, "failed", fmt.Sprintf("required gate %q failed", gate.Type))
			return &StepResult{
				Step:   stepName,
				Status: "failed",
				Detail: fmt.Sprintf("required gate %q failed", gate.Type),
				Data:   gatesData,
			}, &gatesData, fmt.Errorf("required gate %q failed", gate.Type)
		}
	}

	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Data:   gatesData,
	}, &gatesData, nil
}

// StepValidateIntegration scans for unconnected exports (E25).
// Informational by default; fatal when opts.EnforceIntegrationValidation is true.
func StepValidateIntegration(ctx context.Context, opts FinalizeWaveOpts, manifest *protocol.IMPLManifest, onEvent EventCallback) (*StepResult, *protocol.IntegrationReport, error) {
	const stepName = "validate-integration"
	emitStepEvent(onEvent, stepName, "running", "")

	integrationReport, err := protocol.ValidateIntegration(manifest, opts.WaveNum, opts.RepoPath)
	if err != nil {
		// Non-fatal: integration validation errors don't block the pipeline
		loggerFrom(opts.Logger).Warn("engine.StepValidateIntegration", "err", err)
		emitStepEvent(onEvent, stepName, "complete", fmt.Sprintf("error (non-fatal): %v", err))
		return &StepResult{
			Step:   stepName,
			Status: "success",
			Detail: fmt.Sprintf("validation error (non-fatal): %v", err),
		}, nil, nil
	}

	// Persist to manifest
	waveKey := fmt.Sprintf("wave%d", opts.WaveNum)
	_ = protocol.AppendIntegrationReport(opts.IMPLPath, waveKey, integrationReport)

	// M2 (E25): When EnforceIntegrationValidation is true, unconnected exports block.
	if opts.EnforceIntegrationValidation && len(integrationReport.Gaps) > 0 {
		emitStepEvent(onEvent, stepName, "failed", fmt.Sprintf("%d unconnected export(s)", len(integrationReport.Gaps)))
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: fmt.Sprintf("%d unconnected export(s) (EnforceIntegrationValidation=true)", len(integrationReport.Gaps)),
			Data:   integrationReport,
		}, integrationReport, fmt.Errorf("%d unconnected export(s) detected (EnforceIntegrationValidation=true)", len(integrationReport.Gaps))
	}

	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Data:   integrationReport,
	}, integrationReport, nil
}

// StepMergeAgents merges agent branches into the target branch.
func StepMergeAgents(ctx context.Context, opts FinalizeWaveOpts, onEvent EventCallback) (*StepResult, error) {
	const stepName = "merge-agents"
	emitStepEvent(onEvent, stepName, "running", "")

	mergeRes, err := protocol.MergeAgents(opts.IMPLPath, opts.WaveNum, opts.RepoPath, opts.MergeTarget)
	if err != nil {
		emitStepEvent(onEvent, stepName, "failed", err.Error())
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: err.Error(),
		}, fmt.Errorf("merge-agents: %w", err)
	}
	if !mergeRes.IsSuccess() {
		emitStepEvent(onEvent, stepName, "failed", fmt.Sprintf("%v", mergeRes.Errors))
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: fmt.Sprintf("%v", mergeRes.Errors),
		}, fmt.Errorf("merge-agents: %v", mergeRes.Errors)
	}

	mergeData := mergeRes.GetData()
	if !mergeData.Success {
		opts.ObsEmitter.Emit(ctx, observability.NewWaveFailedEvent("", opts.WaveNum, "merge-agents encountered conflicts"))
		emitStepEvent(onEvent, stepName, "failed", "merge encountered conflicts")
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: "merge encountered conflicts",
			Data:   mergeData,
		}, fmt.Errorf("merge-agents encountered conflicts")
	}

	opts.ObsEmitter.Emit(ctx, observability.NewWaveMergeEvent("", opts.WaveNum))
	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Data:   mergeData,
	}, nil
}

// StepFixGoMod fixes go.mod replace paths that may have been corrupted by worktree artifacts.
func StepFixGoMod(ctx context.Context, opts FinalizeWaveOpts, onEvent EventCallback) (*StepResult, error) {
	const stepName = "fix-gomod"
	emitStepEvent(onEvent, stepName, "running", "")

	fixed, err := protocol.FixGoModReplacePaths(opts.RepoPath)
	if err != nil {
		loggerFrom(opts.Logger).Warn("engine.StepFixGoMod", "err", err)
		emitStepEvent(onEvent, stepName, "complete", fmt.Sprintf("error (non-fatal): %v", err))
		return &StepResult{
			Step:   stepName,
			Status: "success",
			Detail: fmt.Sprintf("fixup error (non-fatal): %v", err),
		}, nil
	}

	detail := "no changes needed"
	if fixed {
		detail = "auto-corrected go.mod replace paths"
		loggerFrom(opts.Logger).Warn("engine.StepFixGoMod", "detail", detail)
	}

	emitStepEvent(onEvent, stepName, "complete", detail)
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Detail: detail,
	}, nil
}

// StepVerifyBuild runs the test_command and lint_command to verify the build after merge.
func StepVerifyBuild(ctx context.Context, opts FinalizeWaveOpts, onEvent EventCallback) (*StepResult, error) {
	const stepName = "verify-build"
	emitStepEvent(onEvent, stepName, "running", "")

	verifyBuildRes := protocol.VerifyBuild(opts.IMPLPath, opts.RepoPath)
	if !verifyBuildRes.IsSuccess() {
		emitStepEvent(onEvent, stepName, "failed", fmt.Sprintf("%v", verifyBuildRes.Errors))
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: fmt.Sprintf("%v", verifyBuildRes.Errors),
		}, fmt.Errorf("verify-build: %v", verifyBuildRes.Errors)
	}

	verifyData := verifyBuildRes.GetData()
	passed := verifyData.TestPassed && verifyData.LintPassed

	if !passed {
		// Attempt auto-diagnosis using H7 pattern matching
		manifest, loadErr := protocol.Load(opts.IMPLPath)
		var diagnosis *builddiag.Diagnosis
		if loadErr == nil {
			language := inferLanguageFromTestCommand(manifest.TestCommand)
			if language != "" {
				errorLog := verifyData.TestOutput + "\n" + verifyData.LintOutput
				if d, diagErr := builddiag.DiagnoseError(errorLog, language); diagErr == nil {
					diagnosis = d
				}
			}
		}

		detail := fmt.Sprintf("test_passed=%v, lint_passed=%v", verifyData.TestPassed, verifyData.LintPassed)
		emitStepEvent(onEvent, stepName, "failed", detail)
		result := &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: detail,
			Data:   verifyData,
		}
		// Attach diagnosis to Data if available
		if diagnosis != nil {
			result.Data = map[string]interface{}{
				"verify_build": verifyData,
				"diagnosis":    diagnosis,
			}
		}
		return result, fmt.Errorf("verify-build failed (test_passed=%v, lint_passed=%v)", verifyData.TestPassed, verifyData.LintPassed)
	}

	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Data:   verifyData,
	}, nil
}

// StepCleanup removes worktrees and agent branches after merge.
// Non-fatal: cleanup failure does not fail the wave.
func StepCleanup(ctx context.Context, opts FinalizeWaveOpts, onEvent EventCallback) (*StepResult, error) {
	const stepName = "cleanup"
	emitStepEvent(onEvent, stepName, "running", "")

	cleanupResult, err := protocol.Cleanup(opts.IMPLPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		// Non-fatal
		loggerFrom(opts.Logger).Warn("engine.StepCleanup", "err", err)
		emitStepEvent(onEvent, stepName, "complete", fmt.Sprintf("error (non-fatal): %v", err))
		return &StepResult{
			Step:   stepName,
			Status: "success",
			Detail: fmt.Sprintf("cleanup error (non-fatal): %v", err),
		}, nil
	}

	cleanupData := cleanupResult.GetData()
	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Data:   cleanupData,
	}, nil
}
