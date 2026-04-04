package observability

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockStore implements Store for testing query functions.
type mockStore struct {
	events []Event
}

func (m *mockStore) RecordEvent(_ context.Context, event Event) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockStore) QueryEvents(_ context.Context, filters QueryFilters) ([]Event, error) {
	var result []Event
	for _, e := range m.events {
		if !matchesFilters(e, filters) {
			continue
		}
		result = append(result, e)
		if filters.Limit > 0 && len(result) >= filters.Limit {
			break
		}
	}
	return result, nil
}

func (m *mockStore) GetRollup(_ context.Context, _ RollupRequest) (*RollupResult, error) {
	return &RollupResult{}, nil
}

func (m *mockStore) Close() error { return nil }

func matchesFilters(e Event, f QueryFilters) bool {
	if len(f.EventTypes) > 0 && !contains(f.EventTypes, e.EventType()) {
		return false
	}
	if len(f.AgentIDs) > 0 {
		agentID := eventAgentID(e)
		if agentID == "" || !contains(f.AgentIDs, agentID) {
			return false
		}
	}
	if len(f.IMPLSlugs) > 0 {
		slug := eventIMPLSlug(e)
		if slug == "" || !contains(f.IMPLSlugs, slug) {
			return false
		}
	}
	if len(f.ProgramSlugs) > 0 {
		slug := eventProgramSlug(e)
		if slug == "" || !contains(f.ProgramSlugs, slug) {
			return false
		}
	}
	if f.StartTime != nil && e.Timestamp().Before(*f.StartTime) {
		return false
	}
	if f.EndTime != nil && e.Timestamp().After(*f.EndTime) {
		return false
	}
	return true
}

func eventAgentID(e Event) string {
	switch v := e.(type) {
	case *CostEvent:
		return v.AgentID
	case *AgentPerformanceEvent:
		return v.AgentID
	default:
		return ""
	}
}

func eventIMPLSlug(e Event) string {
	switch v := e.(type) {
	case *CostEvent:
		return v.IMPLSlug
	case *AgentPerformanceEvent:
		return v.IMPLSlug
	case *ActivityEvent:
		return v.IMPLSlug
	default:
		return ""
	}
}

func eventProgramSlug(e Event) string {
	switch v := e.(type) {
	case *CostEvent:
		return v.ProgramSlug
	case *AgentPerformanceEvent:
		return v.ProgramSlug
	case *ActivityEvent:
		return v.ProgramSlug
	default:
		return ""
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func TestQueryGetAgentHistory(t *testing.T) {
	store := &mockStore{}
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Insert 5 performance events for agent A with varying timestamps.
	for i := range 5 {
		_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
			ID:              fmt.Sprintf("perf-%d", i),
			Type:            "agent_performance",
			Time:            base.Add(time.Duration(i) * time.Hour),
			AgentID:         "A",
			WaveNumber:      1,
			IMPLSlug:        "test-impl",
			Status:          "success",
			DurationSeconds: 60,
		})
	}
	// Insert an event for agent B (should not appear).
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID:      "perf-other",
		Type:    "agent_performance",
		Time:    base,
		AgentID: "B",
		Status:  "success",
	})

	histRes := GetAgentHistory(ctx, store, "A", 10)
	if histRes.IsFatal() {
		t.Fatalf("GetAgentHistory returned error: %s", histRes.Errors[0].Message)
	}
	result := histRes.GetData().Events
	if len(result) != 5 {
		t.Fatalf("expected 5 events, got %d", len(result))
	}
	// Verify descending order.
	for i := 1; i < len(result); i++ {
		if result[i].Time.After(result[i-1].Time) {
			t.Errorf("events not in descending order at index %d", i)
		}
	}
}

func TestQueryGetIMPLMetrics(t *testing.T) {
	store := &mockStore{}
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Cost events.
	_ = store.RecordEvent(ctx, &CostEvent{
		ID: "c1", Type: "cost", Time: base, AgentID: "A",
		IMPLSlug: "impl-1", CostUSD: 0.50,
	})
	_ = store.RecordEvent(ctx, &CostEvent{
		ID: "c2", Type: "cost", Time: base, AgentID: "B",
		IMPLSlug: "impl-1", CostUSD: 0.30,
	})

	// Performance events: 2 success, 1 failed.
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p1", Type: "agent_performance", Time: base, AgentID: "A",
		IMPLSlug: "impl-1", Status: "success", DurationSeconds: 120,
	})
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p2", Type: "agent_performance", Time: base, AgentID: "B",
		IMPLSlug: "impl-1", Status: "success", DurationSeconds: 180,
	})
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p3", Type: "agent_performance", Time: base, AgentID: "C",
		IMPLSlug: "impl-1", Status: "failed", FailureType: "transient", DurationSeconds: 60,
	})

	// Activity events: 2 wave merges.
	_ = store.RecordEvent(ctx, &ActivityEvent{
		ID: "a1", Type: "activity", Time: base,
		ActivityType: "wave_merge", IMPLSlug: "impl-1", WaveNumber: 1,
	})
	_ = store.RecordEvent(ctx, &ActivityEvent{
		ID: "a2", Type: "activity", Time: base,
		ActivityType: "wave_merge", IMPLSlug: "impl-1", WaveNumber: 2,
	})

	metricsRes := GetIMPLMetrics(ctx, store, "impl-1")
	if metricsRes.IsFatal() {
		t.Fatalf("GetIMPLMetrics returned error: %s", metricsRes.Errors[0].Message)
	}
	metrics := metricsRes.GetData()
	if metrics.TotalCost != 0.80 {
		t.Errorf("TotalCost = %f, want 0.80", metrics.TotalCost)
	}
	expectedDur := (120.0 + 180.0 + 60.0) / 60.0
	if metrics.TotalDurationMin != expectedDur {
		t.Errorf("TotalDurationMin = %f, want %f", metrics.TotalDurationMin, expectedDur)
	}
	expectedRate := 2.0 / 3.0
	if metrics.SuccessRate < expectedRate-0.01 || metrics.SuccessRate > expectedRate+0.01 {
		t.Errorf("SuccessRate = %f, want ~%f", metrics.SuccessRate, expectedRate)
	}
	if metrics.AgentsFailed != 1 {
		t.Errorf("AgentsFailed = %d, want 1", metrics.AgentsFailed)
	}
	if metrics.WavesCompleted != 2 {
		t.Errorf("WavesCompleted = %d, want 2", metrics.WavesCompleted)
	}
}

func TestQueryGetIMPLMetricsEmpty(t *testing.T) {
	store := &mockStore{}
	ctx := context.Background()

	metricsRes2 := GetIMPLMetrics(ctx, store, "nonexistent")
	if metricsRes2.IsFatal() {
		t.Fatalf("GetIMPLMetrics returned error: %s", metricsRes2.Errors[0].Message)
	}
	metrics := metricsRes2.GetData()
	if metrics.TotalCost != 0 || metrics.SuccessRate != 0 || metrics.AgentsFailed != 0 {
		t.Error("expected zero metrics for missing IMPL")
	}
}

func TestQueryGetProgramSummary(t *testing.T) {
	store := &mockStore{}
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Events across 2 IMPLs in the same program.
	_ = store.RecordEvent(ctx, &CostEvent{
		ID: "c1", Type: "cost", Time: base, AgentID: "A",
		IMPLSlug: "impl-1", ProgramSlug: "prog-1", CostUSD: 1.00,
	})
	_ = store.RecordEvent(ctx, &CostEvent{
		ID: "c2", Type: "cost", Time: base, AgentID: "A",
		IMPLSlug: "impl-2", ProgramSlug: "prog-1", CostUSD: 2.00,
	})

	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p1", Type: "agent_performance", Time: base, AgentID: "A",
		IMPLSlug: "impl-1", ProgramSlug: "prog-1", Status: "success", DurationSeconds: 300,
	})
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p2", Type: "agent_performance", Time: base, AgentID: "B",
		IMPLSlug: "impl-2", ProgramSlug: "prog-1", Status: "failed", DurationSeconds: 120,
	})

	summaryRes := GetProgramSummary(ctx, store, "prog-1")
	if summaryRes.IsFatal() {
		t.Fatalf("GetProgramSummary returned error: %s", summaryRes.Errors[0].Message)
	}
	summary := summaryRes.GetData()
	if summary.TotalCost != 3.00 {
		t.Errorf("TotalCost = %f, want 3.00", summary.TotalCost)
	}
	if summary.IMPLCount != 2 {
		t.Errorf("IMPLCount = %d, want 2", summary.IMPLCount)
	}
	expectedDur := (300.0 + 120.0) / 60.0
	if summary.TotalDurationMin != expectedDur {
		t.Errorf("TotalDurationMin = %f, want %f", summary.TotalDurationMin, expectedDur)
	}
	if summary.OverallSuccessRate != 0.5 {
		t.Errorf("OverallSuccessRate = %f, want 0.5", summary.OverallSuccessRate)
	}
}

func TestQueryGetCostBreakdown(t *testing.T) {
	store := &mockStore{}
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	_ = store.RecordEvent(ctx, &CostEvent{
		ID: "c1", Type: "cost", Time: base, AgentID: "A",
		IMPLSlug: "impl-1", CostUSD: 0.10,
	})
	_ = store.RecordEvent(ctx, &CostEvent{
		ID: "c2", Type: "cost", Time: base, AgentID: "A",
		IMPLSlug: "impl-1", CostUSD: 0.20,
	})
	_ = store.RecordEvent(ctx, &CostEvent{
		ID: "c3", Type: "cost", Time: base, AgentID: "B",
		IMPLSlug: "impl-1", CostUSD: 0.50,
	})

	bdRes := GetCostBreakdown(ctx, store, "impl-1")
	if bdRes.IsFatal() {
		t.Fatalf("GetCostBreakdown returned error: %s", bdRes.Errors[0].Message)
	}
	breakdown := bdRes.GetData().PerAgent
	if len(breakdown) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(breakdown))
	}
	if breakdown["A"] < 0.299 || breakdown["A"] > 0.301 {
		t.Errorf("agent A cost = %f, want 0.30", breakdown["A"])
	}
	if breakdown["B"] != 0.50 {
		t.Errorf("agent B cost = %f, want 0.50", breakdown["B"])
	}
}

// TestQueryFunctionReturnsQueryData verifies Query returns QueryData with Count.
func TestQueryFunctionReturnsQueryData(t *testing.T) {
	store := &mockStore{}
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Record 3 activity events.
	for i := range 3 {
		_ = store.RecordEvent(ctx, &ActivityEvent{
			ID:           fmt.Sprintf("a%d", i),
			Type:         "activity",
			Time:         base.Add(time.Duration(i) * time.Minute),
			ActivityType: "wave_start",
			IMPLSlug:     "impl-q",
		})
	}

	res := Query(ctx, store, QueryFilters{
		EventTypes: []string{"activity"},
		IMPLSlugs:  []string{"impl-q"},
	})
	if !res.IsSuccess() {
		t.Fatalf("expected success, got code=%s errors=%v", res.Code, res.Errors)
	}
	data := res.GetData()
	if data.Count != 3 {
		t.Errorf("QueryData.Count = %d, want 3", data.Count)
	}
	if len(data.Events) != 3 {
		t.Errorf("len(QueryData.Events) = %d, want 3", len(data.Events))
	}
}

// TestQueryFunctionEmptyResultReturnsSuccess verifies Query returns empty slice (not error) when no matches.
func TestQueryFunctionEmptyResultReturnsSuccess(t *testing.T) {
	store := &mockStore{}
	ctx := context.Background()

	res := Query(ctx, store, QueryFilters{IMPLSlugs: []string{"nonexistent"}})
	if !res.IsSuccess() {
		t.Fatalf("expected success for empty result, got code=%s", res.Code)
	}
	data := res.GetData()
	if data.Count != 0 {
		t.Errorf("QueryData.Count = %d, want 0", data.Count)
	}
	if data.Events == nil {
		t.Error("expected non-nil Events slice for empty result")
	}
}

// TestQueryFunctionStoreErrorReturnsFatal verifies store errors produce FATAL result.
func TestQueryFunctionStoreErrorReturnsFatal(t *testing.T) {
	store := &errorStore{}
	ctx := context.Background()

	res := Query(ctx, store, QueryFilters{})
	if !res.IsFatal() {
		t.Fatalf("expected FATAL result, got code=%s", res.Code)
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if res.Errors[0].Code != "O002_OBS_QUERY_FAILED" {
		t.Errorf("error code = %q, want O002_OBS_QUERY_FAILED", res.Errors[0].Code)
	}
}

// errorStore is a Store that always returns an error from QueryEvents.
type errorStore struct{}

func (e *errorStore) RecordEvent(_ context.Context, _ Event) error { return nil }
func (e *errorStore) QueryEvents(_ context.Context, _ QueryFilters) ([]Event, error) {
	return nil, fmt.Errorf("query error: connection refused")
}
func (e *errorStore) GetRollup(_ context.Context, _ RollupRequest) (*RollupResult, error) {
	return nil, nil
}
func (e *errorStore) Close() error { return nil }

func TestQueryGetFailurePatterns(t *testing.T) {
	store := &mockStore{}
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 3 transient failures across agents A and B, 1 fixable for C.
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p1", Type: "agent_performance", Time: base, AgentID: "A",
		IMPLSlug: "impl-1", Status: "failed", FailureType: "transient",
	})
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p2", Type: "agent_performance", Time: base, AgentID: "B",
		IMPLSlug: "impl-1", Status: "failed", FailureType: "transient",
	})
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p3", Type: "agent_performance", Time: base, AgentID: "A",
		IMPLSlug: "impl-1", Status: "failed", FailureType: "transient",
	})
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p4", Type: "agent_performance", Time: base, AgentID: "C",
		IMPLSlug: "impl-1", Status: "failed", FailureType: "fixable",
	})
	// A success event (no failure type) should be excluded.
	_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
		ID: "p5", Type: "agent_performance", Time: base, AgentID: "A",
		IMPLSlug: "impl-1", Status: "success",
	})

	pattRes := GetFailurePatterns(ctx, store, QueryFilters{})
	if pattRes.IsFatal() {
		t.Fatalf("GetFailurePatterns returned error: %s", pattRes.Errors[0].Message)
	}
	patterns := pattRes.GetData().Patterns
	if len(patterns) != 2 {
		t.Fatalf("expected 2 failure patterns, got %d", len(patterns))
	}
	// Sorted by count descending: transient (3) before fixable (1).
	if patterns[0].FailureType != "transient" {
		t.Errorf("first pattern = %s, want transient", patterns[0].FailureType)
	}
	if patterns[0].Count != 3 {
		t.Errorf("transient count = %d, want 3", patterns[0].Count)
	}
	if len(patterns[0].AffectedAgents) != 2 {
		t.Errorf("transient agents = %d, want 2", len(patterns[0].AffectedAgents))
	}
	if patterns[1].FailureType != "fixable" {
		t.Errorf("second pattern = %s, want fixable", patterns[1].FailureType)
	}
	if patterns[1].Count != 1 {
		t.Errorf("fixable count = %d, want 1", patterns[1].Count)
	}
}
