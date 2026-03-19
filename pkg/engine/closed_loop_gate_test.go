package engine

import (
	"context"
	"errors"
	"testing"
)

// TestClosedLoopGateRetry_FixesOnFirstAttempt verifies that when the fix agent
// succeeds and the gate passes on the first re-run, Fixed=true and Attempts=1.
func TestClosedLoopGateRetry_FixesOnFirstAttempt(t *testing.T) {
	origAgent := closedLoopRunAgentFunc
	origExec := closedLoopExecCommandFunc
	defer func() {
		closedLoopRunAgentFunc = origAgent
		closedLoopExecCommandFunc = origExec
	}()

	// Fix agent always succeeds
	closedLoopRunAgentFunc = func(ctx context.Context, model, prompt, worktreePath string) error {
		return nil
	}

	// Gate passes on first re-run
	closedLoopExecCommandFunc = func(ctx context.Context, dir, cmdStr string) (string, error) {
		return "ok", nil
	}

	opts := ClosedLoopRetryOpts{
		AgentID:      "A",
		GateType:     "build",
		GateCommand:  "go build ./...",
		GateOutput:   "undefined: Foo",
		WorktreePath: t.TempDir(),
		MaxRetries:   3,
	}

	result, err := ClosedLoopGateRetry(context.Background(), opts)
	if err != nil {
		t.Fatalf("ClosedLoopGateRetry returned error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if !result.Fixed {
		t.Error("expected Fixed=true, got false")
	}
	if result.Attempts != 1 {
		t.Errorf("expected Attempts=1, got %d", result.Attempts)
	}
	if result.AgentID != "A" {
		t.Errorf("expected AgentID=A, got %q", result.AgentID)
	}
}

// TestClosedLoopGateRetry_ExhaustsRetries verifies that when the gate never
// passes, Fixed=false and Attempts equals MaxRetries.
func TestClosedLoopGateRetry_ExhaustsRetries(t *testing.T) {
	origAgent := closedLoopRunAgentFunc
	origExec := closedLoopExecCommandFunc
	defer func() {
		closedLoopRunAgentFunc = origAgent
		closedLoopExecCommandFunc = origExec
	}()

	agentCallCount := 0
	closedLoopRunAgentFunc = func(ctx context.Context, model, prompt, worktreePath string) error {
		agentCallCount++
		return nil
	}

	execCallCount := 0
	closedLoopExecCommandFunc = func(ctx context.Context, dir, cmdStr string) (string, error) {
		execCallCount++
		return "still failing", errors.New("build failed")
	}

	maxRetries := 3
	opts := ClosedLoopRetryOpts{
		AgentID:      "B",
		GateType:     "test",
		GateCommand:  "go test ./...",
		GateOutput:   "FAIL pkg/foo",
		WorktreePath: t.TempDir(),
		MaxRetries:   maxRetries,
	}

	result, err := ClosedLoopGateRetry(context.Background(), opts)
	if err != nil {
		t.Fatalf("ClosedLoopGateRetry returned error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Fixed {
		t.Error("expected Fixed=false, got true")
	}
	if result.Attempts != maxRetries {
		t.Errorf("expected Attempts=%d, got %d", maxRetries, result.Attempts)
	}
	if agentCallCount != maxRetries {
		t.Errorf("expected fix agent called %d times, got %d", maxRetries, agentCallCount)
	}
	if execCallCount != maxRetries {
		t.Errorf("expected exec called %d times, got %d", maxRetries, execCallCount)
	}
}

// TestClosedLoopGateRetry_EmitsEvents verifies that the four lifecycle events
// are emitted in the expected sequence.
func TestClosedLoopGateRetry_EmitsEvents(t *testing.T) {
	origAgent := closedLoopRunAgentFunc
	origExec := closedLoopExecCommandFunc
	defer func() {
		closedLoopRunAgentFunc = origAgent
		closedLoopExecCommandFunc = origExec
	}()

	closedLoopRunAgentFunc = func(ctx context.Context, model, prompt, worktreePath string) error {
		return nil
	}

	callNum := 0
	closedLoopExecCommandFunc = func(ctx context.Context, dir, cmdStr string) (string, error) {
		callNum++
		if callNum == 1 {
			return "still failing", errors.New("fail")
		}
		return "ok", nil
	}

	var events []string
	opts := ClosedLoopRetryOpts{
		AgentID:      "C",
		GateType:     "lint",
		GateCommand:  "go vet ./...",
		GateOutput:   "vet: found issues",
		WorktreePath: t.TempDir(),
		MaxRetries:   3,
		OnEvent: func(ev Event) {
			events = append(events, ev.Event)
		},
	}

	result, err := ClosedLoopGateRetry(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Fixed {
		t.Error("expected Fixed=true")
	}

	// Verify event sequence: started, attempt, attempt, fixed
	want := []string{
		"closed_loop_started",
		"closed_loop_attempt",
		"closed_loop_attempt",
		"closed_loop_fixed",
	}
	if len(events) != len(want) {
		t.Fatalf("expected %d events, got %d: %v", len(want), len(events), events)
	}
	for i, ev := range want {
		if events[i] != ev {
			t.Errorf("event[%d]: want %q, got %q", i, ev, events[i])
		}
	}
}

// TestClosedLoopGateRetry_RunsInWorktree verifies that commands run with
// the agent's worktree path as their working directory.
func TestClosedLoopGateRetry_RunsInWorktree(t *testing.T) {
	origAgent := closedLoopRunAgentFunc
	origExec := closedLoopExecCommandFunc
	defer func() {
		closedLoopRunAgentFunc = origAgent
		closedLoopExecCommandFunc = origExec
	}()

	expectedWtPath := t.TempDir()

	var observedAgentPath string
	closedLoopRunAgentFunc = func(ctx context.Context, model, prompt, worktreePath string) error {
		observedAgentPath = worktreePath
		return nil
	}

	var observedExecDir string
	closedLoopExecCommandFunc = func(ctx context.Context, dir, cmdStr string) (string, error) {
		observedExecDir = dir
		return "ok", nil
	}

	opts := ClosedLoopRetryOpts{
		AgentID:      "D",
		GateType:     "build",
		GateCommand:  "go build ./...",
		GateOutput:   "build failed",
		WorktreePath: expectedWtPath,
		MaxRetries:   1,
	}

	result, err := ClosedLoopGateRetry(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Fixed {
		t.Error("expected Fixed=true")
	}

	// Verify fix agent ran in the worktree
	if observedAgentPath != expectedWtPath {
		t.Errorf("fix agent ran in %q, want %q", observedAgentPath, expectedWtPath)
	}
	// Verify exec ran in the worktree
	if observedExecDir != expectedWtPath {
		t.Errorf("gate exec ran in %q, want %q", observedExecDir, expectedWtPath)
	}
}

// TestClosedLoopGateRetry_ValidationErrors verifies validation errors for missing required fields.
func TestClosedLoopGateRetry_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		opts    ClosedLoopRetryOpts
		wantErr string
	}{
		{
			name:    "missing AgentID",
			opts:    ClosedLoopRetryOpts{GateCommand: "go build ./...", WorktreePath: "/tmp"},
			wantErr: "AgentID is required",
		},
		{
			name:    "missing GateCommand",
			opts:    ClosedLoopRetryOpts{AgentID: "A", WorktreePath: "/tmp"},
			wantErr: "GateCommand is required",
		},
		{
			name:    "missing WorktreePath",
			opts:    ClosedLoopRetryOpts{AgentID: "A", GateCommand: "go build ./..."},
			wantErr: "WorktreePath is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ClosedLoopGateRetry(ctx, tc.opts)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsSubstr(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

// TestClosedLoopGateRetry_ExhaustsEmitsEvent verifies that when retries are
// exhausted, a closed_loop_exhausted event is emitted.
func TestClosedLoopGateRetry_ExhaustsEmitsEvent(t *testing.T) {
	origAgent := closedLoopRunAgentFunc
	origExec := closedLoopExecCommandFunc
	defer func() {
		closedLoopRunAgentFunc = origAgent
		closedLoopExecCommandFunc = origExec
	}()

	closedLoopRunAgentFunc = func(ctx context.Context, model, prompt, worktreePath string) error {
		return nil
	}
	closedLoopExecCommandFunc = func(ctx context.Context, dir, cmdStr string) (string, error) {
		return "fail", errors.New("still broken")
	}

	var events []string
	opts := ClosedLoopRetryOpts{
		AgentID:      "E",
		GateType:     "build",
		GateCommand:  "go build ./...",
		GateOutput:   "error",
		WorktreePath: t.TempDir(),
		MaxRetries:   2,
		OnEvent: func(ev Event) {
			events = append(events, ev.Event)
		},
	}

	result, err := ClosedLoopGateRetry(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fixed {
		t.Error("expected Fixed=false")
	}

	// Last event should be closed_loop_exhausted
	if len(events) == 0 {
		t.Fatal("no events emitted")
	}
	last := events[len(events)-1]
	if last != "closed_loop_exhausted" {
		t.Errorf("last event: want closed_loop_exhausted, got %q", last)
	}
}

// containsSubstr is a local helper to check substring presence.
func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
