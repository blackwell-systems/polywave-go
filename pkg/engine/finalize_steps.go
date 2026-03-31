package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/builddiag"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/collision"
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

	stateDir := protocol.SAWStateDir(opts.RepoPath)
	cache := gatecache.New(ctx, stateDir, 5*time.Minute)
	gateRes := protocol.RunGatesWithCache(manifest, opts.WaveNum, opts.RepoPath, cache, opts.Logger)
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

	mergeRes, err := protocol.MergeAgents(protocol.MergeAgentsOpts{
		ManifestPath: opts.IMPLPath,
		WaveNum:      opts.WaveNum,
		RepoDir:      opts.RepoPath,
		MergeTarget:  opts.MergeTarget,
		Logger:       opts.Logger,
	})
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

	if opts.ObsEmitter != nil {
		if r := opts.ObsEmitter.EmitSync(ctx, observability.NewWaveMergeEvent("", opts.WaveNum)); !r.IsSuccess() {
			loggerFrom(opts.Logger).Warn("engine: wave_merge emit failed", "wave", opts.WaveNum, "err", r.Errors)
		}
	}
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

	// In program context (MergeTarget set), wave merges land in the IMPL branch worktree,
	// not in opts.RepoPath (the main repo). Run verify-build there so the test suite sees
	// the merged changes, not the pre-merge state of main.
	buildRepoPath := opts.RepoPath
	if opts.MergeTarget != "" {
		if worktrees, err := git.WorktreeList(opts.RepoPath); err == nil {
			for _, wt := range worktrees {
				if wt[1] == opts.MergeTarget {
					buildRepoPath = wt[0]
					break
				}
			}
		}
	}

	verifyBuildRes := protocol.VerifyBuild(opts.IMPLPath, buildRepoPath)
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

	cleanupResult, err := protocol.Cleanup(opts.IMPLPath, opts.WaveNum, opts.RepoPath, opts.Logger)
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

// StepVerifyCompletionReports checks that every agent in the wave has a
// completion report (I4). Fatal: returns error listing missing agent IDs.
func StepVerifyCompletionReports(ctx context.Context, opts FinalizeWaveOpts, manifest *protocol.IMPLManifest, onEvent EventCallback) (*StepResult, error) {
	const stepName = "verify-completion-reports"
	emitStepEvent(onEvent, stepName, "running", "")

	var missing []string
	if opts.WaveNum > 0 && opts.WaveNum <= len(manifest.Waves) {
		for _, agent := range manifest.Waves[opts.WaveNum-1].Agents {
			if _, ok := manifest.CompletionReports[agent.ID]; !ok {
				missing = append(missing, agent.ID)
			}
		}
	}

	if len(missing) > 0 {
		detail := fmt.Sprintf("missing for agents: %v", missing)
		emitStepEvent(onEvent, stepName, "failed", detail)
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: detail,
		}, fmt.Errorf("verify-completion-reports: missing for agents: %v (I4)", missing)
	}

	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
	}, nil
}

// StepCheckAgentStatuses blocks merge if any agent in the wave has status
// "partial" or "blocked" (E7). Fatal on first violation.
func StepCheckAgentStatuses(ctx context.Context, opts FinalizeWaveOpts, manifest *protocol.IMPLManifest, onEvent EventCallback) (*StepResult, error) {
	const stepName = "check-agent-statuses"
	emitStepEvent(onEvent, stepName, "running", "")

	if opts.WaveNum > 0 && opts.WaveNum <= len(manifest.Waves) {
		for _, agent := range manifest.Waves[opts.WaveNum-1].Agents {
			report, ok := manifest.CompletionReports[agent.ID]
			if !ok {
				continue
			}
			if report.Status == protocol.StatusPartial || report.Status == protocol.StatusBlocked {
				detail := fmt.Sprintf("agent %s has status %q — cannot merge (E7)", agent.ID, report.Status)
				emitStepEvent(onEvent, stepName, "failed", detail)
				return &StepResult{
					Step:   stepName,
					Status: "failed",
					Detail: detail,
				}, fmt.Errorf("check-agent-statuses: agent %s has status %q — cannot merge (E7)", agent.ID, report.Status)
			}
		}
	}

	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
	}, nil
}

// StepPredictConflicts calls protocol.PredictConflictsFromReports to detect
// file-level conflicts before merge (E11). Fatal on conflict.
func StepPredictConflicts(ctx context.Context, opts FinalizeWaveOpts, manifest *protocol.IMPLManifest, onEvent EventCallback) (*StepResult, error) {
	const stepName = "predict-conflicts"
	emitStepEvent(onEvent, stepName, "running", "")

	if conflictRes := protocol.PredictConflictsFromReports(manifest, opts.WaveNum); !conflictRes.IsSuccess() {
		msg := ""
		if len(conflictRes.Errors) > 0 {
			msg = conflictRes.Errors[0].Message
		}
		emitStepEvent(onEvent, stepName, "failed", msg)
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: msg,
		}, fmt.Errorf("predict-conflicts: %s", msg)
	}

	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
	}, nil
}

// StepCheckTypeCollisions runs opt-in type collision detection using
// pkg/collision.DetectCollisions. Skips when CollisionDetectionEnabled is
// false. Fatal when CollisionReport.Valid == false.
func StepCheckTypeCollisions(ctx context.Context, opts FinalizeWaveOpts, onEvent EventCallback) (*StepResult, *collision.CollisionReport, error) {
	const stepName = "check-type-collisions"
	emitStepEvent(onEvent, stepName, "running", "")

	if !opts.CollisionDetectionEnabled {
		emitStepEvent(onEvent, stepName, "skipped", "CollisionDetectionEnabled=false")
		return &StepResult{
			Step:   stepName,
			Status: "skipped",
			Detail: "CollisionDetectionEnabled=false",
		}, nil, nil
	}

	report, err := collision.DetectCollisions(opts.IMPLPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		detail := fmt.Sprintf("collision detection error: %v", err)
		emitStepEvent(onEvent, stepName, "failed", detail)
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: detail,
		}, nil, fmt.Errorf("check-type-collisions: %w", err)
	}

	if !report.Valid {
		emitStepEvent(onEvent, stepName, "failed", "collisions detected")
		return &StepResult{
			Step:   stepName,
			Status: "failed",
			Detail: "collisions detected",
			Data:   &report,
		}, &report, fmt.Errorf("check-type-collisions: collisions detected")
	}

	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Data:   &report,
	}, &report, nil
}

// StepCheckWiringDeclarations validates wiring declarations in the manifest
// (E35). Non-fatal: gaps are surfaced but do not block merge. Returns
// (success, nil, nil) when manifest.Wiring is empty.
func StepCheckWiringDeclarations(ctx context.Context, opts FinalizeWaveOpts, manifest *protocol.IMPLManifest, onEvent EventCallback) (*StepResult, *protocol.WiringValidationData, error) {
	const stepName = "check-wiring-declarations"
	emitStepEvent(onEvent, stepName, "running", "")

	if len(manifest.Wiring) == 0 {
		emitStepEvent(onEvent, stepName, "complete", "no wiring declarations")
		return &StepResult{
			Step:   stepName,
			Status: "success",
			Detail: "no wiring declarations",
		}, nil, nil
	}

	wiringRes := protocol.ValidateWiringDeclarations(manifest, opts.RepoPath)
	if wiringRes.IsFatal() {
		// Non-fatal: log warning but do not block
		detail := fmt.Sprintf("wiring validation error (non-fatal): %v", wiringRes.Errors)
		loggerFrom(opts.Logger).Warn("engine.StepCheckWiringDeclarations", "errors", wiringRes.Errors)
		emitStepEvent(onEvent, stepName, "complete", detail)
		return &StepResult{
			Step:   stepName,
			Status: "success",
			Detail: detail,
		}, nil, nil
	}

	wiringData := wiringRes.GetData()
	emitStepEvent(onEvent, stepName, "complete", wiringData.Summary)
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Detail: wiringData.Summary,
		Data:   wiringData,
	}, wiringData, nil
}

// StepPopulateIntegrationChecklist calls protocol.PopulateIntegrationChecklist
// and saves the updated manifest (M5). Non-fatal: errors are logged but do not
// block merge.
func StepPopulateIntegrationChecklist(ctx context.Context, opts FinalizeWaveOpts, manifest *protocol.IMPLManifest, onEvent EventCallback) (*StepResult, *protocol.IMPLManifest, error) {
	const stepName = "populate-integration-checklist"
	emitStepEvent(onEvent, stepName, "running", "")

	updated, err := protocol.PopulateIntegrationChecklist(manifest)
	if err != nil {
		loggerFrom(opts.Logger).Warn("engine.StepPopulateIntegrationChecklist", "err", err)
		emitStepEvent(onEvent, stepName, "complete", fmt.Sprintf("error (non-fatal): %v", err))
		return &StepResult{
			Step:   stepName,
			Status: "success",
			Detail: fmt.Sprintf("checklist population error (non-fatal): %v", err),
		}, nil, nil
	}

	if saveRes := protocol.Save(updated, opts.IMPLPath); saveRes.IsFatal() {
		saveErrMsg := "save failed"
		if len(saveRes.Errors) > 0 {
			saveErrMsg = saveRes.Errors[0].Message
		}
		loggerFrom(opts.Logger).Warn("engine.StepPopulateIntegrationChecklist", "save_err", saveErrMsg)
		emitStepEvent(onEvent, stepName, "complete", fmt.Sprintf("save error (non-fatal): %v", saveErrMsg))
		return &StepResult{
			Step:   stepName,
			Status: "success",
			Detail: fmt.Sprintf("save error (non-fatal): %v", saveErrMsg),
		}, nil, nil
	}

	emitStepEvent(onEvent, stepName, "complete", "")
	return &StepResult{
		Step:   stepName,
		Status: "success",
		Data:   updated,
	}, updated, nil
}
