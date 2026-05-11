package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func TestScoutCorrectionLoop_SucceedsFirstTry(t *testing.T) {
	scoutCalls := 0
	opts := ScoutCorrectionOpts{
		ScoutOpts: RunScoutOpts{
			Feature:     "test feature",
			RepoPath:    "/tmp/repo",
			IMPLOutPath: "/tmp/impl.yaml",
		},
		MaxRetries: 3,
		runScoutFn: func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
			scoutCalls++
			return nil
		},
		validateFn: func(ctx context.Context, implPath string) ([]result.PolywaveError, error) {
			// Passes on first try.
			return nil, nil
		},
		setStateFn: func(ctx context.Context, implPath string) result.Result[SetBlockedData] {
			t.Fatal("setStateFn should not be called on success")
			return result.NewSuccess(SetBlockedData{})
		},
	}

	res := ScoutCorrectionLoop(context.Background(), opts, nil)
	if res.IsFatal() {
		t.Fatalf("expected success result, got fatal: %v", res.Errors)
	}
	if scoutCalls != 1 {
		t.Fatalf("expected 1 RunScout call, got %d", scoutCalls)
	}
	data := res.GetData()
	if data.Attempts != 1 {
		t.Errorf("expected Attempts=1, got %d", data.Attempts)
	}
}

func TestScoutCorrectionLoop_RetriesAndSucceeds(t *testing.T) {
	scoutCalls := 0
	retryCalls := 0

	opts := ScoutCorrectionOpts{
		ScoutOpts: RunScoutOpts{
			Feature:     "test feature",
			RepoPath:    "/tmp/repo",
			IMPLOutPath: "/tmp/impl.yaml",
		},
		MaxRetries: 3,
		OnRetry: func(attempt int, errors []string) {
			retryCalls++
		},
		runScoutFn: func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
			scoutCalls++
			// On retry, verify the correction prompt is prepended.
			if scoutCalls > 1 {
				if !strings.Contains(opts.Feature, "validation errors") {
					t.Errorf("expected correction prompt in Feature on retry, got: %s", opts.Feature)
				}
			}
			return nil
		},
		validateFn: func(ctx context.Context, implPath string) ([]result.PolywaveError, error) {
			// Fail first time, pass second time.
			if scoutCalls == 1 {
				return []result.PolywaveError{
					{Code: "E16_001", Message: "missing slug field"},
				}, nil
			}
			return nil, nil
		},
		setStateFn: func(ctx context.Context, implPath string) result.Result[SetBlockedData] {
			t.Fatal("setStateFn should not be called when retry succeeds")
			return result.NewSuccess(SetBlockedData{})
		},
	}

	res := ScoutCorrectionLoop(context.Background(), opts, nil)
	if res.IsFatal() {
		t.Fatalf("expected success result, got fatal: %v", res.Errors)
	}
	if scoutCalls != 2 {
		t.Fatalf("expected 2 RunScout calls, got %d", scoutCalls)
	}
	if retryCalls != 1 {
		t.Fatalf("expected 1 OnRetry call, got %d", retryCalls)
	}
}

func TestScoutCorrectionLoop_ExhaustsRetriesAndSetsState(t *testing.T) {
	scoutCalls := 0
	stateSet := false

	opts := ScoutCorrectionOpts{
		ScoutOpts: RunScoutOpts{
			Feature:     "test feature",
			RepoPath:    "/tmp/repo",
			IMPLOutPath: "/tmp/impl.yaml",
		},
		MaxRetries: 2,
		runScoutFn: func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
			scoutCalls++
			return nil
		},
		validateFn: func(ctx context.Context, implPath string) ([]result.PolywaveError, error) {
			// Always fail.
			return []result.PolywaveError{
				{Code: "E16_001", Message: "missing slug field"},
				{Code: "E16_002", Message: "no waves defined"},
			}, nil
		},
		setStateFn: func(ctx context.Context, implPath string) result.Result[SetBlockedData] {
			stateSet = true
			return result.NewSuccess(SetBlockedData{IMPLPath: implPath})
		},
	}

	res := ScoutCorrectionLoop(context.Background(), opts, nil)
	if !res.IsFatal() {
		t.Fatal("expected fatal result after exhausting retries, got non-fatal")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors in fatal result")
	}
	if !strings.Contains(res.Errors[0].Message, "validation failed after 2 retries") {
		t.Fatalf("unexpected error message: %v", res.Errors[0].Message)
	}
	if !strings.Contains(res.Errors[0].Message, "missing slug field") {
		t.Fatalf("expected error details in message: %v", res.Errors[0].Message)
	}
	// Initial attempt + 2 retries = 3 total calls.
	if scoutCalls != 3 {
		t.Fatalf("expected 3 RunScout calls (1 initial + 2 retries), got %d", scoutCalls)
	}
	if !stateSet {
		t.Fatal("expected IMPL state to be set to BLOCKED")
	}
}

func TestScoutCorrectionLoop_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	opts := ScoutCorrectionOpts{
		ScoutOpts: RunScoutOpts{
			Feature:     "test",
			RepoPath:    "/tmp/repo",
			IMPLOutPath: "/tmp/impl.yaml",
		},
		runScoutFn: func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
			t.Fatal("RunScout should not be called with cancelled context")
			return nil
		},
	}

	res := ScoutCorrectionLoop(ctx, opts, nil)
	if !res.IsFatal() {
		t.Fatal("expected fatal result for cancelled context")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors in fatal result")
	}
	if res.Errors[0].Code != result.CodeContextCancelled {
		t.Errorf("expected CONTEXT_CANCELLED code, got %q", res.Errors[0].Code)
	}
}

func TestBuildCorrectionPrompt(t *testing.T) {
	errors := []result.PolywaveError{
		{Code: "E16_001", Message: "missing slug", Field: "slug"},
		{Code: "E16_002", Message: "no waves defined"},
	}

	prompt := buildCorrectionPrompt(errors)
	if !strings.Contains(prompt, "validation errors") {
		t.Fatal("expected 'validation errors' header")
	}
	if !strings.Contains(prompt, "missing slug") {
		t.Fatal("expected error message in prompt")
	}
	if !strings.Contains(prompt, "field: slug") {
		t.Fatal("expected field annotation in prompt")
	}
	if !strings.Contains(prompt, "[E16_002]") {
		t.Fatal("expected error code in prompt")
	}
}
