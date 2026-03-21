package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
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
		validateFn: func(implPath string) ([]protocol.ValidationError, error) {
			// Passes on first try.
			return nil, nil
		},
		setStateFn: func(implPath string) error {
			t.Fatal("setStateFn should not be called on success")
			return nil
		},
	}

	err := ScoutCorrectionLoop(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if scoutCalls != 1 {
		t.Fatalf("expected 1 RunScout call, got %d", scoutCalls)
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
		validateFn: func(implPath string) ([]protocol.ValidationError, error) {
			// Fail first time, pass second time.
			if scoutCalls == 1 {
				return []protocol.ValidationError{
					{Code: "E16_001", Message: "missing slug field"},
				}, nil
			}
			return nil, nil
		},
		setStateFn: func(implPath string) error {
			t.Fatal("setStateFn should not be called when retry succeeds")
			return nil
		},
	}

	err := ScoutCorrectionLoop(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
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
		validateFn: func(implPath string) ([]protocol.ValidationError, error) {
			// Always fail.
			return []protocol.ValidationError{
				{Code: "E16_001", Message: "missing slug field"},
				{Code: "E16_002", Message: "no waves defined"},
			}, nil
		},
		setStateFn: func(implPath string) error {
			stateSet = true
			return nil
		},
	}

	err := ScoutCorrectionLoop(context.Background(), opts, nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "validation failed after 2 retries") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "missing slug field") {
		t.Fatalf("expected error details in message: %v", err)
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

	err := ScoutCorrectionLoop(ctx, opts, nil)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestBuildCorrectionPrompt(t *testing.T) {
	errors := []protocol.ValidationError{
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
