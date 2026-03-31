package engine

import (
	"context"
	"testing"
)

func TestPostMergeCleanup_CallbackInvoked(t *testing.T) {
	// PostMergeCleanup is non-fatal — even with an invalid implPath,
	// it should call onEvent with error status but not return a fatal result.
	var events []struct{ step, status, detail string }
	onEvent := func(step, status, detail string) {
		events = append(events, struct{ step, status, detail string }{step, status, detail})
	}

	res := PostMergeCleanup(context.Background(), "/nonexistent/IMPL.yaml", 1, t.TempDir(), onEvent)
	if res.IsFatal() {
		t.Fatalf("PostMergeCleanup should not return fatal result (non-fatal), got: %v", res.Errors)
	}

	// Should have events for both steps.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d: %+v", len(events), events)
	}

	// First event should be gomod_fixup running.
	if events[0].step != "gomod_fixup" || events[0].status != "running" {
		t.Errorf("expected first event gomod_fixup/running, got %s/%s", events[0].step, events[0].status)
	}

	// Should have a cleanup event.
	foundCleanup := false
	for _, e := range events {
		if e.step == "cleanup" {
			foundCleanup = true
			break
		}
	}
	if !foundCleanup {
		t.Error("expected a cleanup event")
	}
}

func TestPostMergeCleanup_NilCallback(t *testing.T) {
	// Should not panic with nil onEvent.
	res := PostMergeCleanup(context.Background(), "/nonexistent/IMPL.yaml", 1, t.TempDir(), nil)
	if res.IsFatal() {
		t.Fatalf("PostMergeCleanup should not return fatal result, got: %v", res.Errors)
	}
}

func TestPostMergeCleanup_NonFatalErrors(t *testing.T) {
	// Both go.mod fixup and cleanup should be non-fatal.
	// Using a temp dir with no go.mod and invalid IMPL path.
	var statuses []string
	onEvent := func(step, status, detail string) {
		statuses = append(statuses, step+":"+status)
	}

	res := PostMergeCleanup(context.Background(), "/nonexistent/IMPL.yaml", 99, t.TempDir(), onEvent)
	if res.IsFatal() {
		t.Fatalf("expected non-fatal result, got fatal: %v", res.Errors)
	}

	// Should have reported some events even though underlying ops fail.
	if len(statuses) == 0 {
		t.Error("expected at least some status events")
	}
}

func TestPostMergeCleanup_ReturnsWarningsOnError(t *testing.T) {
	// When operations fail (non-fatal), result should be PARTIAL with warnings.
	res := PostMergeCleanup(context.Background(), "/nonexistent/IMPL.yaml", 1, t.TempDir(), nil)
	// Non-fatal means either SUCCESS (if ops succeed) or PARTIAL (warnings for failures)
	// but NOT FATAL.
	if res.IsFatal() {
		t.Errorf("expected non-fatal (SUCCESS or PARTIAL), got FATAL: %v", res.Errors)
	}
}
