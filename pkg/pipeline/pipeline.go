package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// Pipeline is a named sequence of steps executed in order.
// It is safe for concurrent use: each Run call operates on a caller-supplied
// State so no shared mutable state lives on the Pipeline itself.
type Pipeline struct {
	name  string
	steps []Step
}

// New creates an empty pipeline with the given name.
func New(name string) *Pipeline {
	return &Pipeline{name: name}
}

// AddStep appends a step to the pipeline and returns the pipeline for chaining.
func (p *Pipeline) AddStep(step Step) *Pipeline {
	p.steps = append(p.steps, step)
	return p
}

// RunData carries metadata about a completed pipeline run.
type RunData struct {
	StepsExecuted int
	TotalDuration time.Duration
}

// StepData carries metadata about a completed step execution.
type StepData struct {
	StepName string
	Duration time.Duration
}

// Run executes all steps sequentially, respecting conditions, error strategies,
// and context cancellation.
//
// Condition semantics:
//   - "" or "always"     — always run
//   - "on_success"       — run only when state.Errors is empty so far
//   - "on_failure"       — run only when state.Errors is non-empty
//
// ErrorStrategy semantics:
//   - "fail"             — return error immediately (default)
//   - "continue"         — append error to state.Errors and continue
//   - "retry"            — retry up to step.MaxRetries additional attempts;
//                          on final failure apply the underlying fail/continue
//                          behaviour (treated as "fail" when using ErrorRetry)
//
// Returns result.Result[RunData]:
//   - SUCCESS: all steps executed without error
//   - PARTIAL: some steps failed with ErrorContinue but pipeline continued
//   - FATAL: pipeline aborted due to a step failure or context cancellation
func (p *Pipeline) Run(ctx context.Context, state *State) result.Result[RunData] {
	start := time.Now()
	stepsExecuted := 0
	var warnings []result.SAWError

	for _, step := range p.steps {
		// Check context cancellation before each step.
		select {
		case <-ctx.Done():
			return result.NewFailure[RunData]([]result.SAWError{
				result.NewFatal(result.CodePipelineRunFailed,
					fmt.Sprintf("pipeline %q cancelled before step %q: %v", p.name, step.Name, ctx.Err())),
			})
		default:
		}

		// Evaluate condition.
		if !shouldRun(step.Condition, state) {
			continue
		}

		// Execute the step (with optional retries).
		stepResult := executeStep(ctx, step, state)
		stepsExecuted++

		if stepResult.IsFatal() {
			switch step.ErrorStrategy {
			case ErrorContinue:
				// Append the step errors as warnings and continue.
				for _, e := range stepResult.Errors {
					wrapped := result.NewWarning(e.Code,
						fmt.Sprintf("step %q: %s", step.Name, e.Message))
					warnings = append(warnings, wrapped)
					state.Errors = append(state.Errors, wrapped)
				}
			default: // ErrorFail or unset
				return result.NewFailure[RunData]([]result.SAWError{
					result.NewFatal(result.CodePipelineRunFailed,
						fmt.Sprintf("pipeline %q step %q failed: %s",
							p.name, step.Name, stepResult.Errors[0].Message)),
				})
			}
		}
	}

	data := RunData{
		StepsExecuted: stepsExecuted,
		TotalDuration: time.Since(start),
	}

	if len(warnings) > 0 {
		return result.NewPartial(data, warnings)
	}
	return result.NewSuccess(data)
}

// shouldRun returns true when the step's condition allows execution.
func shouldRun(condition string, state *State) bool {
	switch condition {
	case "on_success":
		return len(state.Errors) == 0
	case "on_failure":
		return len(state.Errors) > 0
	default: // "" or "always"
		return true
	}
}

// executeStep runs a step function, retrying according to the step's
// MaxRetries setting when ErrorStrategy is ErrorRetry.
//
// Returns result.Result[StepData]:
//   - SUCCESS: step executed without error, with timing metadata
//   - FATAL: step returned an error (after all retries are exhausted)
func executeStep(ctx context.Context, step Step, state *State) result.Result[StepData] {
	if step.Func == nil {
		return result.NewFailure[StepData]([]result.SAWError{
			result.NewFatal(result.CodeStepExecutionFailed,
				fmt.Sprintf("step %q has no function", step.Name)),
		})
	}

	start := time.Now()

	if step.ErrorStrategy != ErrorRetry {
		if err := step.Func(ctx, state); err != nil {
			return result.NewFailure[StepData]([]result.SAWError{
				result.NewFatal(result.CodeStepExecutionFailed, err.Error()).WithCause(err),
			})
		}
		return result.NewSuccess(StepData{
			StepName: step.Name,
			Duration: time.Since(start),
		})
	}

	// Retry loop.
	maxAttempts := 1
	if step.MaxRetries > 0 {
		maxAttempts = 1 + step.MaxRetries
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Respect context cancellation between retries.
		select {
		case <-ctx.Done():
			return result.NewFailure[StepData]([]result.SAWError{
				result.NewFatal(result.CodeStepExecutionFailed, ctx.Err().Error()).WithCause(ctx.Err()),
			})
		default:
		}

		lastErr = step.Func(ctx, state)
		if lastErr == nil {
			return result.NewSuccess(StepData{
				StepName: step.Name,
				Duration: time.Since(start),
			})
		}
	}
	return result.NewFailure[StepData]([]result.SAWError{
		result.NewFatal(result.CodeStepExecutionFailed, lastErr.Error()).WithCause(lastErr),
	})
}
