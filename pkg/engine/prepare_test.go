package engine

import (
	"context"
	"testing"
)

func TestPrepareWave_MissingIMPLPath(t *testing.T) {
	res, err := PrepareWave(context.Background(), PrepareWaveOpts{
		RepoPath: "/tmp/fake-repo",
		WaveNum:  1,
	})
	if err == nil {
		t.Fatal("expected error for missing IMPLPath")
	}
	if res == nil {
		t.Fatal("expected partial result even on error")
	}
	if res.Success {
		t.Error("result should not be Success on error")
	}
	if err.Error() != "IMPLPath is required" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestPrepareWave_MissingRepoPath(t *testing.T) {
	res, err := PrepareWave(context.Background(), PrepareWaveOpts{
		IMPLPath: "/tmp/fake.yaml",
		WaveNum:  1,
	})
	if err == nil {
		t.Fatal("expected error for missing RepoPath")
	}
	if res == nil {
		t.Fatal("expected partial result even on error")
	}
	if err.Error() != "RepoPath is required" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestPrepareWave_MissingWaveNum(t *testing.T) {
	res, err := PrepareWave(context.Background(), PrepareWaveOpts{
		IMPLPath: "/tmp/fake.yaml",
		RepoPath: "/tmp/fake-repo",
	})
	if err == nil {
		t.Fatal("expected error for missing WaveNum")
	}
	if res == nil {
		t.Fatal("expected partial result even on error")
	}
	if err.Error() != "WaveNum is required (must be >= 1)" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestPrepareWave_OnEventCallback(t *testing.T) {
	// Use a non-existent IMPL path — we expect it to fail at dep_check or load_manifest,
	// but the callback should still be invoked for each step that runs.
	var events []struct {
		Step   string
		Status string
		Detail string
	}

	cb := func(step, status, detail string) {
		events = append(events, struct {
			Step   string
			Status string
			Detail string
		}{step, status, detail})
	}

	res, err := PrepareWave(context.Background(), PrepareWaveOpts{
		IMPLPath: "/tmp/nonexistent-impl.yaml",
		RepoPath: "/tmp/nonexistent-repo",
		WaveNum:  1,
		OnEvent:  cb,
	})

	// We expect an error (dep check or manifest load will fail)
	if err == nil {
		t.Fatal("expected error for nonexistent paths")
	}

	// Verify callback was invoked
	if len(events) == 0 {
		t.Fatal("expected OnEvent callback to be invoked at least once")
	}

	// The result should be partial with steps recorded
	if res == nil {
		t.Fatal("expected partial result")
	}
	if len(res.Steps) == 0 {
		t.Fatal("expected at least one step in partial result")
	}

	// Verify events match steps
	if len(events) != len(res.Steps) {
		t.Errorf("event count %d != step count %d", len(events), len(res.Steps))
	}

	// The last step should have status "failed"
	lastStep := res.Steps[len(res.Steps)-1]
	if lastStep.Status != "failed" {
		t.Errorf("expected last step status 'failed', got %q", lastStep.Status)
	}

	// Result should not be marked successful
	if res.Success {
		t.Error("result should not be Success when pipeline fails")
	}
}

func TestPrepareWave_DepConflictReturnsPartialResult(t *testing.T) {
	// Use a path where dep check will fail (nonexistent IMPL)
	res, err := PrepareWave(context.Background(), PrepareWaveOpts{
		IMPLPath: "/tmp/nonexistent-impl-for-dep-check.yaml",
		RepoPath: "/tmp/nonexistent-repo",
		WaveNum:  1,
	})

	if err == nil {
		t.Fatal("expected error")
	}

	if res == nil {
		t.Fatal("expected partial result even on failure")
	}

	// Should have at least resume_detection step before failure
	if len(res.Steps) == 0 {
		t.Fatal("expected at least one step in partial result")
	}

	// First step should be resume_detection (succeeds even with bad paths)
	if res.Steps[0].Step != "resume_detection" {
		t.Errorf("expected first step to be resume_detection, got %q", res.Steps[0].Step)
	}

	if res.Success {
		t.Error("partial result should not have Success=true")
	}

	// Wave number should be preserved in result
	if res.Wave != 1 {
		t.Errorf("expected wave=1, got %d", res.Wave)
	}
}

func TestPrepareWave_NilOnEvent(t *testing.T) {
	// Verify that nil OnEvent doesn't panic
	res, err := PrepareWave(context.Background(), PrepareWaveOpts{
		IMPLPath: "/tmp/nonexistent.yaml",
		RepoPath: "/tmp/nonexistent-repo",
		WaveNum:  1,
		OnEvent:  nil, // explicitly nil
	})

	// Should fail but not panic
	if err == nil {
		t.Fatal("expected error")
	}
	if res == nil {
		t.Fatal("expected partial result")
	}
}
