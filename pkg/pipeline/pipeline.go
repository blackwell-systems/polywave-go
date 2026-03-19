package pipeline

import (
	"context"
	"fmt"
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
func (p *Pipeline) Run(ctx context.Context, state *State) error {
	for _, step := range p.steps {
		// Check context cancellation before each step.
		select {
		case <-ctx.Done():
			return fmt.Errorf("pipeline %q cancelled before step %q: %w", p.name, step.Name, ctx.Err())
		default:
		}

		// Evaluate condition.
		if !shouldRun(step.Condition, state) {
			continue
		}

		// Execute the step (with optional retries).
		err := executeStep(ctx, step, state)
		if err != nil {
			switch step.ErrorStrategy {
			case ErrorContinue:
				state.Errors = append(state.Errors, fmt.Errorf("step %q: %w", step.Name, err))
				// Continue to next step.
			default: // ErrorFail or unset
				return fmt.Errorf("pipeline %q step %q failed: %w", p.name, step.Name, err)
			}
		}
	}
	return nil
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
func executeStep(ctx context.Context, step Step, state *State) error {
	if step.Func == nil {
		return fmt.Errorf("step %q has no function", step.Name)
	}

	if step.ErrorStrategy != ErrorRetry {
		return step.Func(ctx, state)
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
			return ctx.Err()
		default:
		}

		lastErr = step.Func(ctx, state)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}
