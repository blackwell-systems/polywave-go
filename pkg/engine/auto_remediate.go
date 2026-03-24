package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// autoRemediateFixBuildFailureFunc is the function variable used to call FixBuildFailure.
// Override in tests to avoid real AI calls.
var autoRemediateFixBuildFailureFunc = func(ctx context.Context, opts FixBuildOpts) error {
	return FixBuildFailure(ctx, opts)
}

// autoRemediateVerifyBuildFunc is the function variable used to call protocol.VerifyBuild.
// Override in tests to avoid real build execution.
var autoRemediateVerifyBuildFunc = func(implPath, repoPath string) (*protocol.VerifyBuildData, error) {
	res := protocol.VerifyBuild(implPath, repoPath)
	if !res.IsSuccess() {
		return nil, fmt.Errorf("verify build failed: %v", res.Errors)
	}
	data := res.GetData()
	return &data, nil
}

// AutoRemediateOpts configures an automated remediation attempt for a failed wave.
type AutoRemediateOpts struct {
	IMPLPath     string              // absolute path to IMPL manifest
	RepoPath     string              // absolute path to the target repository
	WaveNum      int                 // wave number that failed
	FailedResult *FinalizeWaveResult // the failed finalize result
	MaxRetries   int                 // maximum number of fix attempts
	ChatModel    string              // model for fix agent
	OnEvent      func(Event)         // progress event callback (may be nil)
}

// AutoRemediateResult reports the outcome of the auto-remediation loop.
type AutoRemediateResult struct {
	Fixed       bool                `json:"fixed"`
	Attempts    int                 `json:"attempts"`
	FinalResult *FinalizeWaveResult `json:"final_result"`
}

// AutoRemediate attempts to automatically fix a failed FinalizeWave result.
// It loops up to MaxRetries times, calling FixBuildFailure and then re-running
// VerifyBuild after each fix attempt. Returns Fixed=true on the first successful
// verification, or Fixed=false once all retries are exhausted.
func AutoRemediate(ctx context.Context, opts AutoRemediateOpts) (*AutoRemediateResult, error) {
	result := &AutoRemediateResult{
		FinalResult: opts.FailedResult,
	}

	emit := func(eventName string, data interface{}) {
		if opts.OnEvent != nil {
			opts.OnEvent(Event{Event: eventName, Data: data})
		}
	}

	// Emit started event
	emit("auto_remediate_started", map[string]interface{}{
		"wave":        opts.WaveNum,
		"max_retries": opts.MaxRetries,
	})

	// If MaxRetries is zero, return immediately without any attempts
	if opts.MaxRetries <= 0 {
		emit("auto_remediate_exhausted", map[string]interface{}{
			"wave":     opts.WaveNum,
			"attempts": 0,
			"reason":   "max_retries is zero",
		})
		return result, nil
	}

	currentFailed := opts.FailedResult

	for attempt := 1; attempt <= opts.MaxRetries; attempt++ {
		// Emit attempt event
		emit("auto_remediate_attempt", map[string]interface{}{
			"wave":    opts.WaveNum,
			"attempt": attempt,
			"max":     opts.MaxRetries,
		})

		// Step a: Extract error log from the current failed verify build result
		errorLog := extractErrorLog(currentFailed)

		// Step b: Determine gate type from which gate failed
		gateType := determineFailedGateType(currentFailed)

		// Step c: Call FixBuildFailure with error context
		fixOpts := FixBuildOpts{
			IMPLPath:  opts.IMPLPath,
			RepoPath:  opts.RepoPath,
			WaveNum:   opts.WaveNum,
			ErrorLog:  errorLog,
			GateType:  gateType,
			ChatModel: opts.ChatModel,
		}

		if err := autoRemediateFixBuildFailureFunc(ctx, fixOpts); err != nil {
			// Fix agent itself failed — treat as failed attempt, continue loop
			_ = err
		}

		// Step d: Re-run VerifyBuild after fix attempt
		verifyResult, err := autoRemediateVerifyBuildFunc(opts.IMPLPath, opts.RepoPath)
		if err != nil {
			// System-level error — return immediately
			result.Attempts = attempt
			return result, fmt.Errorf("engine.AutoRemediate: verify-build system error on attempt %d: %w", attempt, err)
		}

		// Update current failed result with new verify build output
		updatedResult := &FinalizeWaveResult{
			Wave:         opts.WaveNum,
			VerifyBuild:  verifyResult,
			BuildPassed:  verifyResult.TestPassed && verifyResult.LintPassed,
		}

		result.Attempts = attempt
		result.FinalResult = updatedResult

		// Step e: If build passes, return Fixed=true
		if updatedResult.BuildPassed {
			result.Fixed = true
			emit("auto_remediate_success", map[string]interface{}{
				"wave":    opts.WaveNum,
				"attempt": attempt,
			})
			return result, nil
		}

		// Step f: Still failing — update currentFailed and loop
		currentFailed = updatedResult
	}

	// Step g: All retries exhausted — return Fixed=false
	emit("auto_remediate_exhausted", map[string]interface{}{
		"wave":     opts.WaveNum,
		"attempts": opts.MaxRetries,
		"reason":   "max retries reached",
	})

	return result, nil
}

// extractErrorLog builds a combined error log string from a FinalizeWaveResult.
func extractErrorLog(r *FinalizeWaveResult) string {
	if r == nil {
		return ""
	}
	if r.VerifyBuild == nil {
		return ""
	}

	var parts []string
	if r.VerifyBuild.TestOutput != "" {
		parts = append(parts, "=== Test Output ===\n"+r.VerifyBuild.TestOutput)
	}
	if r.VerifyBuild.LintOutput != "" {
		parts = append(parts, "=== Lint Output ===\n"+r.VerifyBuild.LintOutput)
	}
	return strings.Join(parts, "\n\n")
}

// determineFailedGateType returns the gate type string for the first failing gate
// in the FinalizeWaveResult. Prefers "test" over "lint" when both fail.
func determineFailedGateType(r *FinalizeWaveResult) string {
	if r == nil {
		return ""
	}
	if r.VerifyBuild != nil {
		if !r.VerifyBuild.TestPassed {
			return "test"
		}
		if !r.VerifyBuild.LintPassed {
			return "lint"
		}
	}
	// Check gate results for more specific types (typecheck, build, custom)
	for _, gate := range r.GateResults {
		if gate.Required && !gate.Passed {
			return gate.Type
		}
	}
	return ""
}
