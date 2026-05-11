package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	obs "github.com/blackwell-systems/polywave-go/pkg/observability"
)

func tempDB(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.db")
}

func mustOpen(t *testing.T) obs.Store {
	t.Helper()
	s, err := Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpen(t *testing.T) {
	path := tempDB(t)
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// File should exist.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file not created")
	}
}

func TestOpen_NonexistentParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "nested")
	path := filepath.Join(dir, "test.db")

	// modernc.org/sqlite creates the file; the parent directory
	// must exist. Open should fail or work depending on driver behavior.
	_, err := Open(path)
	if err == nil {
		t.Log("Open succeeded with non-existent parent (driver created it)")
	} else {
		t.Logf("Open correctly failed with non-existent parent: %v", err)
	}
}

func TestRecordAndQueryCostEvent(t *testing.T) {
	s := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	ev := &obs.CostEvent{
		ID:           obs.NewEventID(),
		Type:         "cost",
		Time:         now,
		AgentID:      "A",
		WaveNumber:   1,
		IMPLSlug:     "my-impl",
		ProgramSlug:  "my-program",
		Model:        "claude-sonnet-4-20250514",
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0.0123,
	}

	if err := s.RecordEvent(ctx, ev); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	events, err := s.QueryEvents(ctx, obs.QueryFilters{EventTypes: []string{"cost"}})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ce, ok := events[0].(*obs.CostEvent)
	if !ok {
		t.Fatalf("expected *CostEvent, got %T", events[0])
	}
	if ce.CostUSD != 0.0123 {
		t.Errorf("CostUSD = %f, want 0.0123", ce.CostUSD)
	}
	if ce.AgentID != "A" {
		t.Errorf("AgentID = %q, want %q", ce.AgentID, "A")
	}
	if ce.IMPLSlug != "my-impl" {
		t.Errorf("IMPLSlug = %q, want %q", ce.IMPLSlug, "my-impl")
	}
}

func TestRecordAndQueryAgentPerformanceEvent(t *testing.T) {
	s := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	ev := &obs.AgentPerformanceEvent{
		ID:              obs.NewEventID(),
		Type:            "agent_performance",
		Time:            now,
		AgentID:         "B",
		WaveNumber:      2,
		IMPLSlug:        "perf-impl",
		Status:          "success",
		RetryCount:      1,
		DurationSeconds: 120,
		TestsPassed:     5,
	}

	if err := s.RecordEvent(ctx, ev); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	events, err := s.QueryEvents(ctx, obs.QueryFilters{EventTypes: []string{"agent_performance"}})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	pe, ok := events[0].(*obs.AgentPerformanceEvent)
	if !ok {
		t.Fatalf("expected *AgentPerformanceEvent, got %T", events[0])
	}
	if pe.Status != "success" {
		t.Errorf("Status = %q, want %q", pe.Status, "success")
	}
	if pe.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", pe.RetryCount)
	}
}

func TestRecordAndQueryActivityEvent(t *testing.T) {
	s := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	ev := &obs.ActivityEvent{
		ID:           obs.NewEventID(),
		Type:         "activity",
		Time:         now,
		ActivityType: "wave_start",
		IMPLSlug:     "act-impl",
		WaveNumber:   1,
		Details:      "starting wave 1",
	}

	if err := s.RecordEvent(ctx, ev); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	events, err := s.QueryEvents(ctx, obs.QueryFilters{EventTypes: []string{"activity"}})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ae, ok := events[0].(*obs.ActivityEvent)
	if !ok {
		t.Fatalf("expected *ActivityEvent, got %T", events[0])
	}
	if ae.ActivityType != "wave_start" {
		t.Errorf("ActivityType = %q, want %q", ae.ActivityType, "wave_start")
	}
}

func TestQueryFilters_ByIMPLSlug(t *testing.T) {
	s := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, slug := range []string{"impl-a", "impl-b", "impl-a"} {
		ev := &obs.CostEvent{
			ID:       obs.NewEventID(),
			Type:     "cost",
			Time:     now,
			IMPLSlug: slug,
			CostUSD:  1.0,
		}
		if err := s.RecordEvent(ctx, ev); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}

	events, err := s.QueryEvents(ctx, obs.QueryFilters{
		EventTypes: []string{"cost"},
		IMPLSlugs:  []string{"impl-a"},
	})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for impl-a, got %d", len(events))
	}
}

func TestQueryFilters_ByTimeRange(t *testing.T) {
	s := mustOpen(t)
	ctx := context.Background()

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		ev := &obs.CostEvent{
			ID:      obs.NewEventID(),
			Type:    "cost",
			Time:    base.Add(time.Duration(i) * time.Hour),
			CostUSD: float64(i),
		}
		if err := s.RecordEvent(ctx, ev); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}

	start := base.Add(1 * time.Hour)
	end := base.Add(3 * time.Hour)
	events, err := s.QueryEvents(ctx, obs.QueryFilters{
		EventTypes: []string{"cost"},
		StartTime:  &start,
		EndTime:    &end,
	})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events in time range, got %d", len(events))
	}
}

func TestQueryFilters_Limit(t *testing.T) {
	s := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for i := 0; i < 10; i++ {
		ev := &obs.CostEvent{
			ID:      obs.NewEventID(),
			Type:    "cost",
			Time:    now.Add(time.Duration(i) * time.Second),
			CostUSD: float64(i),
		}
		if err := s.RecordEvent(ctx, ev); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}

	events, err := s.QueryEvents(ctx, obs.QueryFilters{
		EventTypes: []string{"cost"},
		Limit:      3,
	})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events with limit, got %d", len(events))
	}
}

func TestGetRollup_Cost(t *testing.T) {
	s := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC()

	events := []obs.Event{
		&obs.CostEvent{ID: obs.NewEventID(), Type: "cost", Time: now, AgentID: "A", IMPLSlug: "x", CostUSD: 1.5},
		&obs.CostEvent{ID: obs.NewEventID(), Type: "cost", Time: now, AgentID: "B", IMPLSlug: "x", CostUSD: 2.5},
	}
	for _, ev := range events {
		if err := s.RecordEvent(ctx, ev); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}

	result, err := s.GetRollup(ctx, obs.RollupRequest{
		Type:     "cost",
		GroupBy:  []string{"agent"},
		IMPLSlug: "x",
	})
	if err != nil {
		t.Fatalf("GetRollup: %v", err)
	}
	if result.TotalCost != 4.0 {
		t.Errorf("TotalCost = %f, want 4.0", result.TotalCost)
	}
	if len(result.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(result.Groups))
	}
}

func TestClose_Idempotent(t *testing.T) {
	path := tempDB(t)
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Second close should not panic. It may return an error from the driver
	// but should not crash.
	_ = s.Close()
}

func TestQueryFilters_ByAgentID(t *testing.T) {
	s := mustOpen(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, agent := range []string{"A", "B", "A"} {
		ev := &obs.AgentPerformanceEvent{
			ID:      obs.NewEventID(),
			Type:    "agent_performance",
			Time:    now,
			AgentID: agent,
			Status:  "success",
		}
		if err := s.RecordEvent(ctx, ev); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}

	events, err := s.QueryEvents(ctx, obs.QueryFilters{
		EventTypes: []string{"agent_performance"},
		AgentIDs:   []string{"A"},
	})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for agent A, got %d", len(events))
	}
}
