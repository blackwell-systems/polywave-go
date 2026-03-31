package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// makeState returns a minimal State suitable for most tests.
func makeState() *State {
	return &State{
		Values: make(map[string]interface{}),
	}
}

// TestPipeline_SequentialExecution verifies that steps run in declaration order.
func TestPipeline_SequentialExecution(t *testing.T) {
	var order []int
	step := func(n int) Step {
		return Step{
			Name:          fmt.Sprintf("step-%d", n),
			ErrorStrategy: ErrorFail,
			Func: func(ctx context.Context, state *State) error {
				order = append(order, n)
				return nil
			},
		}
	}

	p := New("seq").AddStep(step(1)).AddStep(step(2)).AddStep(step(3))
	r := p.Run(context.Background(), makeState())
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("wrong execution order: %v", order)
	}
}

// TestPipeline_RunData_StepsExecuted verifies that RunData.StepsExecuted matches
// the number of steps that actually ran.
func TestPipeline_RunData_StepsExecuted(t *testing.T) {
	p := New("count-test")
	for i := 0; i < 4; i++ {
		p.AddStep(Step{
			Name:          fmt.Sprintf("step-%d", i),
			ErrorStrategy: ErrorFail,
			Func: func(ctx context.Context, state *State) error {
				return nil
			},
		})
	}

	r := p.Run(context.Background(), makeState())
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	data := r.GetData()
	if data.StepsExecuted != 4 {
		t.Fatalf("expected StepsExecuted=4, got %d", data.StepsExecuted)
	}
}

// TestPipeline_ErrorFail verifies that a failing step with ErrorFail aborts
// the pipeline immediately and returns a FATAL result.
func TestPipeline_ErrorFail(t *testing.T) {
	ran := false
	p := New("fail-test")
	p.AddStep(Step{
		Name:          "bad",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			return errors.New("boom")
		},
	})
	p.AddStep(Step{
		Name:          "should-not-run",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			ran = true
			return nil
		},
	})

	r := p.Run(context.Background(), makeState())
	if !r.IsFatal() {
		t.Fatal("expected FATAL result, got success")
	}
	if ran {
		t.Fatal("second step should not have run after ErrorFail")
	}
}

// TestPipeline_ErrorContinue verifies that a failing step with ErrorContinue
// appends the error to state.Errors but lets subsequent steps run,
// and returns a PARTIAL result.
func TestPipeline_ErrorContinue(t *testing.T) {
	ran := false
	p := New("continue-test")
	p.AddStep(Step{
		Name:          "soft-fail",
		ErrorStrategy: ErrorContinue,
		Func: func(ctx context.Context, state *State) error {
			return errors.New("non-fatal")
		},
	})
	p.AddStep(Step{
		Name:          "should-run",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			ran = true
			return nil
		},
	})

	state := makeState()
	r := p.Run(context.Background(), state)
	if r.IsFatal() {
		t.Fatalf("pipeline returned fatal: %v", r.Errors)
	}
	if !r.IsPartial() {
		t.Fatalf("expected PARTIAL result, got code=%s", r.Code)
	}
	if !ran {
		t.Fatal("second step did not run after ErrorContinue")
	}
	if len(state.Errors) != 1 {
		t.Fatalf("expected 1 error in state, got %d", len(state.Errors))
	}
}

// TestPipeline_ErrorRetry verifies that a step is retried the configured number
// of times before ultimately failing with a FATAL result.
func TestPipeline_ErrorRetry(t *testing.T) {
	attempts := 0
	p := New("retry-test")
	p.AddStep(Step{
		Name:          "flaky",
		ErrorStrategy: ErrorRetry,
		MaxRetries:    2,
		Func: func(ctx context.Context, state *State) error {
			attempts++
			return errors.New("still failing")
		},
	})

	r := p.Run(context.Background(), makeState())
	if !r.IsFatal() {
		t.Fatal("expected FATAL result after exhausting retries")
	}
	// 1 initial attempt + 2 retries = 3 total
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

// TestPipeline_ErrorRetry_Success verifies that a step that eventually succeeds
// within the retry budget returns a SUCCESS result.
func TestPipeline_ErrorRetry_Success(t *testing.T) {
	attempts := 0
	p := New("retry-success")
	p.AddStep(Step{
		Name:          "eventually-ok",
		ErrorStrategy: ErrorRetry,
		MaxRetries:    3,
		Func: func(ctx context.Context, state *State) error {
			attempts++
			if attempts < 3 {
				return errors.New("not yet")
			}
			return nil
		},
	})

	r := p.Run(context.Background(), makeState())
	if r.IsFatal() {
		t.Fatalf("expected success, got FATAL: %v", r.Errors)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

// TestPipeline_ContextCancellation verifies that a cancelled context stops
// execution before the next step starts and returns a FATAL result.
func TestPipeline_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	secondRan := false
	p := New("cancel-test")
	p.AddStep(Step{
		Name:          "first",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			cancel() // cancel after first step
			return nil
		},
	})
	p.AddStep(Step{
		Name:          "second",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			secondRan = true
			return nil
		},
	})

	r := p.Run(ctx, makeState())
	if !r.IsFatal() {
		t.Fatal("expected FATAL result due to cancellation")
	}
	if secondRan {
		t.Fatal("second step should not have run after cancellation")
	}
}

// TestPipeline_ConditionOnSuccess verifies that a step with "on_success"
// is skipped when there are prior errors.
func TestPipeline_ConditionOnSuccess(t *testing.T) {
	ran := false
	state := makeState()
	state.Errors = []error{errors.New("prior error")}

	p := New("cond-success")
	p.AddStep(Step{
		Name:          "guarded",
		ErrorStrategy: ErrorFail,
		Condition:     "on_success",
		Func: func(ctx context.Context, s *State) error {
			ran = true
			return nil
		},
	})

	r := p.Run(context.Background(), state)
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	if ran {
		t.Fatal("on_success step should not have run when errors exist")
	}
}

// TestPipeline_ConditionOnFailure verifies that a step with "on_failure"
// runs only when prior errors exist.
func TestPipeline_ConditionOnFailure(t *testing.T) {
	ranWhenClean := false
	ranWhenDirty := false

	// Pipeline with no prior errors: on_failure step should be skipped.
	p1 := New("cond-failure-clean")
	p1.AddStep(Step{
		Name:          "on-fail-step",
		ErrorStrategy: ErrorFail,
		Condition:     "on_failure",
		Func: func(ctx context.Context, s *State) error {
			ranWhenClean = true
			return nil
		},
	})
	if r := p1.Run(context.Background(), makeState()); r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	if ranWhenClean {
		t.Fatal("on_failure step should not run when state has no errors")
	}

	// Pipeline with prior errors: on_failure step should run.
	dirtyState := makeState()
	dirtyState.Errors = []error{errors.New("something failed")}

	p2 := New("cond-failure-dirty")
	p2.AddStep(Step{
		Name:          "on-fail-step",
		ErrorStrategy: ErrorFail,
		Condition:     "on_failure",
		Func: func(ctx context.Context, s *State) error {
			ranWhenDirty = true
			return nil
		},
	})
	if r := p2.Run(context.Background(), dirtyState); r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	if !ranWhenDirty {
		t.Fatal("on_failure step should have run when state has errors")
	}
}

// TestState_ValuesMap verifies that state.Values correctly carries data between steps.
func TestState_ValuesMap(t *testing.T) {
	p := New("values-test")
	p.AddStep(Step{
		Name:          "writer",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			state.Values["key"] = "hello"
			return nil
		},
	})
	p.AddStep(Step{
		Name:          "reader",
		ErrorStrategy: ErrorFail,
		Func: func(ctx context.Context, state *State) error {
			if state.Values["key"] != "hello" {
				return fmt.Errorf("expected 'hello', got %v", state.Values["key"])
			}
			return nil
		},
	})

	r := p.Run(context.Background(), makeState())
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
}
