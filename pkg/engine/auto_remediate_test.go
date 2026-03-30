package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestAutoRemediate_FixesOnFirstAttempt verifies that when FixBuildFailure succeeds
// and the subsequent VerifyBuild passes, AutoRemediate returns Fixed=true with 1 attempt.
func TestAutoRemediate_FixesOnFirstAttempt(t *testing.T) {
	// Save and restore originals
	origFix := autoRemediateFixBuildFailureFunc
	origVerify := autoRemediateVerifyBuildFunc
	defer func() {
		autoRemediateFixBuildFailureFunc = origFix
		autoRemediateVerifyBuildFunc = origVerify
	}()

	autoRemediateFixBuildFailureFunc = func(ctx context.Context, opts FixBuildOpts) error {
		return nil // fix succeeds
	}
	autoRemediateVerifyBuildFunc = func(implPath, repoPath string) (*protocol.VerifyBuildData, error) {
		return &protocol.VerifyBuildData{
			TestPassed: true,
			LintPassed: true,
		}, nil
	}

	failedResult := &FinalizeWaveResult{
		Wave:          1,
		BuildPassed:   false,
		VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: false, LintPassed: true, TestOutput: "FAIL: some test error"}},
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}

	result, err := AutoRemediate(context.Background(), AutoRemediateOpts{
		IMPLPath:     "/tmp/fake/IMPL.yaml",
		RepoPath:     "/tmp/fake/repo",
		WaveNum:      1,
		FailedResult: failedResult,
		MaxRetries:   3,
		ChatModel:    "test-model",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Fixed {
		t.Error("expected Fixed=true on first attempt")
	}
	if result.Attempts != 1 {
		t.Errorf("expected Attempts=1, got %d", result.Attempts)
	}
	if result.FinalResult == nil {
		t.Fatal("expected non-nil FinalResult")
	}
	if !result.FinalResult.BuildPassed {
		t.Error("expected FinalResult.BuildPassed=true")
	}
}

// TestAutoRemediate_FixesOnSecondAttempt verifies that when the first fix attempt
// fails but the second succeeds, AutoRemediate returns Fixed=true with 2 attempts.
func TestAutoRemediate_FixesOnSecondAttempt(t *testing.T) {
	origFix := autoRemediateFixBuildFailureFunc
	origVerify := autoRemediateVerifyBuildFunc
	defer func() {
		autoRemediateFixBuildFailureFunc = origFix
		autoRemediateVerifyBuildFunc = origVerify
	}()

	verifyCallCount := 0
	autoRemediateFixBuildFailureFunc = func(ctx context.Context, opts FixBuildOpts) error {
		return nil // always "succeeds" from the fix agent perspective
	}
	autoRemediateVerifyBuildFunc = func(implPath, repoPath string) (*protocol.VerifyBuildData, error) {
		verifyCallCount++
		if verifyCallCount == 1 {
			// First verify: still failing
			return &protocol.VerifyBuildData{
				TestPassed: false,
				LintPassed: true,
				TestOutput: "FAIL: still broken",
			}, nil
		}
		// Second verify: passing
		return &protocol.VerifyBuildData{
			TestPassed: true,
			LintPassed: true,
		}, nil
	}

	failedResult := &FinalizeWaveResult{
		Wave:          1,
		BuildPassed:   false,
		VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: false, LintPassed: true, TestOutput: "FAIL: original error"}},
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}

	result, err := AutoRemediate(context.Background(), AutoRemediateOpts{
		IMPLPath:     "/tmp/fake/IMPL.yaml",
		RepoPath:     "/tmp/fake/repo",
		WaveNum:      1,
		FailedResult: failedResult,
		MaxRetries:   3,
		ChatModel:    "test-model",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Fixed {
		t.Error("expected Fixed=true on second attempt")
	}
	if result.Attempts != 2 {
		t.Errorf("expected Attempts=2, got %d", result.Attempts)
	}
	if verifyCallCount != 2 {
		t.Errorf("expected 2 verify calls, got %d", verifyCallCount)
	}
}

// TestAutoRemediate_ExhaustsRetries verifies that when all fix attempts fail,
// AutoRemediate returns Fixed=false after MaxRetries attempts.
func TestAutoRemediate_ExhaustsRetries(t *testing.T) {
	origFix := autoRemediateFixBuildFailureFunc
	origVerify := autoRemediateVerifyBuildFunc
	defer func() {
		autoRemediateFixBuildFailureFunc = origFix
		autoRemediateVerifyBuildFunc = origVerify
	}()

	fixCallCount := 0
	verifyCallCount := 0

	autoRemediateFixBuildFailureFunc = func(ctx context.Context, opts FixBuildOpts) error {
		fixCallCount++
		return nil
	}
	autoRemediateVerifyBuildFunc = func(implPath, repoPath string) (*protocol.VerifyBuildData, error) {
		verifyCallCount++
		// Always still failing
		return &protocol.VerifyBuildData{
			TestPassed: false,
			LintPassed: false,
			TestOutput: "FAIL: persistent error",
			LintOutput: "lint: error on every attempt",
		}, nil
	}

	failedResult := &FinalizeWaveResult{
		Wave:          2,
		BuildPassed:   false,
		VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: false, LintPassed: false, TestOutput: "initial failure"}},
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}

	const maxRetries = 4
	result, err := AutoRemediate(context.Background(), AutoRemediateOpts{
		IMPLPath:     "/tmp/fake/IMPL.yaml",
		RepoPath:     "/tmp/fake/repo",
		WaveNum:      2,
		FailedResult: failedResult,
		MaxRetries:   maxRetries,
		ChatModel:    "test-model",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fixed {
		t.Error("expected Fixed=false when all retries fail")
	}
	if result.Attempts != maxRetries {
		t.Errorf("expected Attempts=%d, got %d", maxRetries, result.Attempts)
	}
	if fixCallCount != maxRetries {
		t.Errorf("expected %d fix calls, got %d", maxRetries, fixCallCount)
	}
	if verifyCallCount != maxRetries {
		t.Errorf("expected %d verify calls, got %d", maxRetries, verifyCallCount)
	}
}

// TestAutoRemediate_ZeroRetries verifies that when MaxRetries=0, AutoRemediate
// returns immediately without making any fix or verify calls.
func TestAutoRemediate_ZeroRetries(t *testing.T) {
	origFix := autoRemediateFixBuildFailureFunc
	origVerify := autoRemediateVerifyBuildFunc
	defer func() {
		autoRemediateFixBuildFailureFunc = origFix
		autoRemediateVerifyBuildFunc = origVerify
	}()

	fixCalled := false
	verifyCalled := false

	autoRemediateFixBuildFailureFunc = func(ctx context.Context, opts FixBuildOpts) error {
		fixCalled = true
		return nil
	}
	autoRemediateVerifyBuildFunc = func(implPath, repoPath string) (*protocol.VerifyBuildData, error) {
		verifyCalled = true
		return &protocol.VerifyBuildData{TestPassed: true, LintPassed: true}, nil
	}

	failedResult := &FinalizeWaveResult{
		Wave:          1,
		BuildPassed:   false,
		VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: false}},
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}

	result, err := AutoRemediate(context.Background(), AutoRemediateOpts{
		IMPLPath:     "/tmp/fake/IMPL.yaml",
		RepoPath:     "/tmp/fake/repo",
		WaveNum:      1,
		FailedResult: failedResult,
		MaxRetries:   0,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fixed {
		t.Error("expected Fixed=false with zero retries")
	}
	if result.Attempts != 0 {
		t.Errorf("expected Attempts=0, got %d", result.Attempts)
	}
	if fixCalled {
		t.Error("expected no fix calls with MaxRetries=0")
	}
	if verifyCalled {
		t.Error("expected no verify calls with MaxRetries=0")
	}
}

// TestAutoRemediate_EmitsEvents verifies that the OnEvent callback is called
// with the expected event names throughout the remediation loop.
func TestAutoRemediate_EmitsEvents(t *testing.T) {
	origFix := autoRemediateFixBuildFailureFunc
	origVerify := autoRemediateVerifyBuildFunc
	defer func() {
		autoRemediateFixBuildFailureFunc = origFix
		autoRemediateVerifyBuildFunc = origVerify
	}()

	verifyCallCount := 0
	autoRemediateFixBuildFailureFunc = func(ctx context.Context, opts FixBuildOpts) error {
		return nil
	}
	autoRemediateVerifyBuildFunc = func(implPath, repoPath string) (*protocol.VerifyBuildData, error) {
		verifyCallCount++
		if verifyCallCount == 1 {
			return &protocol.VerifyBuildData{TestPassed: false, LintPassed: true}, nil
		}
		return &protocol.VerifyBuildData{TestPassed: true, LintPassed: true}, nil
	}

	var emittedEvents []string
	onEvent := func(e Event) {
		emittedEvents = append(emittedEvents, e.Event)
	}

	failedResult := &FinalizeWaveResult{
		Wave:          1,
		BuildPassed:   false,
		VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: false}},
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}

	result, err := AutoRemediate(context.Background(), AutoRemediateOpts{
		IMPLPath:     "/tmp/fake/IMPL.yaml",
		RepoPath:     "/tmp/fake/repo",
		WaveNum:      1,
		FailedResult: failedResult,
		MaxRetries:   3,
		ChatModel:    "test-model",
		OnEvent:      onEvent,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Fixed {
		t.Error("expected Fixed=true")
	}

	// Check that expected events were emitted
	wantEvents := []string{
		"auto_remediate_started",
		"auto_remediate_attempt", // attempt 1
		"auto_remediate_attempt", // attempt 2
		"auto_remediate_success",
	}

	if len(emittedEvents) != len(wantEvents) {
		t.Errorf("expected %d events, got %d: %v", len(wantEvents), len(emittedEvents), emittedEvents)
	} else {
		for i, want := range wantEvents {
			if emittedEvents[i] != want {
				t.Errorf("event[%d]: want %q, got %q", i, want, emittedEvents[i])
			}
		}
	}
}

// TestAutoRemediate_EmitsExhaustedEvent verifies that "auto_remediate_exhausted"
// is emitted when all retries fail.
func TestAutoRemediate_EmitsExhaustedEvent(t *testing.T) {
	origFix := autoRemediateFixBuildFailureFunc
	origVerify := autoRemediateVerifyBuildFunc
	defer func() {
		autoRemediateFixBuildFailureFunc = origFix
		autoRemediateVerifyBuildFunc = origVerify
	}()

	autoRemediateFixBuildFailureFunc = func(ctx context.Context, opts FixBuildOpts) error {
		return nil
	}
	autoRemediateVerifyBuildFunc = func(implPath, repoPath string) (*protocol.VerifyBuildData, error) {
		return &protocol.VerifyBuildData{TestPassed: false, LintPassed: true}, nil
	}

	var emittedEvents []string
	onEvent := func(e Event) {
		emittedEvents = append(emittedEvents, e.Event)
	}

	failedResult := &FinalizeWaveResult{
		Wave:          1,
		BuildPassed:   false,
		VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: false}},
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}

	result, err := AutoRemediate(context.Background(), AutoRemediateOpts{
		IMPLPath:     "/tmp/fake/IMPL.yaml",
		RepoPath:     "/tmp/fake/repo",
		WaveNum:      1,
		FailedResult: failedResult,
		MaxRetries:   2,
		OnEvent:      onEvent,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fixed {
		t.Error("expected Fixed=false")
	}

	// Verify "auto_remediate_exhausted" was the last event
	if len(emittedEvents) == 0 {
		t.Fatal("no events emitted")
	}
	lastEvent := emittedEvents[len(emittedEvents)-1]
	if lastEvent != "auto_remediate_exhausted" {
		t.Errorf("expected last event to be %q, got %q", "auto_remediate_exhausted", lastEvent)
	}
}

// TestAutoRemediate_VerifyBuildSystemError verifies that a system-level error from
// VerifyBuild is propagated immediately as an error return (not swallowed).
func TestAutoRemediate_VerifyBuildSystemError(t *testing.T) {
	origFix := autoRemediateFixBuildFailureFunc
	origVerify := autoRemediateVerifyBuildFunc
	defer func() {
		autoRemediateFixBuildFailureFunc = origFix
		autoRemediateVerifyBuildFunc = origVerify
	}()

	autoRemediateFixBuildFailureFunc = func(ctx context.Context, opts FixBuildOpts) error {
		return nil
	}
	verifyErr := errors.New("cannot read manifest: file not found")
	autoRemediateVerifyBuildFunc = func(implPath, repoPath string) (*protocol.VerifyBuildData, error) {
		return nil, verifyErr
	}

	failedResult := &FinalizeWaveResult{
		Wave:          1,
		BuildPassed:   false,
		VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: false}},
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}

	_, err := AutoRemediate(context.Background(), AutoRemediateOpts{
		IMPLPath:     "/tmp/fake/IMPL.yaml",
		RepoPath:     "/tmp/fake/repo",
		WaveNum:      1,
		FailedResult: failedResult,
		MaxRetries:   3,
	})

	if err == nil {
		t.Fatal("expected error when VerifyBuild returns system error")
	}
	if !errors.Is(err, verifyErr) {
		t.Errorf("expected wrapped verifyErr, got: %v", err)
	}
}

// TestAutoRemediate_FixAgentErrorContinues verifies that when FixBuildFailure
// returns an error, the loop still continues and re-verifies.
func TestAutoRemediate_FixAgentErrorContinues(t *testing.T) {
	origFix := autoRemediateFixBuildFailureFunc
	origVerify := autoRemediateVerifyBuildFunc
	defer func() {
		autoRemediateFixBuildFailureFunc = origFix
		autoRemediateVerifyBuildFunc = origVerify
	}()

	fixCallCount := 0
	autoRemediateFixBuildFailureFunc = func(ctx context.Context, opts FixBuildOpts) error {
		fixCallCount++
		if fixCallCount == 1 {
			return errors.New("fix agent crashed")
		}
		return nil
	}

	verifyCallCount := 0
	autoRemediateVerifyBuildFunc = func(implPath, repoPath string) (*protocol.VerifyBuildData, error) {
		verifyCallCount++
		if verifyCallCount >= 2 {
			return &protocol.VerifyBuildData{TestPassed: true, LintPassed: true}, nil
		}
		return &protocol.VerifyBuildData{TestPassed: false}, nil
	}

	failedResult := &FinalizeWaveResult{
		Wave:          1,
		BuildPassed:   false,
		VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: false}},
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}

	result, err := AutoRemediate(context.Background(), AutoRemediateOpts{
		IMPLPath:     "/tmp/fake/IMPL.yaml",
		RepoPath:     "/tmp/fake/repo",
		WaveNum:      1,
		FailedResult: failedResult,
		MaxRetries:   3,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Even though fix attempt 1 failed, verify attempt 2 should succeed
	if !result.Fixed {
		t.Error("expected Fixed=true when second attempt succeeds")
	}
	if result.Attempts != 2 {
		t.Errorf("expected Attempts=2, got %d", result.Attempts)
	}
}

// TestAutoRemediate_GateTypeDetection verifies determineFailedGateType returns
// correct gate types based on the failed result.
func TestAutoRemediate_GateTypeDetection(t *testing.T) {
	tests := []struct {
		name     string
		result   *FinalizeWaveResult
		expected string
	}{
		{
			name: "test fails first",
			result: &FinalizeWaveResult{
				VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: false, LintPassed: false}},
				VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
				GateResults:   make(map[string][]protocol.GateResult),
				MergeResult:   make(map[string]*protocol.MergeAgentsData),
				CleanupResult: make(map[string]*protocol.CleanupData),
			},
			expected: "test",
		},
		{
			name: "only lint fails",
			result: &FinalizeWaveResult{
				VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: true, LintPassed: false}},
				VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
				GateResults:   make(map[string][]protocol.GateResult),
				MergeResult:   make(map[string]*protocol.MergeAgentsData),
				CleanupResult: make(map[string]*protocol.CleanupData),
			},
			expected: "lint",
		},
		{
			name: "gate result fails",
			result: &FinalizeWaveResult{
				VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestPassed: true, LintPassed: true}},
				GateResults:   map[string][]protocol.GateResult{".": {{Type: "typecheck", Required: true, Passed: false}}},
				VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
				MergeResult:   make(map[string]*protocol.MergeAgentsData),
				CleanupResult: make(map[string]*protocol.CleanupData),
			},
			expected: "typecheck",
		},
		{
			name:     "nil result",
			result:   nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineFailedGateType(tt.result)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestAutoRemediate_ExtractErrorLog verifies that extractErrorLog builds the
// combined log correctly from test and lint outputs.
func TestAutoRemediate_ExtractErrorLog(t *testing.T) {
	result := &FinalizeWaveResult{
		VerifyBuild:   map[string]*protocol.VerifyBuildData{".": {TestOutput: "FAIL: test error", LintOutput: "lint: some warning"}},
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}

	log := extractErrorLog(result)
	if !strings.Contains(log, "test error") {
		t.Error("expected test output in error log")
	}
	if !strings.Contains(log, "lint: some warning") {
		t.Error("expected lint output in error log")
	}

	// Empty VerifyBuild map returns empty string
	emptyResult := &FinalizeWaveResult{
		VerifyBuild:   make(map[string]*protocol.VerifyBuildData),
		VerifyCommits: make(map[string]*protocol.VerifyCommitsData),
		GateResults:   make(map[string][]protocol.GateResult),
		MergeResult:   make(map[string]*protocol.MergeAgentsData),
		CleanupResult: make(map[string]*protocol.CleanupData),
	}
	if extractErrorLog(emptyResult) != "" {
		t.Error("expected empty log for empty VerifyBuild map")
	}

	// Nil result returns empty string
	if extractErrorLog(nil) != "" {
		t.Error("expected empty log for nil result")
	}
}
