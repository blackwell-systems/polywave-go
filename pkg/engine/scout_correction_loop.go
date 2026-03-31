package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// MaxScoutCorrectionRetries is the default number of correction retries before giving up.
const MaxScoutCorrectionRetries = 3

// CorrectionData holds data returned by ScoutCorrectionLoop.
type CorrectionData struct {
	Attempts    int    `json:"attempts"`
	IMPLOutPath string `json:"impl_out_path"`
}

// SetBlockedData holds data returned by setIMPLStateBlocked.
type SetBlockedData struct {
	IMPLPath string `json:"impl_path"`
}

// ScoutCorrectionOpts configures the E16 Scout correction loop.
type ScoutCorrectionOpts struct {
	ScoutOpts  RunScoutOpts
	MaxRetries int                              // default 3 if zero
	OnRetry    func(attempt int, errors []string) // optional callback on each retry

	// runScoutFn overrides RunScout for testing. If nil, uses the real RunScout.
	runScoutFn func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error
	// validateFn overrides IMPL doc validation for testing. If nil, uses validateIMPLDoc.
	validateFn func(implPath string) ([]result.SAWError, error)
	// setStateFn overrides state-setting for testing. If nil, uses setIMPLStateBlocked.
	setStateFn func(implPath string) result.Result[SetBlockedData]
}

// ScoutCorrectionLoop runs RunScout followed by E16 validation, retrying up to
// MaxRetries times if the IMPL doc has validation errors. On each retry it
// prepends a correction prompt describing the specific failures so the Scout
// agent can fix them.
//
// If validation passes on any attempt, it returns a SUCCESS result.
// If all retries are exhausted, it sets the IMPL doc state to BLOCKED and
// returns a FATAL result.
func ScoutCorrectionLoop(ctx context.Context, opts ScoutCorrectionOpts, onChunk func(string)) result.Result[CorrectionData] {
	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = MaxScoutCorrectionRetries
	}

	runScout := opts.runScoutFn
	if runScout == nil {
		runScout = func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
			res := RunScout(ctx, opts, onChunk)
			if res.IsFatal() {
				if len(res.Errors) > 0 {
					return fmt.Errorf("%s", res.Errors[0].Message)
				}
				return fmt.Errorf("RunScout failed")
			}
			return nil
		}
	}
	validate := opts.validateFn
	if validate == nil {
		validate = validateIMPLDoc
	}
	setState := opts.setStateFn
	if setState == nil {
		setState = setIMPLStateBlocked
	}

	var lastErrors []result.SAWError
	attempts := 0

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before each attempt.
		select {
		case <-ctx.Done():
			return result.NewFailure[CorrectionData]([]result.SAWError{
				result.NewFatal("CONTEXT_CANCELLED",
					fmt.Sprintf("context cancelled: %v", ctx.Err())),
			})
		default:
		}

		attempts = attempt + 1

		// On retries, prepend a correction prompt to the feature description.
		scoutOpts := opts.ScoutOpts
		if attempt > 0 && len(lastErrors) > 0 {
			correctionPrompt := buildCorrectionPrompt(lastErrors)
			scoutOpts.Feature = correctionPrompt + "\n\n" + opts.ScoutOpts.Feature

			if opts.OnRetry != nil {
				errorStrs := make([]string, len(lastErrors))
				for i, e := range lastErrors {
					errorStrs[i] = e.Message
				}
				opts.OnRetry(attempt, errorStrs)
			}
		}

		// Run the Scout agent.
		err := runScout(ctx, scoutOpts, onChunk)
		if err != nil {
			return result.NewFailure[CorrectionData]([]result.SAWError{
				result.NewFatal("ENGINE_SCOUT_RUN_FAILED",
					fmt.Sprintf("scout correction loop: RunScout failed on attempt %d: %v", attempts, err)).
					WithContext("attempt", fmt.Sprintf("%d", attempts)),
			})
		}

		// Validate the output IMPL doc.
		validationErrors, err := validate(scoutOpts.IMPLOutPath)
		if err != nil {
			return result.NewFailure[CorrectionData]([]result.SAWError{
				result.NewFatal("ENGINE_SCOUT_VALIDATION_FAILED",
					fmt.Sprintf("scout correction loop: validation failed on attempt %d: %v", attempts, err)).
					WithContext("attempt", fmt.Sprintf("%d", attempts)),
			})
		}

		if len(validationErrors) == 0 {
			// Validation passed.
			return result.NewSuccess(CorrectionData{
				Attempts:    attempts,
				IMPLOutPath: scoutOpts.IMPLOutPath,
			})
		}

		lastErrors = validationErrors
	}

	// All retries exhausted. Set IMPL doc state to blocked.
	setRes := setState(opts.ScoutOpts.IMPLOutPath)
	if setRes.IsFatal() {
		// Non-fatal: log but still return the validation error.
		fmt.Printf("scout correction loop: failed to set IMPL state to BLOCKED: %s\n", setRes.Errors[0].Message)
	}

	errorMsgs := make([]string, len(lastErrors))
	for i, e := range lastErrors {
		errorMsgs[i] = e.Message
	}
	return result.NewFailure[CorrectionData]([]result.SAWError{
		result.NewFatal("ENGINE_SCOUT_CORRECTION_EXHAUSTED",
			fmt.Sprintf("scout correction loop: validation failed after %d retries: %s",
				maxRetries, strings.Join(errorMsgs, "; "))).
			WithContext("retries", fmt.Sprintf("%d", maxRetries)),
	})
}

// buildCorrectionPrompt constructs a prompt describing validation errors for
// the Scout agent to fix on retry.
func buildCorrectionPrompt(errors []result.SAWError) string {
	var sb strings.Builder
	sb.WriteString("The IMPL doc you produced has validation errors. Fix the following issues:\n")
	for i, e := range errors {
		if e.Field != "" {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s (field: %s)\n", i+1, e.Code, e.Message, e.Field))
		} else {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, e.Code, e.Message))
		}
	}
	return sb.String()
}

// validateIMPLDoc loads an IMPL doc and runs E16 validation, returning any errors.
func validateIMPLDoc(implPath string) ([]result.SAWError, error) {
	manifest, err := protocol.Load(implPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load IMPL doc: %w", err)
	}
	return protocol.Validate(manifest), nil
}

// setIMPLStateBlocked updates the IMPL doc state to BLOCKED after exhausting
// correction retries.
func setIMPLStateBlocked(implPath string) result.Result[SetBlockedData] {
	manifest, err := protocol.Load(implPath)
	if err != nil {
		return result.NewFailure[SetBlockedData]([]result.SAWError{
			result.NewFatal("ENGINE_SET_BLOCKED_LOAD_FAILED",
				fmt.Sprintf("failed to load manifest: %v", err)).
				WithContext("impl_path", implPath),
		})
	}
	manifest.State = protocol.StateBlocked
	if saveRes := protocol.Save(manifest, implPath); saveRes.IsFatal() {
		errMsg := "failed to save manifest"
		if len(saveRes.Errors) > 0 {
			errMsg = saveRes.Errors[0].Message
		}
		return result.NewFailure[SetBlockedData]([]result.SAWError{
			result.NewFatal("ENGINE_SET_BLOCKED_SAVE_FAILED", errMsg).
				WithContext("impl_path", implPath),
		})
	}
	return result.NewSuccess(SetBlockedData{IMPLPath: implPath})
}
