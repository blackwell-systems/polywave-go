package pipeline

import (
	"context"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

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
	Values   map[string]any
	Errors   []result.SAWError
}

// GetValue retrieves a typed value from state.Values.
// Returns (zero, false) if the key is absent or value cannot be asserted to T.
func GetValue[T any](state *State, key string) (T, bool) {
	if state.Values == nil {
		var zero T
		return zero, false
	}
	v, ok := state.Values[key]
	if !ok {
		var zero T
		return zero, false
	}
	typed, ok := v.(T)
	return typed, ok
}

// SetValue sets key in state.Values, initializing the map if nil.
func SetValue(state *State, key string, val any) {
	if state.Values == nil {
		state.Values = make(map[string]any)
	}
	state.Values[key] = val
}
