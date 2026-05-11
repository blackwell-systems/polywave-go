package engine

import (
	"context"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/autonomy"
	"github.com/blackwell-systems/polywave-go/pkg/queue"
)

// seedQueue is a helper that adds queue items to a temp repo dir and returns
// a configured queue.Manager for assertions.
func seedQueue(t *testing.T, repoPath string, items []queue.Item) *queue.Manager {
	t.Helper()
	mgr := queue.NewManager(repoPath)
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("seedQueue: Add(%q): %s", item.Slug, res.Errors[0].Message)
		}
	}
	return mgr
}

// TestCheckQueue_NoItems ensures that an empty queue returns Advanced=false, Reason="no_items".
func TestCheckQueue_NoItems(t *testing.T) {
	dir := t.TempDir()
	// Don't add any items.

	cfg := autonomy.DefaultConfig()
	result, err := CheckQueue(context.Background(), CheckQueueOpts{
		RepoPath:       dir,
		AutonomyConfig: cfg,
	})
	if err != nil {
		t.Fatalf("CheckQueue: %v", err)
	}
	if result.Advanced {
		t.Error("expected Advanced=false for empty queue")
	}
	if result.Reason != "no_items" {
		t.Errorf("Reason = %q, want %q", result.Reason, "no_items")
	}
}

// TestCheckQueue_AutonomyBlocked ensures that a gated autonomy level blocks advance.
func TestCheckQueue_AutonomyBlocked(t *testing.T) {
	dir := t.TempDir()
	seedQueue(t, dir, []queue.Item{
		{Title: "Feature A", Priority: 1, Slug: "feature-a"},
	})

	cfg := autonomy.Config{Level: autonomy.LevelGated, MaxAutoRetries: 2, MaxQueueDepth: 10}
	result, err := CheckQueue(context.Background(), CheckQueueOpts{
		RepoPath:       dir,
		AutonomyConfig: cfg,
	})
	if err != nil {
		t.Fatalf("CheckQueue: %v", err)
	}
	if result.Advanced {
		t.Error("expected Advanced=false for gated level")
	}
	if result.Reason != "autonomy_blocked" {
		t.Errorf("Reason = %q, want %q", result.Reason, "autonomy_blocked")
	}
	if result.NextSlug != "feature-a" {
		t.Errorf("NextSlug = %q, want %q", result.NextSlug, "feature-a")
	}
}

// TestCheckQueue_Advances ensures that a supervised autonomy level triggers advance.
func TestCheckQueue_Advances(t *testing.T) {
	dir := t.TempDir()
	mgr := seedQueue(t, dir, []queue.Item{
		{Title: "Feature B", Priority: 1, Slug: "feature-b"},
	})

	cfg := autonomy.Config{Level: autonomy.LevelSupervised, MaxAutoRetries: 2, MaxQueueDepth: 10}
	result, err := CheckQueue(context.Background(), CheckQueueOpts{
		RepoPath:       dir,
		AutonomyConfig: cfg,
	})
	if err != nil {
		t.Fatalf("CheckQueue: %v", err)
	}
	if !result.Advanced {
		t.Errorf("expected Advanced=true for supervised level, Reason=%q", result.Reason)
	}
	if result.Reason != "triggered" {
		t.Errorf("Reason = %q, want %q", result.Reason, "triggered")
	}
	if result.NextSlug != "feature-b" {
		t.Errorf("NextSlug = %q, want %q", result.NextSlug, "feature-b")
	}
	if result.NextTitle != "Feature B" {
		t.Errorf("NextTitle = %q, want %q", result.NextTitle, "Feature B")
	}

	// Verify the status was updated to in_progress.
	listRes := mgr.List()
	if listRes.IsFatal() {
		t.Fatalf("List: %s", listRes.Errors[0].Message)
	}
	items := listRes.GetData().Items
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Status != "in_progress" {
		t.Errorf("item status = %q, want %q", items[0].Status, "in_progress")
	}
}

// TestCheckQueue_RespectsOverride ensures that a per-item gated override blocks
// advance even when the global level is supervised.
func TestCheckQueue_RespectsOverride(t *testing.T) {
	dir := t.TempDir()
	seedQueue(t, dir, []queue.Item{
		{
			Title:            "Feature C",
			Priority:         1,
			Slug:             "feature-c",
			AutonomyOverride: string(autonomy.LevelGated),
		},
	})

	// Global level is supervised — but item override is gated.
	cfg := autonomy.Config{Level: autonomy.LevelSupervised, MaxAutoRetries: 2, MaxQueueDepth: 10}
	result, err := CheckQueue(context.Background(), CheckQueueOpts{
		RepoPath:       dir,
		AutonomyConfig: cfg,
	})
	if err != nil {
		t.Fatalf("CheckQueue: %v", err)
	}
	if result.Advanced {
		t.Error("expected Advanced=false when item override is gated")
	}
	if result.Reason != "autonomy_blocked" {
		t.Errorf("Reason = %q, want %q", result.Reason, "autonomy_blocked")
	}
	if result.NextSlug != "feature-c" {
		t.Errorf("NextSlug = %q, want %q", result.NextSlug, "feature-c")
	}
}

// TestCheckQueue_EmitsEvent ensures that a queue_advanced event is fired when
// the queue advances.
func TestCheckQueue_EmitsEvent(t *testing.T) {
	dir := t.TempDir()
	seedQueue(t, dir, []queue.Item{
		{Title: "Feature D", Priority: 1, Slug: "feature-d"},
	})

	cfg := autonomy.Config{Level: autonomy.LevelAutonomous, MaxAutoRetries: 2, MaxQueueDepth: 10}

	var events []Event
	result, err := CheckQueue(context.Background(), CheckQueueOpts{
		RepoPath:       dir,
		AutonomyConfig: cfg,
		OnEvent: func(ev Event) {
			events = append(events, ev)
		},
	})
	if err != nil {
		t.Fatalf("CheckQueue: %v", err)
	}
	if !result.Advanced {
		t.Errorf("expected Advanced=true for autonomous level, Reason=%q", result.Reason)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != "queue_advanced" {
		t.Errorf("event name = %q, want %q", events[0].Event, "queue_advanced")
	}
	// Verify the payload is the result itself.
	payload, ok := events[0].Data.(*CheckQueueResult)
	if !ok {
		t.Fatalf("event Data type = %T, want *CheckQueueResult", events[0].Data)
	}
	if payload.NextSlug != "feature-d" {
		t.Errorf("event payload NextSlug = %q, want %q", payload.NextSlug, "feature-d")
	}
}
