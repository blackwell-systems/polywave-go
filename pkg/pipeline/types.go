package pipeline

import "context"

// StepFunc is a single pipeline step. It receives the pipeline context
// and returns an error. Steps are executed in sequence within a stage;
// stages may run in parallel.
type StepFunc func(ctx context.Context, state *State) error

// ErrorStrategy defines how the pipeline handles a step failure.
type ErrorStrategy string

const (
	ErrorFail     ErrorStrategy = "fail"     // Abort pipeline immediately
	ErrorContinue ErrorStrategy = "continue" // Log error, continue to next step
	ErrorRetry    ErrorStrategy = "retry"    // Retry step up to MaxRetries times
)

// Step is a named, executable unit in a pipeline.
type Step struct {
	Name          string        `yaml:"name"`
	Func          StepFunc      `yaml:"-"`
	ErrorStrategy ErrorStrategy `yaml:"on_error"`
	MaxRetries    int           `yaml:"max_retries,omitempty"`
	Condition     string        `yaml:"condition,omitempty"` // "always" | "on_success" | "on_failure"
}

// State carries mutable data through the pipeline.
type State struct {
	RepoPath string
	IMPLPath string
	WaveNum  int
	Values   map[string]interface{}
	Errors   []error
}
